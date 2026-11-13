// A series: its cover and facts, play or continue, seasons, the episode grid and related titles.
import { resource, Scroll, signal, Text, View } from "nsh";
import { Button, NavPage } from "nsh/base";
import { content, count, pay, status } from "../format";
import { history } from "../history";
import { fill, isZh, t } from "../i18n";
import { nav } from "../nav";
import { describe } from "../sdk";
import { loadSeries, resumeFrom, type SeriesData } from "../series";
import { completed, isBook, summaryOf, type Episode } from "../types";
import { Chip, Cover, Header, headerClass, Loading, Message, toast } from "../ui";

export function SeriesPage(props: { id: string }) {
  const id = signal(props.id);
  const data = resource(id, (sid, { signal }) => loadSeries(sid, signal));
  const title = () => data()?.series.title ?? "";
  return (
    <NavPage title={title()} header={<Header title={title()} />} headerClass={headerClass} class="bg-background">
      <If
        when={data()}
        else={
          <If when={data.error()} else={<Loading />}>
            {(err) => <Message text={describe(err(), isZh())} action={t().retry} onAction={data.refetch} />}
          </If>
        }
      >
        {(d) => <Body data={d()} onSeason={(sid) => id.set(sid)} />}
      </If>
    </NavPage>
  );
}

function Body(props: { data: SeriesData; onSeason: (id: string) => void }) {
  const s = () => props.data.series;
  const watched = () => history()[s().id];
  const play = (ep: Episode, start: number) => {
    if (isBook(s())) return toast(t().toastBook);
    if (!ep.playable) return toast(t().toastLocked);
    nav.push({
      name: "theater",
      request: { series: summaryOf(s()), episodes: props.data.episodes, start: ep, startSeconds: start },
    });
  };
  const resume = () => {
    if (isBook(s())) return toast(t().toastBook);
    resumeFrom(s().id, props.data.episodes)
      .then((r) => r && play(r.episode, r.start))
      .catch((e) => toast(describe(e, isZh())));
  };
  const playLabel = () => {
    const w = watched();
    return w && !completed(w) ? fill(t().resume, w.episode) : t().play;
  };
  const pills = () =>
    [status(s().status, isZh()), content(s().content_type, isZh()), pay(s().pay_type, isZh()), s().episode_count ? fill(t().episodes, s().episode_count) : ""].filter(Boolean);
  const stats = () =>
    (
      [
        [t().heat, s().heat],
        [t().plays, s().play_count],
        [t().follows, s().follow_count],
      ] as [string, number][]
    )
      .map(([label, n]) => [label, count(n, isZh())])
      .filter(([, v]) => v);

  return (
    <Scroll class="flex-1" contentClass="gap-5 p-4 pb-[calc(env(safe-area-inset-bottom)+24px)]">
      <View class="flex-row gap-4">
        <Cover url={s().cover_url} class="w-32" />
        <View class="flex-1 gap-2">
          <Text class="text-lg font-semibold text-foreground">{s().title}</Text>
          <Text class="text-xs text-muted-foreground">{pills().join(" · ")}</Text>
          <Text class="text-xs text-muted-foreground">{stats().map(([l, v]) => `${l} ${v}`).join("  ")}</Text>
          <Button onPress={resume} class="mt-auto items-center rounded-(--radius) bg-primary py-2.5 active:opacity-80">
            <Text class="font-semibold text-primary-foreground">{playLabel()}</Text>
          </Button>
        </View>
      </View>

      <If when={s().description}>
        <View class="gap-1.5">
          <Text class="font-semibold text-foreground">{t().description}</Text>
          <Text class="text-sm leading-relaxed text-muted-foreground">{s().description}</Text>
        </View>
      </If>

      <If when={(props.data.album?.seasons.length ?? 0) > 1}>
        <View class="gap-2">
          <Text class="font-semibold text-foreground">{t().seasons}</Text>
          <Scroll horizontal contentClass="flex-row gap-2">
            <Each of={props.data.album?.seasons ?? []} key={(x) => x.series_id}>
              {(season) => <Chip label={season().title} on={season().series_id === s().id} onPress={() => props.onSeason(season().series_id)} />}
            </Each>
          </Scroll>
        </View>
      </If>

      <View class="gap-2">
        <Text class="font-semibold text-foreground">{t().episodeList}</Text>
        <View class="flex-row flex-wrap gap-2">
          <Each of={props.data.episodes} key={(e) => e.id}>
            {(ep) => (
              <Button
                onPress={() => play(ep(), 0)}
                class="h-10 w-14 items-center justify-center rounded-(--radius) active:opacity-70"
                classList={{ "bg-primary": watched()?.episode === ep().number, "bg-muted": watched()?.episode !== ep().number, "opacity-40": !ep().playable }}
              >
                <Text class={watched()?.episode === ep().number ? "text-primary-foreground" : "text-foreground"}>{ep().number}</Text>
              </Button>
            )}
          </Each>
        </View>
      </View>

      <If when={props.data.related?.items.length}>
        <View class="gap-2">
          <Text class="font-semibold text-foreground">{props.data.related?.cell_title || t().related}</Text>
          <Scroll horizontal contentClass="flex-row gap-3">
            <Each of={props.data.related?.items ?? []} key={(x) => x.id}>
              {(r) => (
                <Button onPress={() => nav.push({ name: "series", id: r().id })} class="w-28 gap-1.5 active:opacity-70">
                  <Cover url={r().cover_url} />
                  <Text class="text-xs text-foreground line-clamp-2">
                    {r().title}
                  </Text>
                </Button>
              )}
            </Each>
          </Scroll>
        </View>
      </If>
    </Scroll>
  );
}

