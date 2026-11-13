// Library: Continue (unfinished series, with picks from the latest one) and History, covers and episode
// lists from batch lookups (desktop/src/ui/pages/library.rs).
import { effect, List, Scroll, signal, Text, untracked, View } from "nsh";
import { AlertDialog, Button } from "nsh/base";
import { clock } from "../format";
import { clearHistory, historyList } from "../history";
import { fill, isZh, t } from "../i18n";
import { nav } from "../nav";
import { call, describe } from "../sdk";
import { completed, isBook, summaryOf, type HistoryEntry, type Recommend, type SeriesDetail, type SeriesSummary } from "../types";
import { Chip, Cover, toast } from "../ui";

const BATCH = 20;
const MAX_DETAILS = 60;

export function Library() {
  const mode = signal<"continue" | "history">("continue");
  const details = signal<Record<string, SeriesDetail>>({});
  const picks = signal<SeriesSummary[]>([]);
  const requested = new Set<string>();
  let picksFor = "";

  const entries = () => (mode() === "continue" ? historyList().filter((e) => !completed(e)) : historyList());

  effect(() => {
    const missing = entries()
      .map((e) => e.series_id)
      .filter((id) => !requested.has(id))
      .slice(0, MAX_DETAILS);
    missing.forEach((id) => requested.add(id));
    for (let i = 0; i < missing.length; i += BATCH) {
      call<SeriesDetail[] | null>("getSeriesBatch", [missing.slice(i, i + BATCH)])
        .then((list) => {
          const found = (list ?? []).flatMap((d) => (d.series ? [[d.series.id, d] as const] : []));
          details.update((m) => ({ ...m, ...Object.fromEntries(found) }));
        })
        .catch((e) => console.warn("history covers", e));
    }
  });

  // More like the series watched last, minus what was watched.
  effect(() => {
    const latest = historyList()[0];
    if (!latest || latest.series_id === picksFor) return;
    picksFor = latest.series_id;
    call<Recommend>("seriesRecommend", [latest.series_id])
      .then((r) => {
        const seen = new Set(untracked(historyList).map((e) => e.series_id));
        picks.set(r.items.filter((s) => !seen.has(s.id)).slice(0, 12));
      })
      .catch(() => picks.set([]));
  });

  const resume = (entry: HistoryEntry) => {
    const d = details()[entry.series_id];
    const series = d?.series;
    if (!series) return nav.push({ name: "series", id: entry.series_id });
    if (isBook(series)) return toast(t().toastBook);
    const episodes = [...(d.episodes ?? [])].sort((a, b) => a.number - b.number);
    call<{ episode: number; start: number }>("resumePoint", [series.id, episodes.length])
      .then((rp) => {
        const start = episodes.find((e) => e.number === rp.episode) ?? episodes[0];
        if (!start || !start.playable) return nav.push({ name: "series", id: series.id });
        nav.push({ name: "theater", request: { series: summaryOf(series), episodes, start, startSeconds: rp.start } });
      })
      .catch((e) => toast(describe(e, isZh())));
  };

  const confirmClear = async () => {
    const { action } = await AlertDialog.show({
      title: t().confirmClear,
      actions: [
        { id: "no", label: t().no, role: "cancel" },
        { id: "yes", label: t().yes, role: "destructive" },
      ],
    });
    if (action !== "yes") return;
    await clearHistory().then(
      () => toast(t().cleared),
      (e) => toast(describe(e, isZh())),
    );
  };

  const header = (
    <View class="gap-4 pb-2">
      <View class="flex-row items-center gap-2 px-4 pt-2">
        <Chip label={t().continue} on={mode() === "continue"} onPress={() => mode.set("continue")} />
        <Chip label={t().history} on={mode() === "history"} onPress={() => mode.set("history")} />
        <View class="flex-1" />
        <If when={mode() === "history" && historyList().length > 0}>
          <Button onPress={confirmClear} class="px-2 py-1 active:opacity-60">
            <Text class="text-sm text-muted-foreground">{t().clearHistory}</Text>
          </Button>
        </If>
      </View>
      <If when={mode() === "continue" && picks().length > 0}>
        <View class="gap-2">
          <Text class="px-4 font-semibold text-foreground">{fill(t().becauseWatched, historyList()[0]?.title ?? "")}</Text>
          <Scroll horizontal contentClass="flex-row gap-3 px-4">
            <Each of={picks()} key={(s) => s.id}>
              {(s) => (
                <Button onPress={() => nav.push({ name: "series", id: s().id })} class="w-24 gap-1 active:opacity-70">
                  <Cover url={s().cover_url} />
                  <Text class="text-xs text-foreground line-clamp-2">
                    {s().title}
                  </Text>
                </Button>
              )}
            </Each>
          </Scroll>
        </View>
      </If>
    </View>
  );

  return (
    <List
      each={entries()}
      keyExtractor={(e) => e.series_id}
      estimatedItemSize={112}
      header={header}
      empty={
        <View class="items-center p-8">
          <Text class="text-center text-muted-foreground">{mode() === "continue" ? t().emptyContinue : t().emptyHistory}</Text>
        </View>
      }
      class="flex-1"
      contentClass="pb-[calc(env(safe-area-inset-bottom)+16px)]"
    >
      {(e) => (
        <Button onPress={() => resume(e())} class="flex-row gap-3 px-4 py-2 active:bg-muted">
          <Cover url={details()[e().series_id]?.series?.cover_url} class="w-16" />
          <View class="flex-1 justify-center gap-1">
            <Text class="text-foreground line-clamp-2">
              {e().title || e().series_id}
            </Text>
            <Text class="text-xs text-muted-foreground">{entryDetail(e())}</Text>
          </View>
        </Button>
      )}
    </List>
  );
}

function entryDetail(e: HistoryEntry) {
  let s = fill(t().episodeN, e.episode);
  if (e.episodes) s += ` / ${fill(t().episodes, e.episodes)}`;
  if (completed(e)) s += ` · ${t().finished}`;
  else if ((e.position ?? 0) > 0 && !e.finished) s += ` · ${clock(e.position ?? 0)}`;
  return s;
}
