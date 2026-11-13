// The three-column grid of series that Discover and Search page through.
import { List, Text, View, type JSX } from "nsh";
import { Button } from "nsh/base";
import { fill, isZh, t } from "./i18n";
import { nav } from "./nav";
import type { Pager } from "./pager";
import { describe } from "./sdk";
import type { SeriesSummary } from "./types";
import { Cover } from "./ui";

export function SeriesGrid(props: {
  pager: Pager<SeriesSummary>;
  /** Covers some lists lack (search results), by series id. */
  covers?: Record<string, string>;
  header?: JSX.Element;
}) {
  const p = props.pager;
  const footer = () => (
    <View class="items-center p-4">
      <If when={p.error()} else={<If when={p.loading()}><Text class="text-muted-foreground">{t().loading}</Text></If>}>
        {(err) => (
          <Button onPress={() => p.more()} class="active:opacity-60">
            <Text class="text-destructive">
              {describe(err(), isZh())} · {t().retry}
            </Text>
          </Button>
        )}
      </If>
    </View>
  );
  return (
    <List
      each={p.items()}
      keyExtractor={(s) => s.id}
      numColumns={3}
      estimatedItemSize={220}
      onLoadMore={() => p.more()}
      refreshing={p.loading() && p.items().length === 0}
      onRefresh={() => p.reset()}
      header={props.header}
      footer={footer()}
      empty={
        <If when={!p.loading() && !p.error()}>
          <View class="items-center p-8">
            <Text class="text-muted-foreground">{t().empty}</Text>
          </View>
        </If>
      }
      class="flex-1"
      contentClass="gap-3 px-3 pb-[calc(env(safe-area-inset-bottom)+16px)]"
    >
      {(s) => (
        <Button onPress={() => nav.push({ name: "series", id: s().id })} class="flex-1 gap-1.5 active:opacity-70">
          <Cover url={s().cover_url || props.covers?.[s().id]} />
          <Text class="line-clamp-2 text-sm text-foreground">
            {s().title}
          </Text>
          <If when={s().episode_count}>
            {(n) => <Text class="text-xs text-muted-foreground">{fill(t().episodes, n())}</Text>}
          </If>
        </Button>
      )}
    </List>
  );
}
