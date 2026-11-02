package tui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/beilunyang/openhongguo/internal/history"
	"github.com/beilunyang/openhongguo/internal/media"
	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// fakeService answers the surfaces the tests drive and refuses the rest.
type fakeService struct {
	mu       sync.Mutex
	discover map[string]*sdk.Page[model.SeriesSummary]
	series   map[string]*model.Series
	episodes map[string][]model.Episode
	calls    []string

	search   map[string]*sdk.Page[model.SeriesSummary]
	hot      *model.HotSearch
	rank     *model.ShortRanking
	panel    []model.FilterRow
	filtered *model.ShortFilterPage
	person   *model.Person
	works    *model.WorkPage
}

func answer[T any](v *T) (*T, error) {
	if v == nil {
		return nil, errUnused
	}
	return v, nil
}

var errUnused = errors.New("unused")

func (f *fakeService) record(s string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, s)
}

func (f *fakeService) Discover(_ context.Context, o sdk.DiscoverOptions) (*sdk.Page[model.SeriesSummary], error) {
	f.record("discover:" + o.Channel + ":" + o.Cursor)
	if p, ok := f.discover[o.Channel+":"+o.Cursor]; ok {
		return p, nil
	}
	return nil, sdk.ErrAnonymousLimited
}
func (f *fakeService) Search(_ context.Context, q string, o sdk.SearchOptions) (*sdk.Page[model.SeriesSummary], error) {
	key := q + ":" + o.Tab + ":" + strings.Join(o.Filters, ",") + ":" + o.Cursor
	f.record("search:" + key)
	return answer(f.search[key])
}
func (f *fakeService) GetSeries(_ context.Context, id string) (*model.Series, error) {
	f.record("series:" + id)
	if s, ok := f.series[id]; ok {
		return s, nil
	}
	return nil, sdk.ErrContentUnavailable
}
func (f *fakeService) GetSeriesBatch(context.Context, []string) ([]model.SeriesDetail, error) {
	return nil, errUnused
}
func (f *fakeService) GetAlbum(context.Context, string, sdk.AlbumOptions) (*model.Album, error) {
	return nil, errUnused
}
func (f *fakeService) Seasons(context.Context, string) (*model.Album, error) { return nil, errUnused }
func (f *fakeService) Suggest(context.Context, string) ([]model.Suggestion, error) {
	return nil, errUnused
}
func (f *fakeService) SearchCues(context.Context) ([]string, error)        { return nil, errUnused }
func (f *fakeService) HotSearch(context.Context) (*model.HotSearch, error) { return answer(f.hot) }
func (f *fakeService) NewReleases(context.Context, sdk.NewReleaseQuery) (*model.NewReleasePage, error) {
	return nil, errUnused
}
func (f *fakeService) Person(_ context.Context, id string) (*model.Person, error) {
	f.record("person:" + id)
	return answer(f.person)
}
func (f *fakeService) PersonWorks(context.Context, string, string) (*model.WorkPage, error) {
	return answer(f.works)
}
func (f *fakeService) SeriesRecommend(context.Context, string) (*model.Recommend, error) {
	return nil, errUnused
}
func (f *fakeService) ShortFilterPanel(context.Context, string) ([]model.FilterRow, error) {
	if f.panel == nil {
		return nil, errUnused
	}
	return f.panel, nil
}
func (f *fakeService) ShortFilter(_ context.Context, q sdk.ShortFilterQuery) (*model.ShortFilterPage, error) {
	f.record(fmt.Sprintf("filter:%v", q.Selections))
	return answer(f.filtered)
}
func (f *fakeService) Ranking(context.Context, string, string) (*model.Ranking, error) {
	return nil, errUnused
}
func (f *fakeService) RankRows(context.Context, string, int) (*model.Ranking, error) {
	return nil, errUnused
}
func (f *fakeService) RankGroups(context.Context) ([]model.RankList, error) { return nil, errUnused }
func (f *fakeService) ShortRank(context.Context, sdk.ShortRankQuery) (*model.ShortRanking, error) {
	return answer(f.rank)
}
func (f *fakeService) Categories(context.Context, string) (*model.CategoryPage, error) {
	return nil, errUnused
}
func (f *fakeService) CategoryLanding(context.Context, string, sdk.LandingOptions) (*model.CategoryPage, error) {
	return nil, errUnused
}
func (f *fakeService) ListEpisodes(_ context.Context, id string, _ sdk.EpisodeOptions) (*sdk.Page[model.Episode], error) {
	f.record("episodes:" + id)
	if eps, ok := f.episodes[id]; ok {
		return &sdk.Page[model.Episode]{Items: eps}, nil
	}
	return nil, sdk.ErrContentUnavailable
}
func (f *fakeService) ResolvePlayback(_ context.Context, id string, _ sdk.PlaybackOptions) (*model.Playback, error) {
	f.record("playback:" + id)
	return &model.Playback{
		EpisodeID: id, DurationSeconds: 90,
		Streams: []model.Stream{{Definition: "720p", URL: "https://cdn.invalid/" + id + ".mp4"}},
	}, nil
}

