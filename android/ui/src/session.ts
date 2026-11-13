// The player's episode session, a port of desktop/src/ui/theater/mod.rs (itself
// internal/playback/session.go): resolve (the device reset is the Go core's), check expiry only before a
// load, fall back to the backup URL and then one re-resolve on a load failure, autoplay with up to three
// unavailable episodes skipped in a row, prefetch the next model, and write history at start, pause,
// seek, stop, every 15 s, and at the end.
import { signal } from "nsh";
import { expired, select } from "./playback";
import { OhgError } from "./sdk";
import type { Episode, HistoryEntry, PlaybackResult, SeriesSummary, Stream } from "./types";

const MAX_UNAVAILABLE = 3;
export const RECORD_EVERY_MS = 15_000;

export type Status =
  | { kind: "resolving" }
  | { kind: "playing" }
  | { kind: "unavailable"; error: OhgError }
  | { kind: "failed"; message: string }
  | { kind: "ended" };

/** What the player is told to play: protected streams carry their hex content key. */
export interface Source {
  url: string;
  audioUrl: string;
  key: string;
  start: number;
}

export interface SessionDeps {
  resolve(episodeId: string): Promise<PlaybackResult>;
  record(entry: HistoryEntry): Promise<void>;
  autoplay(): boolean;
  quality(): string;
  /** A short message for the user; `skipped` carries the episode number. */
  notify(message: { kind: "skipped"; episode: number } | { kind: "locked" } | { kind: "history" }): void;
  now(): number;
}

export interface PlayRequest {
  series: SeriesSummary;
  episodes: Episode[];
  start: Episode;
  startSeconds: number;
}

