// A cursor-paged list (discover, search): appends pages without repeats, drops answers to a superseded
// query, and stops at `has_more: false` or END_OF_LIST.
import { batch, signal } from "nsh";
import { OhgError } from "./sdk";
import type { Page } from "./types";

export function createPager<T extends { id: string }>(fetch: (cursor: string) => Promise<Page<T>>) {
  const items = signal<T[]>([]);
  const loading = signal(false);
  const error = signal<unknown>(null);
  const hasMore = signal(true);
  let cursor = "";
  let generation = 0;

  async function more() {
    if (loading() || !hasMore()) return;
    const gen = generation;
    loading.set(true);
    error.set(null);
    try {
      const page = await fetch(cursor);
      if (gen !== generation) return;
      batch(() => {
        const seen = new Set(items().map((i) => i.id));
        items.update((list) => [...list, ...page.items.filter((i) => !seen.has(i.id))]);
        cursor = page.cursor;
        hasMore.set(page.has_more && page.items.length > 0);
      });
    } catch (e) {
      if (gen !== generation) return;
      if (e instanceof OhgError && e.code === "END_OF_LIST") hasMore.set(false);
      else error.set(e);
    } finally {
      if (gen === generation) loading.set(false);
    }
  }

  /** Starts over, as for a new query or channel. */
  function reset() {
    generation++;
    cursor = "";
    batch(() => {
      items.set([]);
      loading.set(false);
      error.set(null);
      hasMore.set(true);
    });
    return more();
  }

  return { items, loading, error, hasMore, more, reset };
}

export type Pager<T extends { id: string }> = ReturnType<typeof createPager<T>>;