type fakeMedia struct {
	mu     sync.Mutex
	played []media.Request
}

func (m *fakeMedia) Open(context.Context, string, string) error          { return nil }
func (m *fakeMedia) Save(context.Context, string, string) (int64, error) { return 0, errUnused }
func (m *fakeMedia) Decrypt(context.Context, string, string, string, string, int) (int64, error) {
	return 0, errUnused
}
func (m *fakeMedia) Play(_ context.Context, req media.Request) (media.Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.played = append(m.played, req)
	return media.Result{}, nil
}

func summaries(prefix string, n int) []model.SeriesSummary {
	out := make([]model.SeriesSummary, n)
	for i := range out {
		id := fmt.Sprintf("%s%d", prefix, i+1)
		out[i] = model.SeriesSummary{ID: id, Title: "剧集" + id, EpisodeCount: 10 + i, Description: "简介 " + id}
	}
	return out
}

func newTestApp(t *testing.T, svc *fakeService) *app {
	t.Helper()
	dir := t.TempDir()
	hist := history.NewStore(filepath.Join(dir, "history.json"))
	deps := Deps{Service: svc, Media: &fakeMedia{}, History: hist, ConfigDir: dir}
	s := defaultSettings()
	s.Theme = "mono"
	a := newApp(context.Background(), deps, s, filepath.Join(dir, "settings.json"), nil)
	a.player.send = func(tea.Msg) {}
	a.downloads.send = func(tea.Msg) {}
	return a
}

// drive runs cmd and feeds every message it produces back into the app,
// which is what the program loop does. A command that takes longer than
// half a second (a toast's expiry tick) is left behind.
func drive(t *testing.T, a *app, cmd tea.Cmd) {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg, ok := runCmd(c)
		if !ok || msg == nil {
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		switch msg.(type) {
		case tea.BackgroundColorMsg, tea.QuitMsg:
			continue
		}
		_, next := a.Update(msg)
		queue = append(queue, next)
	}
}

func runCmd(c tea.Cmd) (tea.Msg, bool) {
	ch := make(chan tea.Msg, 1)
	go func() { ch <- c() }()
	select {
	case msg := <-ch:
		return msg, true
	case <-time.After(500 * time.Millisecond):
		return nil, false
	}
}

func press(t *testing.T, a *app, keys ...string) {
	t.Helper()
	for _, k := range keys {
		_, cmd := a.Update(keyPress(k))
		drive(t, a, cmd)
	}
}

func keyPress(k string) tea.KeyPressMsg {
	switch k {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	}
	r := []rune(k)
	return tea.KeyPressMsg{Code: r[0], Text: k}
}

func resize(t *testing.T, a *app, w, h int) {
	t.Helper()
	_, cmd := a.Update(tea.WindowSizeMsg{Width: w, Height: h})
	drive(t, a, cmd)
}