export function createSession(req: PlayRequest, deps: SessionDeps) {
  const status = signal<Status>({ kind: "resolving" });
  const current = signal<Episode>(req.start);
  const source = signal<Source | null>(null);
  const stream = signal<Stream | null>(null);
  const result = signal<PlaybackResult | null>(null);
  let position = 0;
  let paused = false;
  let triedBackup = false;
  let refreshed = false;
  let unavailableRun = 0;
  let startAt = req.startSeconds;
  let lastRecord = deps.now();
  let historyWarned = false;
  let generation = 0;
  let prefetched: { number: number; result: Promise<PlaybackResult> } | null = null;

  const byNumber = (n: number) => req.episodes.find((e) => e.number === n);
  const next = () => byNumber(current().number + 1);
  const prev = () => byNumber(current().number - 1);

  function record(at: number, finished: boolean) {
    lastRecord = deps.now();
    deps
      .record({
        series_id: req.series.id,
        title: req.series.title,
        episode: current().number,
        episodes: req.episodes.length,
        finished,
        position: finished ? 0 : Math.max(at, 0),
      })
      .catch(() => {
        if (!historyWarned) {
          historyWarned = true;
          deps.notify({ kind: "history" });
        }
      });
  }

  function start(ep: Episode, startSeconds: number) {
    if (ep.id === current().id && status().kind === "playing" && result()) return;
    const gen = ++generation;
    source.set(null);
    current.set(ep);
    status.set({ kind: "resolving" });
    result.set(null);
    stream.set(null);
    triedBackup = false;
    refreshed = false;
    startAt = startSeconds;
    position = startSeconds;
    const pending = prefetched?.number === ep.number ? prefetched.result : deps.resolve(ep.id);
    prefetched = null;
    pending.then((r) => gen === generation && onResolved(r, startSeconds, gen)).catch((e) => gen === generation && onUnplayable(asOhg(e)));
    const after = next();
    if (deps.autoplay() && after) {
      const p = deps.resolve(after.id);
      p.catch(() => {});
      prefetched = { number: after.number, result: p };
    }
  }

  function resolveAgain(at: number, gen: number) {
    deps
      .resolve(current().id)
      .then((r) => gen === generation && onResolved(r, at, gen))
      .catch((e) => gen === generation && onUnplayable(asOhg(e)));
  }

  function onResolved(r: PlaybackResult, at: number, gen: number) {
    if (expired(r.playback, Math.floor(deps.now() / 1000)) && !refreshed) {
      refreshed = true;
      return resolveAgain(at, gen);
    }
    const s = select(r.playback, deps.quality());
    if (!s) return onUnplayable(new OhgError("CONTENT_UNAVAILABLE", "no stream"));
    const key = r.contentKey?.key ?? "";
    if (r.playback.protected && !key) return onUnplayable(new OhgError("DRM_PROTECTED", "no content key"));
    result.set(r);
    load(s, at, false);
  }

  function load(s: Stream, at: number, backup: boolean) {
    const r = result();
    if (!r) return;
    stream.set(s);
    source.set({ url: backup ? s.backup_url : s.url, audioUrl: r.playback.audio_url, key: r.contentKey?.key ?? "", start: at });
  }

  /** `playback.Unavailable` skips under autoplay; device-level refusals stop with the "try later" screen. */
  function onUnplayable(err: OhgError) {
    const skippable = ["ANONYMOUS_LIMITED", "AUTHENTICATION_REQUIRED", "DRM_PROTECTED"].includes(err.code);
    if (skippable && deps.autoplay()) {
      unavailableRun++;
      const after = next();
      if (unavailableRun < MAX_UNAVAILABLE && after) {
        deps.notify({ kind: "skipped", episode: current().number });
        return start(after, 0);
      }
    }
    const device = ["ANONYMOUS_LIMITED", "DEVICE_REJECTED", "CHALLENGE_REQUIRED", "DRM_PROTECTED"].includes(err.code);
    status.set(device || skippable ? { kind: "unavailable", error: err } : { kind: "failed", message: err.message });
  }

  /** Backup CDN once, then one re-resolve at the last position. */
  function onLoadFailed(message: string) {
    const s = stream();
    if (s && !triedBackup && s.backup_url) {
      triedBackup = true;
      return load(s, position, true);
    }
    if (!refreshed) {
      refreshed = true;
      triedBackup = false;
      status.set({ kind: "resolving" });
      return resolveAgain(position, generation);
    }
    status.set({ kind: "failed", message });
  }

  start(req.start, req.startSeconds);

  return {
    status: () => status(),
    current: () => current(),
    source: () => source(),
    stream: () => stream(),
    result: () => result(),
    hasNext: () => next() !== undefined,
    hasPrev: () => prev() !== undefined,
    episodes: req.episodes,
    series: req.series,

    /** The player events. */
    loaded() {
      status.set({ kind: "playing" });
      unavailableRun = 0;
      record(startAt, false);
    },
    progress(at: number) {
      position = at;
      if (status().kind === "playing" && !paused && deps.now() - lastRecord >= RECORD_EVERY_MS) record(at, false);
    },
    paused(now: boolean) {
      paused = now;
      if (now) record(position, false);
    },
    seeked(at: number) {
      position = at;
      record(at, false);
    },
    ended() {
      record(0, true);
      const after = deps.autoplay() ? next() : undefined;
      if (after) start(after, 0);
      else {
        source.set(null);
        status.set({ kind: "ended" });
      }
    },
    failed: onLoadFailed,

    /** A user jump records where the current episode was left. */
    jump(ep: Episode) {
      if (!ep.playable) return deps.notify({ kind: "locked" });
      if (status().kind === "playing") record(position, false);
      unavailableRun = 0;
      start(ep, 0);
    },
    next() {
      const after = next();
      if (after) this.jump(after);
    },
    prev() {
      const before = prev();
      if (before) this.jump(before);
    },
    retry() {
      unavailableRun = 0;
      status.set({ kind: "ended" });
      start(current(), 0);
    },
    /** A quality change keeps the position; the caller saves the choice. */
    setQuality(quality: string) {
      const r = result();
      const s = r && select(r.playback, quality);
      if (!s || stream()?.url === s.url) return;
      triedBackup = false;
      load(s, position, false);
    },
    close() {
      if (status().kind === "playing") record(position, false);
      generation++;
      source.set(null);
    },
  };
}

export type Session = ReturnType<typeof createSession>;

function asOhg(e: unknown): OhgError {
  return e instanceof OhgError ? e : new OhgError("LOCAL", e instanceof Error ? e.message : String(e));
}
