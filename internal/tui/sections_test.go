package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// plainFrame is the checked frame without styling, so text split by
// per-rune styles still matches.
func plainFrame(t *testing.T, a *app) string {
	t.Helper()
	return ansi.Strip(checkFrame(t, a))
}

func (f *fakeService) called(s string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Contains(f.calls, s)
}

func TestSearchTypesWithoutGlobalKeysAndFilters(t *testing.T) {
	svc := discoverService()
	svc.search = map[string]*sdk.Page[model.SeriesSummary]{
		"q1:综合::":                        {Items: summaries("s", 3)},
		"q1:综合:content_type_long_video:": {Items: summaries("f", 2)},
	}
	a := newTestApp(t, svc)
	drive(t, a, a.Init())
	resize(t, a, 120, 40)
	press(t, a, "/", "q", "1", "enter")
	if _, ok := a.active().(*searchPage); !ok {
		t.Fatalf("q while typing left the search page")
	}
	frame := plainFrame(t, a)
	if !strings.Contains(frame, "剧集s1") || !strings.Contains(frame, "剧集s3") {
		t.Fatalf("results missing:\n%s", frame)
	}
	press(t, a, "4")
	if !svc.called("search:q1:综合:content_type_long_video:") {
		t.Fatalf("filter chip not sent: %v", svc.calls)
	}
	if frame := plainFrame(t, a); !strings.Contains(frame, "剧集f2") || strings.Contains(frame, "剧集s1") {
		t.Fatalf("filtered results not shown:\n%s", frame)
	}
	press(t, a, "enter")
	if _, ok := a.active().(*seriesPage); !ok {
		t.Fatalf("enter on a result did not open the series")
	}
	press(t, a, "esc", "esc")
	if len(a.stack) != 0 {
		t.Fatalf("esc did not unwind the stack: %d", len(a.stack))
	}
}

func TestHotTopicOpensSearchAndHeadingsAreSkipped(t *testing.T) {
	svc := discoverService()
	svc.hot = &model.HotSearch{Boards: []model.HotBoard{{
		Name:   "热搜榜",
		Items:  []model.HotItem{{Position: 1, SeriesID: "a1", Title: "剧集a1", SearchScore: "355万热搜值"}},
		Topics: []model.HotTopic{{ID: "t1", Title: "话题一", Query: "话题查询"}},
	}}}
	svc.search = map[string]*sdk.Page[model.SeriesSummary]{"话题查询:综合::": {Items: summaries("t", 1)}}
	a := newTestApp(t, svc)
	drive(t, a, a.Init())
	resize(t, a, 120, 40)
	press(t, a, "3")
	s := a.sections[navHot].(*hotSection)
	if _, ok := s.f.list.selected().(seriesRow); !ok {
		t.Fatalf("cursor rests on a heading: %#v", s.f.list.selected())
	}
	if frame := plainFrame(t, a); !strings.Contains(frame, "355万热搜值") {
		t.Fatalf("score missing:\n%s", frame)
	}
	press(t, a, "j", "enter")
	p, ok := a.active().(*searchPage)
	if !ok || p.query != "话题查询" {
		t.Fatalf("topic did not open a search: %#v", a.active())
	}
}

