// The host's mpv player (android/app PlayerComponent.kt).
import { nativeComponent } from "nsh";
import type { Source } from "./session";

export interface PlayerProps {
  source: Source | null;
  paused: boolean;
  rate?: number;
}

export interface PlayerEvents {
  load: { duration: number };
  progress: { position: number; duration: number; buffered: number };
  pause: { paused: boolean };
  buffering: { buffering: boolean };
  seeked: { position: number };
  ended: null;
  error: { message: string };
}

export const Player = nativeComponent<PlayerProps, PlayerEvents>("ohg.player", {
  props: ["source", "paused", "rate"],
  events: ["load", "progress", "pause", "buffering", "seeked", "ended", "error"],
});
