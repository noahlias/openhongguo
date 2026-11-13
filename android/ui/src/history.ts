// Watch history, kept by the Go core in history.json (bindings/internal/desktop): the app holds the
// last list it read, keyed by series id, and reloads it after each write.
import { signal } from "nsh";
import { call } from "./sdk";
import type { HistoryEntry } from "./types";

const entries = signal<Record<string, HistoryEntry>>({});

/** Every entry by series id (tracked where read). */
export const history = () => entries();

/** Newest first. */
export const historyList = () =>
  Object.values(entries()).sort((a, b) => (b.updated_at ?? "").localeCompare(a.updated_at ?? ""));

export async function reloadHistory() {
  const list = await call<HistoryEntry[] | null>("historyList");
  entries.set(Object.fromEntries((list ?? []).map((e) => [e.series_id, e])));
}

export async function recordHistory(entry: HistoryEntry) {
  await call("historyRecord", [entry]);
  await reloadHistory();
}

export async function clearHistory() {
  await call("historyClear");
  await reloadHistory();
}