func TestRankCelebrityOpensPersonPage(t *testing.T) {
	svc := discoverService()
	svc.rank = &model.ShortRanking{
		Contents: []model.RankContent{{ID: "c1", Name: "短剧", Boards: []model.RankBoard{{ID: "b1", Name: "热播榜"}, {ID: "b2", Name: "演员榜"}}}},
		Content:  "c1", Board: "b1",
		Items: []model.RankItem{
			{Position: 1, SeriesID: "a1", Title: "剧集a1", Score: "2550万热度", Change: &model.RankChange{Kind: model.RankChangeKind(1), Places: 2}},
			{Position: 2, Actor: "演员甲", UserID: "u1", Score: "98万人气"},
		},
	}
	svc.person = &model.Person{UserID: "u1", Name: "演员甲", Followers: 12_000}
	svc.works = &model.WorkPage{Items: []model.Work{{SeriesID: "a2", Title: "剧集a2", EpisodeCount: 60, Heat: 5_000_000}}}
	a := newTestApp(t, svc)
	drive(t, a, a.Init())
	resize(t, a, 120, 40)
	press(t, a, "4")
	frame := plainFrame(t, a)
	for _, want := range []string{"短剧", "热播榜", "演员榜", "2550万热度", "演员甲"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("rank frame lacks %q:\n%s", want, frame)
		}
	}
	press(t, a, "j", "enter")
	if !svc.called("person:u1") {
		t.Fatalf("person not loaded: %v", svc.calls)
	}
	frame = plainFrame(t, a)
	if !strings.Contains(frame, "1.2万") || !strings.Contains(frame, "剧集a2") {
		t.Fatalf("person page incomplete:\n%s", frame)
	}
}

func TestFilterChipSendsSelection(t *testing.T) {
	svc := discoverService()
	svc.panel = []model.FilterRow{
		{Key: "genre", Name: "类型", Options: []model.FilterOption{{Name: "真人剧", Value: "short_play"}}},
		{Key: "theme", Name: "题材", Options: []model.FilterOption{{Name: "全部", Value: ""}, {Name: "逆袭", Value: "nx"}}},
	}
	svc.filtered = &model.ShortFilterPage{Items: summaries("r", 2)}
	a := newTestApp(t, svc)
	drive(t, a, a.Init())
	resize(t, a, 120, 40)
	press(t, a, "5", "l", " ")
	if !svc.called("filter:map[genre:[short_play] theme:[nx]]") {
		t.Fatalf("selection not sent: %v", svc.calls)
	}
	frame := plainFrame(t, a)
	if !strings.Contains(frame, "逆袭") || !strings.Contains(frame, "剧集r2") {
		t.Fatalf("filter frame incomplete:\n%s", frame)
	}
}

func TestOverlayDoesNotSwallowResults(t *testing.T) {
	a := newTestApp(t, discoverService())
	drive(t, a, a.Init())
	resize(t, a, 120, 40)
	_, cmd := a.Update(keyPress("2"))
	a.overlay = &helpOverlay{tr: a.env.tr}
	drive(t, a, cmd)
	a.overlay = nil
	if s := a.sections[navDiscover].(*discoverSection); s.f.list.len() == 0 {
		t.Fatalf("discover page was lost while the help overlay was open")
	}
}

func TestDownloadsSectionShowsQueue(t *testing.T) {
	a := newTestApp(t, discoverService())
	a.downloads.tasks = []*downloadTask{{id: 1, title: "剧集a1", numbers: []int{1, 2, 3, 4}, saved: 2, state: taskRunning}}
	drive(t, a, a.Init())
	resize(t, a, 120, 40)
	press(t, a, "8")
	frame := plainFrame(t, a)
	if !strings.Contains(frame, "剧集a1") || !strings.Contains(frame, "2/4") || !strings.Contains(frame, "[########........]") {
		t.Fatalf("queue not drawn:\n%s", frame)
	}
}

func TestSeriesRowSecondLineHasNoGapsForMissingFields(t *testing.T) {
	th := newTheme("dark", true)
	bare := seriesRow{item: model.SeriesSummary{Title: "t", Description: "blurb"}, tr: &zhStrings}
	if got := strings.Split(ansi.Strip(bare.render(40, false, &th)), "\n")[1]; !strings.HasPrefix(got, "blurb") {
		t.Fatalf("second line = %q", got)
	}
	counted := seriesRow{item: model.SeriesSummary{Title: "t", EpisodeCount: 3, Description: "blurb"}, note: "9万", tr: &zhStrings}
	if got := strings.Split(ansi.Strip(counted.render(40, false, &th)), "\n")[1]; !strings.HasPrefix(got, "3 集  blurb") {
		t.Fatalf("second line = %q", got)
	}
}
