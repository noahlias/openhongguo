// Search: suggestions while typing, tabs, the first tab's filters, and paged results whose covers come
// from a batch lookup (desktop/src/ui/pages/search.rs).
import { cleanup, effect, Scroll, signal, Text, untracked, View } from "nsh";
import { Button, Input } from "nsh/base";
import { SeriesGrid } from "../grid";
import { isZh, t } from "../i18n";
import { nav } from "../nav";
import { createPager } from "../pager";
import { call } from "../sdk";
import type { Page, SearchOptions, SeriesDetail, SeriesSummary, Suggestion } from "../types";
import { Chip } from "../ui";

const TABS = ["综合", "剧集", "视频", "小说"];
export const FILTERS: [string, string, string][] = [
  ["content_type_short_play", "真人剧", "Live action"],
  ["content_type_dynamic_comic", "漫剧", "Motion comic"],
  ["content_type_ai_dynamic_comic", "AI剧", "AI drama"],
  ["content_type_long_video", "影视剧", "Film & TV"],
  ["search_range_unread", "未看过", "Unwatched"],
];
const SUGGEST_DELAY_MS = 250;
const BATCH = 20;

export function Search() {
  const draft = signal("");
  const query = signal("");
  const tab = signal(0);
  const filters = signal<string[]>([]);
  const suggestions = signal<Suggestion[]>([]);
  const cue = signal("");
  const covers = signal<Record<string, string>>({});

  const pager = createPager<SeriesSummary>((cursor) => {
    const options: SearchOptions = { cursor, pageSize: 20, tab: TABS[tab()], filters: tab() === 0 ? filters() : [] };
    return call<Page<SeriesSummary>>("search", [query(), options]);
  });

  call<string[] | null>("searchCues").then((cues) => cue.set(cues?.[0] ?? "")).catch(() => {});

  // Typing asks for suggestions once the user pauses; a submitted query hides them.
  effect(() => {
    const prefix = draft().trim();
    if (!prefix || prefix === query()) return suggestions.set([]);
    const abort = new AbortController();
    const timer = setTimeout(() => {
      call<Suggestion[] | null>("suggest", [prefix], { signal: abort.signal })
        .then((list) => suggestions.set((list ?? []).slice(0, 8)))
        .catch(() => {});
    }, SUGGEST_DELAY_MS);
    cleanup(() => {
      clearTimeout(timer);
      abort.abort();
    });
  });

  // Results carry no covers: fetch the missing ones in batches.
  effect(() => {
    const known = untracked(covers);
    const missing = pager.items().filter((s) => !s.cover_url && !(s.id in known)).map((s) => s.id);
    for (let i = 0; i < missing.length; i += BATCH) {
      const chunk = missing.slice(i, i + BATCH);
      covers.update((c) => ({ ...c, ...Object.fromEntries(chunk.map((id) => [id, ""])) }));
      call<SeriesDetail[] | null>("getSeriesBatch", [chunk])
        .then((details) => {
          const found = (details ?? []).flatMap((d) => (d.series ? [[d.series.id, d.series.cover_url] as const] : []));
          covers.update((c) => ({ ...c, ...Object.fromEntries(found) }));
        })
        .catch(() => {});
    }
  });

  const submit = (text: string) => {
    const q = text.trim();
    if (!q) return;
    draft.set(q);
    suggestions.set([]);
    // A series id opens the series.
    if (/^\d{6,}$/.test(q)) return nav.push({ name: "series", id: q });
    query.set(q);
    pager.reset();
  };
  const setTab = (i: number) => {
    tab.set(i);
    if (query()) pager.reset();
  };
  const toggle = (f: string) => {
    filters.update((list) => (list.includes(f) ? list.filter((x) => x !== f) : [...list, f]));
    if (query()) pager.reset();
  };

  return (
    <View class="flex-1">
      <View class="px-3 pt-2">
        <Input
          value={draft()}
          onChange={({ value }) => draft.set(value)}
          onSubmit={({ text }) => submit(text || cue())}
          placeholder={cue() || t().searchPlaceholder}
          returnKeyType="search"
          class="rounded-(--radius) bg-muted px-4 py-2.5 text-base text-foreground"
        />
      </View>
      <If
        when={suggestions().length > 0}
        else={
          <View class="flex-1">
            <Scroll horizontal class="max-h-14 grow-0" contentClass="flex-row gap-2 px-3 py-2.5">
              <Each of={t().searchTabs}>{(label, i) => <Chip label={label()} on={tab() === i()} onPress={() => setTab(i())} />}</Each>
              <If when={tab() === 0}>
                <View class="mx-1 w-px bg-border" />
                <Each of={FILTERS}>
                  {(f) => <Chip label={isZh() ? f()[1] : f()[2]} on={filters().includes(f()[0])} onPress={() => toggle(f()[0])} />}
                </Each>
              </If>
            </Scroll>
            <If when={query()}>
              <SeriesGrid pager={pager} covers={covers()} />
            </If>
          </View>
        }
      >
        <Scroll class="flex-1" keyboardShouldPersistTaps="handled">
          <Each of={suggestions()} key={(s) => s.keyword}>
            {(s) => (
              <Button onPress={() => submit(s().keyword)} class="border-b border-border px-4 py-3 active:bg-muted">
                <Text class="text-foreground">{s().text || s().keyword}</Text>
              </Button>
            )}
          </Each>
        </Scroll>
      </If>
    </View>
  );
}