func checkFrame(t *testing.T, a *app) string {
	t.Helper()
	out := a.View().Content
	lines := strings.Split(out, "\n")
	if len(lines) != a.height {
		t.Fatalf("frame has %d lines, want %d", len(lines), a.height)
	}
	for i, l := range lines {
		if n := width(l); n > a.width {
			t.Fatalf("line %d is %d cells wide, want <= %d: %q", i, n, a.width, l)
		}
	}
	return out
}

func discoverService() *fakeService {
	first := summaries("a", 8)
	second := summaries("b", 8)
	svc := &fakeService{
		discover: map[string]*sdk.Page[model.SeriesSummary]{
			"推荐:":   {Items: first, Cursor: "c1", HasMore: true},
			"推荐:c1": {Items: second, Cursor: "", HasMore: false},
			"真人剧:":  {Items: summaries("h", 3)},
		},
		series:   map[string]*model.Series{},
		episodes: map[string][]model.Episode{},
	}
	for _, s := range append(first, second...) {
		full := &model.Series{SeriesSummary: s, Heat: 123_456_789, Status: 1, Platform: model.PlatformType(3), PayType: 0}
		svc.series[s.ID] = full
		eps := make([]model.Episode, s.EpisodeCount)
		for i := range eps {
			eps[i] = model.Episode{ID: s.ID + "-" + itoa(i+1), SeriesID: s.ID, Number: i + 1, Playable: i != 2}
		}
		svc.episodes[s.ID] = eps
	}
	return svc
}

func TestStartsOnContinueAndSwitchesSections(t *testing.T) {
	a := newTestApp(t, discoverService())
	drive(t, a, a.Init())
	resize(t, a, 120, 40)
	frame := checkFrame(t, a)
	if !strings.Contains(frame, zhStrings.EmptyContinue) {
		t.Fatalf("continue section not shown:\n%s", frame)
	}
	press(t, a, "2")
	frame = checkFrame(t, a)
	if !strings.Contains(frame, "剧集a1") || !strings.Contains(frame, "剧集a8") {
		t.Fatalf("discover rows missing:\n%s", frame)
	}
	if !strings.Contains(frame, "1.2亿") {
		t.Fatalf("detail pane should show formatted heat:\n%s", frame)
	}
}

func TestDiscoverPagesNearTheEndAndDedups(t *testing.T) {
	svc := discoverService()
	a := newTestApp(t, svc)
	drive(t, a, a.Init())
	resize(t, a, 120, 40)
	press(t, a, "2")
	for i := 0; i < 5; i++ {
		press(t, a, "j")
	}
	frame := checkFrame(t, a)
	if !strings.Contains(frame, "剧集b1") {
		t.Fatalf("second page not loaded:\n%s", frame)
	}
	s := a.sections[navDiscover].(*discoverSection)
	if s.f.list.len() != 16 || s.f.hasMore {
		t.Fatalf("rows=%d hasMore=%v", s.f.list.len(), s.f.hasMore)
	}
	// A late answer to an old request is ignored.
	s.Update(discoverPageMsg{req: -1, page: &sdk.Page[model.SeriesSummary]{Items: summaries("z", 2)}})
	if s.f.list.len() != 16 {
		t.Fatalf("stale page accepted: rows=%d", s.f.list.len())
	}
}

func TestChannelTabReloads(t *testing.T) {
	svc := discoverService()
	a := newTestApp(t, svc)
	drive(t, a, a.Init())
	resize(t, a, 120, 40)
	press(t, a, "2", "]")
	frame := checkFrame(t, a)
	if !strings.Contains(frame, "剧集h1") || strings.Contains(frame, "剧集a1") {
		t.Fatalf("channel switch did not reload:\n%s", frame)
	}
}

