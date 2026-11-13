// The app's flows against a fake Go core: `ohg.call` answers with the envelope the host app returns.
import { expect, open, test } from "nsh/test";

const series = {
  id: "7001",
  title: "重生之我在片场",
  description: "一部短剧",
  cover_url: "",
  episode_count: 3,
  status: 1,
  pay_type: 0,
  content_type: 1,
  platform: 1,
  heat: 12000,
  play_count: 0,
  follow_count: 0,
  digg_count: 0,
  right_text: "",
  is_multi_season: false,
  season_index: 0,
};
const episode = (n: number) => ({ id: `e${n}`, series_id: "7001", number: n, title: "", playable: n !== 3, vertical: true, is_new: false });

function fakeCore(history: unknown[] = []) {
  const calls: { method: string; args: unknown[] }[] = [];
  const answers: Record<string, (args: unknown[]) => unknown> = {
    discover: () => ({ items: [{ id: "7001", title: series.title, episode_count: 3 }], cursor: "", has_more: false }),
    searchCues: () => ["热门短剧"],
    search: () => ({ items: [{ id: "7001", title: series.title }], cursor: "", has_more: false }),
    suggest: () => [],
    getSeries: () => series,
    getSeriesBatch: () => [{ series, episodes: [episode(1), episode(2), episode(3)] }],
    listEpisodes: () => ({ items: [episode(2), episode(1), episode(3)], cursor: "", has_more: false }),
    seasons: () => ({ id: "", title: "", seasons: [] }),
    seriesRecommend: () => ({ cell_title: "", items: [] }),
    historyList: () => history,
    historyClear: () => {
      history.length = 0;
      return null;
    },
    resumePoint: () => ({ episode: 2, start: 30 }),
    resolvePlaybackFresh: () => ({
      playback: { episode_id: "e2", duration_seconds: 60, expires_at: 0, protected: false, audio_url: "", streams: [{ definition: "720p", quality: "", codec: "h264", width: 720, height: 1280, bitrate: 1, url: "https://v/720", backup_url: "", protected: false }] },
    }),
  };
  const handler = (data: unknown) => {
    const { method, args } = data as { method: string; args: string };
    const parsed = JSON.parse(args) as unknown[];
    calls.push({ method, args: parsed });
    const answer = answers[method];
    return JSON.stringify(answer ? { result: answer(parsed) } : { error: { code: "LOCAL", message: `no ${method}` } });
  };
  return { calls, handlers: { "ohg.call": handler, "ohg.cancel": null } };
}

test("discover opens a series, whose episodes are sorted and whose play continues from history", async () => {
  const core = fakeCore([{ series_id: "7001", title: series.title, episode: 2, episodes: 3, finished: false, position: 33, updated_at: "2026-11-01T00:00:00Z" }]);
  const app = await open("./App.tsx", { platform: "android", host: { handlers: core.handlers } });
  await app.waitFor(series.title);
  await app.press(series.title);
  await app.waitFor("继续 第 2 集");
  expect(app.texts().filter((t) => /^[123]$/.test(t))).toEqual(["1", "2", "3"]);
  await app.press("继续 第 2 集");
  await app.waitFor("重生之我在片场 · 第 2 集");
  const resolved = core.calls.filter((c) => c.method === "resolvePlaybackFresh");
  expect(resolved[0].args).toEqual(["e2"]);
});

test("a locked episode is refused with a toast", async () => {
  const core = fakeCore();
  const app = await open("./App.tsx", { platform: "android", host: { handlers: core.handlers } });
  await app.press(series.title);
  await app.waitFor("role:button[3]");
  await app.press("role:button[3]");
  await app.waitFor("这一集匿名设备不能播放");
  expect(core.calls.some((c) => c.method === "resolvePlaybackFresh")).toBe(false);
});

test("search sends the query with the tab and the first tab's filters", async () => {
  const core = fakeCore();
  const app = await open("./test/tab.tsx", { platform: "android", host: { handlers: core.handlers, params: { tab: "search" } } });
  await app.waitFor("未看过");
  await app.press("未看过");
  await app.type("text:热门短剧", "片场");
  await app.submit("text:片场");
  await app.waitFor(series.title);
  const search = core.calls.find((c) => c.method === "search");
  expect(search?.args).toEqual(["片场", { cursor: "", pageSize: 20, tab: "综合", filters: ["search_range_unread"] }]);
});

test("settings switch the language", async () => {
  const app = await open("./test/tab.tsx", { platform: "android", host: { handlers: fakeCore().handlers, params: { tab: "settings" } } });
  await app.waitFor("English");
  await app.press("English");
  await app.waitFor("Language");
});
