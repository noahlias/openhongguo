// The ports of the desktop's logic, with the desktop's own cases (desktop/src/sdk/types.rs, ui/format.rs).
import { expect, test } from "nsh/test";
import { clock, count } from "./format";
import { select, usable } from "./playback";
import { OhgError } from "./sdk";
import { createSession, type SessionDeps } from "./session";
import type { Episode, HistoryEntry, Playback, PlaybackResult, Stream } from "./types";
import { completed } from "./types";

const stream = (definition: string, height: number, bitrate: number): Stream => ({
  definition,
  quality: "",
  codec: "",
  width: 0,
  height,
  bitrate,
  url: `u-${definition}`,
  backup_url: `b-${definition}`,
  protected: false,
});

const playback = (streams: Stream[], extra: Partial<Playback> = {}): Playback => ({
  episode_id: "e",
  duration_seconds: 60,
  expires_at: 0,
  protected: false,
  audio_url: "a",
  streams,
  ...extra,
});

test("select orders by height then bitrate", () => {
  const pb = playback([stream("720p", 1280, 2), stream("1080p", 1920, 1), stream("360p", 640, 9), { ...stream("x", 0, 0), url: "" }]);
  expect(select(pb, "best")?.definition).toBe("1080p");
  expect(select(pb, "")?.definition).toBe("1080p");
  expect(select(pb, "worst")?.definition).toBe("360p");
  expect(select(pb, "720P")?.definition).toBe("720p");
  expect(select(pb, "4k")?.definition).toBe("1080p");
  expect(usable(pb).length).toBe(3);
});

test("counts and clocks match the desktop", () => {
  expect(count(0, true)).toBe("");
  expect(count(9_999, true)).toBe("9999");
  expect(count(12_000, true)).toBe("1.2万");
  expect(count(100_000, true)).toBe("10万");
  expect(count(340_000_000, true)).toBe("3.4亿");
  expect(count(12_300, false)).toBe("12.3K");
  expect(count(1_200_000, false)).toBe("1.2M");
  expect(clock(0)).toBe("0:00");
  expect(clock(65.4)).toBe("1:05");
  expect(clock(3725)).toBe("1:02:05");
  expect(clock(Number.NaN)).toBe("0:00");
});

test("completed needs the last episode", () => {
  const e: HistoryEntry = { series_id: "s", episode: 3, episodes: 10, finished: true };
  expect(completed(e)).toBe(false);
  expect(completed({ ...e, episode: 10 })).toBe(true);
});

const episodes: Episode[] = [1, 2, 3, 4].map((n) => ({
  id: `e${n}`,
  series_id: "s",
  number: n,
  title: "",
  playable: true,
  vertical: true,
  is_new: false,
}));

function fakeDeps(resolve: (id: string) => Promise<PlaybackResult>) {
  const records: HistoryEntry[] = [];
  const notes: string[] = [];
  let now = 1_000_000;
  const deps: SessionDeps = {
    resolve,
    record: async (e) => void records.push(e),
    autoplay: () => true,
    quality: () => "best",
    notify: (m) => notes.push(m.kind === "skipped" ? `skipped ${m.episode}` : m.kind),
    now: () => now,
  };
  return { deps, records, notes, advance: (ms: number) => (now += ms) };
}

const settle = () => new Promise<void>((r) => setTimeout(r, 0));

test("a session plays, records ticks and autoplays the next episode", async () => {
  const f = fakeDeps(async (id) => ({ playback: playback([stream("720p", 1280, 1)], { episode_id: id }) }));
  const s = createSession({ series: { id: "s", title: "T" }, episodes, start: episodes[0], startSeconds: 12 }, f.deps);
  await settle();
  expect(s.source()).toEqual({ url: "u-720p", audioUrl: "a", key: "", start: 12 });
  s.loaded();
  expect(s.status().kind).toBe("playing");
  expect(f.records[0]).toMatchObject({ episode: 1, position: 12, finished: false });
  s.progress(20);
  expect(f.records.length).toBe(1);
  f.advance(15_000);
  s.progress(27);
  expect(f.records[1]).toMatchObject({ episode: 1, position: 27 });
  s.ended();
  expect(f.records[2]).toMatchObject({ episode: 1, finished: true, position: 0 });
  await settle();
  expect(s.current().number).toBe(2);
  expect(s.source()?.start).toBe(0);
});

test("a load failure tries the backup, then resolves once more, then fails", async () => {
  let resolves = 0;
  const f = fakeDeps(async () => {
    resolves++;
    return { playback: playback([stream("720p", 1280, 1)]) };
  });
  const s = createSession({ series: { id: "s", title: "T" }, episodes, start: episodes[0], startSeconds: 0 }, f.deps);
  await settle();
  const before = resolves;
  s.progress(30);
  s.failed("boom");
  expect(s.source()).toMatchObject({ url: "b-720p", start: 30 });
  s.failed("boom");
  await settle();
  expect(resolves).toBe(before + 1);
  expect(s.source()?.url).toBe("u-720p");
  s.failed("boom");
  s.failed("boom");
  expect(s.status()).toEqual({ kind: "failed", message: "boom" });
});

test("unavailable episodes are skipped up to three in a row", async () => {
  const f = fakeDeps(async () => {
    throw new OhgError("ANONYMOUS_LIMITED", "empty");
  });
  const s = createSession({ series: { id: "s", title: "T" }, episodes, start: episodes[0], startSeconds: 0 }, f.deps);
  for (let i = 0; i < 6; i++) await settle();
  expect(f.notes).toEqual(["skipped 1", "skipped 2"]);
  expect(s.current().number).toBe(3);
  expect(s.status().kind).toBe("unavailable");
});

test("a protected model without a key is unavailable", async () => {
  const f = fakeDeps(async () => ({ playback: playback([stream("720p", 1280, 1)], { protected: true }) }));
  f.deps.autoplay = () => false;
  const s = createSession({ series: { id: "s", title: "T" }, episodes, start: episodes[0], startSeconds: 0 }, f.deps);
  await settle();
  expect(s.status()).toMatchObject({ kind: "unavailable" });
  expect(s.source()).toBeNull();
});
