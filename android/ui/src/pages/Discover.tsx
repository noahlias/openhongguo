// Discover: the service's channels, each a paged grid (desktop/src/ui/pages/discover.rs).
import { Scroll, signal, View } from "nsh";
import { SeriesGrid } from "../grid";
import { isZh } from "../i18n";
import { createPager } from "../pager";
import { call } from "../sdk";
import type { DiscoverOptions, Page, SeriesSummary } from "../types";
import { Chip } from "../ui";

export const CHANNELS: [string, string][] = [
  ["推荐", "Featured"],
  ["真人剧", "Live action"],
  ["漫剧", "Motion comics"],
  ["电影", "Movies"],
  ["找剧", "Browse"],
  ["经典", "Classics"],
  ["知识", "Knowledge"],
  ["大视听", "Big screen"],
  ["社区", "Community"],
];

export function Discover() {
  const channel = signal(0);
  const pager = createPager<SeriesSummary>((cursor) =>
    call<Page<SeriesSummary>>("discover", [{ cursor, channel: CHANNELS[channel()][0] } satisfies DiscoverOptions]),
  );
  pager.more();
  const select = (i: number) => {
    if (i === channel()) return;
    channel.set(i);
    pager.reset();
  };
  return (
    <View class="flex-1">
      <Scroll horizontal class="max-h-14 grow-0" contentClass="flex-row gap-2 px-3 py-2.5">
        <Each of={CHANNELS}>
          {(c, i) => <Chip label={isZh() ? c()[0] : c()[1]} on={channel() === i()} onPress={() => select(i())} />}
        </Each>
      </Scroll>
      <SeriesGrid pager={pager} />
    </View>
  );
}
