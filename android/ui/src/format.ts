// Ports of desktop/src/ui/format.rs (itself internal/tui/format.go and labels.go).

const trimZero = (f: number) => f.toFixed(1).replace(/\.0$/, "");

/** 1.2万 / 3.4亿 in Chinese, 12.3K / 1.2M in English; zero is empty. */
export function count(n: number, zh: boolean): string {
  if (n <= 0) return "";
  const units: [number, string][] = zh
    ? [[100_000_000, "亿"], [10_000, "万"]]
    : [[1_000_000_000, "B"], [1_000_000, "M"], [1_000, "K"]];
  for (const [div, unit] of units) if (n >= div) return `${trimZero(n / div)}${unit}`;
  return String(n);
}

/** m:ss or h:mm:ss. */
export function clock(seconds: number): string {
  const s = Number.isFinite(seconds) && seconds > 0 ? Math.floor(seconds + 0.5) : 0;
  const pad = (v: number) => String(v).padStart(2, "0");
  if (s >= 3600) return `${Math.floor(s / 3600)}:${pad(Math.floor((s % 3600) / 60))}:${pad(s % 60)}`;
  return `${Math.floor(s / 60)}:${pad(s % 60)}`;
}

export function status(s: number, zh: boolean): string {
  const z: Record<number, [string, string]> = {
    0: ["更新中", "Updating"],
    1: ["已完结", "Finished"],
    3: ["今日更新", "Updated today"],
    4: ["已停更", "Stopped"],
  };
  return z[s]?.[zh ? 0 : 1] ?? "";
}

export function pay(p: number, zh: boolean): string {
  const z: Record<number, [string, string]> = { 0: ["免费", "Free"], 1: ["会员", "VIP"], 2: ["单点付费", "Pay per title"] };
  return z[p]?.[zh ? 0 : 1] ?? "";
}

export function content(c: number, zh: boolean): string {
  const z: Record<number, [string, string]> = {
    1: ["短剧", "Short drama"],
    2: ["情景剧", "Scene play"],
    3: ["达人推荐", "Creator pick"],
    4: ["图文推荐", "Slideshow"],
    5: ["电影", "Movie"],
    6: ["电视剧", "TV series"],
    7: ["小短剧", "Short play"],
    1002: ["AI 短剧", "AI drama"],
    1003: ["合集", "Album"],
    1004: ["漫剧", "Motion comic"],
  };
  return z[c]?.[zh ? 0 : 1] ?? "";
}

/** "2026-10-24T08:15:00Z" → "2026-10-24 08:15". */
export const date = (iso: string) => (iso.length >= 16 ? iso.slice(0, 16).replace("T", " ") : iso);
