// What the series page loads (desktop/src/ui/pages/series.rs): the series and all its episodes are
// required, its seasons and related titles are extras the page works without.
import { call } from "./sdk";
import type { Album, Episode, EpisodeOptions, Page, Recommend, ResumePoint, Series } from "./types";

export interface SeriesData {
  series: Series;
  episodes: Episode[];
  album: Album | null;
  related: Recommend | null;
}

export async function allEpisodes(id: string, signal?: AbortSignal): Promise<Episode[]> {
  const all: Episode[] = [];
  let cursor = "";
  for (;;) {
    const options: EpisodeOptions = { cursor, pageSize: 200 };
    const page = await call<Page<Episode>>("listEpisodes", [id, options], { signal });
    all.push(...page.items);
    if (!page.has_more || !page.cursor || page.cursor === cursor) break;
    cursor = page.cursor;
  }
  return all.sort((a, b) => a.number - b.number);
}

export async function loadSeries(id: string, signal?: AbortSignal): Promise<SeriesData> {
  const extra = <T>(p: Promise<T>) => p.catch(() => null);
  const [series, episodes, album, related] = await Promise.all([
    call<Series>("getSeries", [id], { signal }),
    allEpisodes(id, signal),
    extra(call<Album>("seasons", [id], { signal })),
    extra(call<Recommend>("seriesRecommend", [id], { signal })),
  ]);
  return { series, episodes, album, related };
}

/** The episode and second to continue from (history's position − 3 s; all watched → episode 1). */
export async function resumeFrom(id: string, episodes: Episode[]): Promise<{ episode: Episode; start: number } | null> {
  const rp = await call<ResumePoint>("resumePoint", [id, episodes.length]);
  const episode = episodes.find((e) => e.number === rp.episode) ?? episodes[0];
  return episode ? { episode, start: rp.start } : null;
}
