// Stream choice, a port of desktop/src/sdk/types.rs (`Playback::usable`, `select`, `expired`).
import type { Playback, Stream } from "./types";

function heightRank(s: Stream): number {
  if (s.height > 0) return s.height;
  const digits = /^\d+/.exec(s.definition)?.[0];
  return digits ? Number(digits) : s.width;
}

export function streamLabel(s: Stream): string {
  if (s.definition) return s.definition;
  if (s.height > 0) return `${s.height}p`;
  return s.quality;
}

/** Streams with a URL, lowest to highest. */
export function usable(pb: Playback): Stream[] {
  return (pb.streams ?? [])
    .filter((s) => s.url.trim() !== "")
    .sort((a, b) => heightRank(a) - heightRank(b) || a.bitrate - b.bitrate);
}

/** "best"/"" picks the highest, "worst" the lowest, anything else matches a
 * definition, quality label, or codec, falling back to the best. */
export function select(pb: Playback, quality: string): Stream | undefined {
  const list = usable(pb);
  const want = quality.trim().toLowerCase();
  if (want === "" || want === "best") return list[list.length - 1];
  if (want === "worst") return list[0];
  return list.find((s) => [s.definition, s.quality, s.codec].some((v) => v.toLowerCase() === want)) ?? list[list.length - 1];
}

/** True when the URLs expire within a minute (`playback.Expired`). */
export const expired = (pb: Playback, nowUnix: number) => pb.expires_at > 0 && pb.expires_at < nowUnix + 60;