func TestEnterOpensSeriesPageAndPlays(t *testing.T) {
	svc := discoverService()
	a := newTestApp(t, svc)
	drive(t, a, a.Init())
	resize(t, a, 120, 40)
	press(t, a, "2", "enter")
	frame := checkFrame(t, a)
	if len(a.stack) != 1 || !strings.Contains(frame, "已完结") {
		t.Fatalf("series page not shown:\n%s", frame)
	}
	if !strings.Contains(frame, " 10 ") {
		t.Fatalf("episode grid missing:\n%s", frame)
	}
	press(t, a, "l", "l", "enter")
	frame = checkFrame(t, a)
	if !strings.Contains(frame, zhStrings.ToastLocked) {
		t.Fatalf("locked episode should toast:\n%s", frame)
	}
	press(t, a, "l", "enter")
	m := a.env.deps.Media.(*fakeMedia)
	for i := 0; i < 400; i++ {
		m.mu.Lock()
		n := len(m.played)
		m.mu.Unlock()
		if n > 0 {
			break
		}
		waitABit()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.played) != 1 || m.played[0].URL != "https://cdn.invalid/a1-4.mp4" || !m.played[0].NoTerminal {
		t.Fatalf("played: %+v", m.played)
	}
	press(t, a, "esc")
	if len(a.stack) != 0 {
		t.Fatal("esc should pop the page")
	}
}

func TestSettingsCycleAndPersist(t *testing.T) {
	a := newTestApp(t, discoverService())
	drive(t, a, a.Init())
	resize(t, a, 120, 40)
	press(t, a, "0")
	frame := checkFrame(t, a)
	if !strings.Contains(frame, zhStrings.SetLanguage) {
		t.Fatalf("settings not shown:\n%s", frame)
	}
	press(t, a, "l")
	if a.env.settings.Language != "en" || a.env.tr != &enStrings {
		t.Fatalf("language not switched: %+v", *a.env.settings)
	}
	frame = checkFrame(t, a)
	if !strings.Contains(frame, enStrings.SetLanguage) {
		t.Fatalf("labels should be English now:\n%s", frame)
	}
	saved, err := loadSettings(a.settingsPath)
	if err != nil || saved.Language != "en" {
		t.Fatalf("saved=%+v err=%v", saved, err)
	}
}

func TestNarrowScreenHidesPanes(t *testing.T) {
	a := newTestApp(t, discoverService())
	drive(t, a, a.Init())
	resize(t, a, 60, 20)
	press(t, a, "2")
	frame := checkFrame(t, a)
	if strings.Contains(frame, "1 "+zhStrings.SecContinue) {
		t.Fatalf("nav should be hidden at 60 cols:\n%s", frame)
	}
	resize(t, a, 24, 8)
	checkFrame(t, a)
}

func TestHelpOverlayAndQuit(t *testing.T) {
	a := newTestApp(t, discoverService())
	drive(t, a, a.Init())
	resize(t, a, 120, 40)
	press(t, a, "?")
	frame := checkFrame(t, a)
	if !strings.Contains(frame, zhStrings.HelpTitle) {
		t.Fatalf("help not shown:\n%s", frame)
	}
	press(t, a, "esc")
	if a.overlay != nil {
		t.Fatal("overlay should close")
	}
	_, cmd := a.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("q should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q should produce QuitMsg")
	}
}

func TestFormatCount(t *testing.T) {
	cases := []struct {
		n    int64
		lang string
		want string
	}{
		{0, "zh", ""}, {999, "zh", "999"}, {12_345, "zh", "1.2万"}, {120_000_000, "zh", "1.2亿"},
		{12_345, "en", "12.3K"}, {1_200_000, "en", "1.2M"}, {10_000, "zh", "1万"},
	}
	for _, c := range cases {
		if got := formatCount(c.n, c.lang); got != c.want {
			t.Errorf("formatCount(%d,%s)=%q want %q", c.n, c.lang, got, c.want)
		}
	}
	if got := twoColumns("全球死考第一季", "80 集", 20); width(got) != 20 {
		t.Fatalf("twoColumns width %d: %q", width(got), got)
	}
}

func TestStringsComplete(t *testing.T) {
	for _, tr := range []Strings{zhStrings, enStrings} {
		if msg := emptyField(tr); msg != "" {
			t.Fatal(msg)
		}
	}
}
