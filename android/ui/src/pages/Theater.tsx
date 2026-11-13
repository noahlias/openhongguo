// The theater: the mpv player driven by the episode session (session.ts), with tap-to-show controls,
// a seek bar, previous/next, the episode sheet and the quality sheet.
import { cleanup, signal, Text, View, type NativeComponentRef } from "nsh";
import { Button, NavPage, Sheet, Slider } from "nsh/base";
import { clock } from "../format";
import { recordHistory } from "../history";
import { fill, isZh, t } from "../i18n";
import { nav } from "../nav";
import { streamLabel, usable } from "../playback";
import { Player } from "../player";
import { call, describe } from "../sdk";
import { createSession, type PlayRequest } from "../session";
import { settings, updateSettings } from "../settings";
import type { PlaybackResult } from "../types";
import { toast } from "../ui";

export function Theater(props: { request: PlayRequest }) {
  const session = createSession(props.request, {
    resolve: (id) => call<PlaybackResult>("resolvePlaybackFresh", [id]),
    record: (entry) => recordHistory(entry),
    autoplay: () => settings().autoplay,
    quality: () => settings().quality,
    notify: (m) =>
      toast(m.kind === "skipped" ? fill(t().toastSkipped, m.episode) : m.kind === "locked" ? t().toastLocked : t().toastHistory),
    now: () => Date.now(),
  });
  cleanup(() => session.close());

  let player: NativeComponentRef | undefined;
  const paused = signal(false);
  const buffering = signal(false);
  const position = signal(0);
  const duration = signal(0);
  const dragging = signal<number | null>(null);
  const controls = signal(true);
  const episodesOpen = signal(false);
  const qualityOpen = signal(false);
  let hideTimer: ReturnType<typeof setTimeout> | undefined;

  const showControls = () => {
    controls.set(true);
    clearTimeout(hideTimer);
    hideTimer = setTimeout(() => !paused() && controls.set(false), 3500);
  };
  cleanup(() => clearTimeout(hideTimer));
  showControls();

  const togglePause = () => {
    paused.update((p) => !p);
    showControls();
  };
  const seek = (at: number) => {
    position.set(at);
    player?.command("seek", { position: at });
  };
  const setQuality = (q: string) => {
    qualityOpen.set(false);
    session.setQuality(q);
    updateSettings({ quality: q }).catch((e) => toast(describe(e, isZh())));
  };
  const status = () => session.status();
  const episodeTitle = () => `${session.series.title} · ${fill(t().episodeN, session.current().number)}`;

  return (
    <NavPage title={episodeTitle()} headerVisible={false} statusBar={{ foreground: "light" }} class="bg-black">
      <View class="flex-1 bg-black">
        <Player
          ref={player}
          source={session.source()}
          paused={paused()}
          class="absolute inset-0"
          onLoad={({ duration: d }) => {
            duration.set(d);
            paused.set(false);
            session.loaded();
          }}
          onProgress={(e) => {
            position.set(e.position);
            if (e.duration > 0) duration.set(e.duration);
            session.progress(e.position);
          }}
          onPause={(e) => {
            paused.set(e.paused);
            session.paused(e.paused);
            if (e.paused) controls.set(true);
          }}
          onBuffering={(e) => buffering.set(e.buffering)}
          onSeeked={(e) => session.seeked(e.position)}
          onEnded={() => session.ended()}
          onError={(e) => session.failed(e.message)}
        />

        <View class="absolute inset-0" onPress={() => (controls() ? controls.set(false) : showControls())} />

        <If when={status().kind === "resolving" || buffering()}>
          <View class="absolute inset-0 items-center justify-center" style={{ pointerEvents: "none" }}>
            <Text class="text-white/80">{t().loading}</Text>
          </View>
        </If>

        <If when={status().kind === "unavailable" || status().kind === "failed" || status().kind === "ended"}>
          <View class="absolute inset-0 items-center justify-center gap-4 bg-black/80 p-8">
            <Text class="text-lg font-semibold text-white">{statusTitle(status())}</Text>
            <Text class="text-center text-white/70">{statusBody(status())}</Text>
            <Button onPress={() => session.retry()} class="rounded-(--radius) bg-primary px-5 py-2 active:opacity-80">
              <Text class="text-primary-foreground">{t().retry}</Text>
            </Button>
          </View>
        </If>

        <If when={controls()}>
          <View class="absolute inset-x-0 top-0 flex-row items-center gap-3 bg-black/40 px-3 pb-3 pt-[calc(env(safe-area-inset-top)+8px)]">
            <Button onPress={() => nav.pop()} accessibilityLabel={t().back} class="px-2 py-1 active:opacity-60">
              <Text class="text-2xl text-white">‹</Text>
            </Button>
            <Text class="flex-1 text-white line-clamp-1">
              {episodeTitle()}
            </Text>
          </View>

          <View class="absolute inset-x-0 top-1/2 -mt-8 flex-row items-center justify-center gap-12" style={{ pointerEvents: "box-none" }}>
            <Button onPress={() => session.prev()} disabled={!session.hasPrev()} class="p-3 active:opacity-60 disabled:opacity-30">
              <Text class="text-3xl text-white">⏮</Text>
            </Button>
            <Button onPress={togglePause} accessibilityLabel={paused() ? t().play : "pause"} class="size-16 items-center justify-center rounded-full bg-black/50 active:opacity-60">
              <Text class="text-3xl text-white">{paused() ? "▶" : "⏸"}</Text>
            </Button>
            <Button onPress={() => session.next()} disabled={!session.hasNext()} class="p-3 active:opacity-60 disabled:opacity-30">
              <Text class="text-3xl text-white">⏭</Text>
            </Button>
          </View>

          <View class="absolute inset-x-0 bottom-0 gap-2 bg-black/40 px-4 pt-3 pb-[calc(env(safe-area-inset-bottom)+12px)]">
            <View class="flex-row items-center gap-3">
              <Text class="w-12 text-xs text-white">{clock(dragging() ?? position())}</Text>
              <Slider
                value={dragging() ?? position()}
                min={0}
                max={Math.max(duration(), 1)}
                step={0}
                accessibilityLabel="seek"
                onChange={({ value }) => {
                  dragging.set(value as number);
                  showControls();
                }}
                onChangeEnd={({ value }) => {
                  dragging.set(null);
                  seek(value as number);
                }}
                class="flex-1"
                trackClass="h-6 justify-center"
                indicatorClass="h-1 rounded-full bg-white/30"
                rangeClass="rounded-full bg-primary"
                thumbClass="size-3.5 rounded-full bg-white"
              />
              <Text class="w-12 text-right text-xs text-white">{clock(duration())}</Text>
            </View>
            <View class="flex-row items-center justify-end gap-2">
              <Button onPress={() => episodesOpen.set(true)} class="rounded-full bg-white/15 px-3 py-1.5 active:opacity-60">
                <Text class="text-sm text-white">{t().episodeList}</Text>
              </Button>
              <If when={session.stream()}>
                {(s) => (
                  <Button onPress={() => qualityOpen.set(true)} class="rounded-full bg-white/15 px-3 py-1.5 active:opacity-60">
                    <Text class="text-sm text-white">{streamLabel(s())}</Text>
                  </Button>
                )}
              </If>
            </View>
          </View>
        </If>
      </View>

      <Sheet
        open={episodesOpen()}
        onOpenChange={(e) => episodesOpen.set(e.open)}
        side="bottom"
        popupClass="max-h-[60%] rounded-t-2xl bg-popover p-4 pb-[calc(env(safe-area-inset-bottom)+16px)]"
        backdropClass="bg-scrim"
      >
        <Text class="mb-3 font-semibold text-popover-foreground">{t().episodeList}</Text>
        <View class="flex-row flex-wrap gap-2">
          <Each of={session.episodes} key={(e) => e.id}>
            {(ep) => (
              <Button
                onPress={() => {
                  episodesOpen.set(false);
                  session.jump(ep());
                }}
                class="h-10 w-14 items-center justify-center rounded-(--radius) active:opacity-70"
                classList={{ "bg-primary": session.current().id === ep().id, "bg-muted": session.current().id !== ep().id, "opacity-40": !ep().playable }}
              >
                <Text class={session.current().id === ep().id ? "text-primary-foreground" : "text-foreground"}>{ep().number}</Text>
              </Button>
            )}
          </Each>
        </View>
      </Sheet>

      <Sheet
        open={qualityOpen()}
        onOpenChange={(e) => qualityOpen.set(e.open)}
        side="bottom"
        popupClass="rounded-t-2xl bg-popover p-4 pb-[calc(env(safe-area-inset-bottom)+16px)]"
        backdropClass="bg-scrim"
      >
        <Text class="mb-2 font-semibold text-popover-foreground">{t().quality}</Text>
        <Each of={session.result() ? usable(session.result()!.playback).reverse() : []} key={(s) => s.url}>
          {(s) => (
            <Button onPress={() => setQuality(streamLabel(s()))} class="flex-row justify-between py-3 active:opacity-60">
              <Text class={session.stream()?.url === s().url ? "text-primary" : "text-popover-foreground"}>{streamLabel(s())}</Text>
              <Text class="text-xs text-muted-foreground">{s().codec}</Text>
            </Button>
          )}
        </Each>
      </Sheet>
    </NavPage>
  );
}

type Status = ReturnType<ReturnType<typeof createSession>["status"]>;

function statusTitle(s: Status) {
  if (s.kind === "unavailable") return t().unavailableTitle;
  if (s.kind === "ended") return t().finished;
  return t().unavailableTitle;
}

function statusBody(s: Status) {
  if (s.kind === "unavailable") return s.error.unavailable ? t().unavailableBody : describe(s.error, isZh());
  if (s.kind === "failed") return s.message;
  return "";
}
