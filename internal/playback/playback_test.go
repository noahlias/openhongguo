package playback

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/beilunyang/openhongguo/internal/history"
	"github.com/beilunyang/openhongguo/internal/media"
	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

type fakeService struct {
	mu        sync.Mutex
	playbacks map[string]*model.Playback
	errs      map[string]error
	resolved  []string
}

func (f *fakeService) ListEpisodes(context.Context, string, sdk.EpisodeOptions) (*sdk.Page[model.Episode], error) {
	return nil, errors.New("unused")
}

func (f *fakeService) GetSeries(context.Context, string) (*model.Series, error) {
	return nil, errors.New("unused")
}

func (f *fakeService) ResolvePlayback(_ context.Context, id string, _ sdk.PlaybackOptions) (*model.Playback, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resolved = append(f.resolved, id)
	if err, ok := f.errs[id]; ok {
		return nil, err
	}
	pb, ok := f.playbacks[id]
	if !ok {
		return nil, sdk.ErrContentUnavailable
	}
	return pb, nil
}

type fakeMedia struct {
	mu      sync.Mutex
	played  []media.Request
	results []media.Result
	errs    []error
	opened  []string
}

func (m *fakeMedia) Open(_ context.Context, url, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.opened = append(m.opened, url)
	return nil
}

func (m *fakeMedia) Save(_ context.Context, _, path string) (int64, error) {
	if err := os.WriteFile(path, []byte("video"), 0o600); err != nil {
		return 0, err
	}
	return 5, nil
}

func (m *fakeMedia) Decrypt(_ context.Context, _, _, _, path string, _ int) (int64, error) {
	return m.Save(context.Background(), "", path)
}

func (m *fakeMedia) Play(_ context.Context, req media.Request) (media.Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	i := len(m.played)
	m.played = append(m.played, req)
	var res media.Result
	var err error
	if i < len(m.results) {
		res = m.results[i]
	}
	if i < len(m.errs) {
		err = m.errs[i]
	}
	return res, err
}

type memHistory struct {
	mu      sync.Mutex
	entries map[string]history.Entry
	fail    bool
}

func (h *memHistory) Get(_ context.Context, id string) (history.Entry, bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	e, ok := h.entries[id]
	return e, ok, nil
}

func (h *memHistory) Record(_ context.Context, e history.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.fail {
		return errors.New("disk full")
	}
	if h.entries == nil {
		h.entries = map[string]history.Entry{}
	}
	h.entries[e.SeriesID] = e
	return nil
}

func clearPlayback(id string) *model.Playback {
	return &model.Playback{
		EpisodeID: id, DurationSeconds: 100,
		Streams: []model.Stream{{
			Definition: "720p", URL: "https://cdn.invalid/" + id + ".mp4", BackupURL: "https://backup.invalid/" + id + ".mp4",
		}},
	}
}

func episodes(n int) []model.Episode {
	out := make([]model.Episode, n)
	for i := range out {
		out[i] = model.Episode{ID: "ep" + string(rune('0'+i+1)), SeriesID: "s1", Number: i + 1, Playable: true}
	}
	return out
}

func service(eps []model.Episode) *fakeService {
	f := &fakeService{playbacks: map[string]*model.Playback{}, errs: map[string]error{}}
	for _, e := range eps {
		f.playbacks[e.ID] = clearPlayback(e.ID)
	}
	return f
}

func collect(events *[]Event) func(Event) {
	return func(e Event) { *events = append(*events, e) }
}

func kinds(events []Event) string {
	names := map[EventKind]string{
		EventStarted: "started", EventFinished: "finished", EventStopped: "stopped", EventJumped: "jumped",
		EventSkipped: "skipped", EventBackupRetry: "retry", EventHistoryWarning: "warning",
	}
	var out []string
	for _, e := range events {
		out = append(out, names[e.Kind])
	}
	return strings.Join(out, " ")
}

func TestResolveKeepsProtectedModel(t *testing.T) {
	svc := &fakeService{playbacks: map[string]*model.Playback{}}
	svc.playbacks["ep1"] = clearPlayback("ep1")
	svc.playbacks["ep1"].Protected = true
	svc.errs = map[string]error{}
	d := Deps{Service: &protectedService{svc}}
	pb, err := Resolve(context.Background(), d, "ep1")
	if err != nil || pb == nil || !pb.Protected {
		t.Fatalf("pb=%v err=%v", pb, err)
	}
}

// protectedService answers like the SDK does for an encrypted episode: the
// model together with ErrDRMProtected.
type protectedService struct{ *fakeService }

func (p *protectedService) ResolvePlayback(ctx context.Context, id string, o sdk.PlaybackOptions) (*model.Playback, error) {
	pb, err := p.fakeService.ResolvePlayback(ctx, id, o)
	if err != nil {
		return nil, err
	}
	return pb, sdk.ErrDRMProtected
}

func TestSessionAutoplayRecordsEveryEpisode(t *testing.T) {
	eps := episodes(3)
	svc, m, h := service(eps), &fakeMedia{}, &memHistory{}
	var events []Event
	s := NewSession(Deps{Service: svc, Media: m, History: h}, Options{Autoplay: true, Player: "mpv", NoTerminal: true}, collect(&events))
	err := s.Run(context.Background(), Target{SeriesID: "s1", Title: "剧", Episodes: eps, Start: eps[0], Remember: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := kinds(events); got != "started finished started finished started finished" {
		t.Fatalf("events: %s", got)
	}
	if len(m.played) != 3 || m.played[2].URL != "https://cdn.invalid/ep3.mp4" || !m.played[0].NoTerminal {
		t.Fatalf("played: %+v", m.played)
	}
	e := h.entries["s1"]
	if e.Episode != 3 || !e.Finished || e.Episodes != 3 || e.Title != "剧" {
		t.Fatalf("history: %+v", e)
	}
}

func TestSessionStopRecordsPosition(t *testing.T) {
	eps := episodes(2)
	svc, h := service(eps), &memHistory{}
	m := &fakeMedia{results: []media.Result{{Position: 42}}, errs: []error{media.ErrStopped}}
	var events []Event
	s := NewSession(Deps{Service: svc, Media: m, History: h}, Options{Autoplay: true}, collect(&events))
	if err := s.Run(context.Background(), Target{SeriesID: "s1", Episodes: eps, Start: eps[0], Remember: true}); err != nil {
		t.Fatal(err)
	}
	if got := kinds(events); got != "started stopped" {
		t.Fatalf("events: %s", got)
	}
	if e := h.entries["s1"]; e.Episode != 1 || e.Finished || e.Position != 42 {
		t.Fatalf("history: %+v", e)
	}
}

func TestSessionJumpSwitchesEpisode(t *testing.T) {
	eps := episodes(3)
	svc := service(eps)
	m := &fakeMedia{results: []media.Result{{Position: 5, Jump: media.JumpNext}, {}}}
	var events []Event
	s := NewSession(Deps{Service: svc, Media: m}, Options{}, collect(&events))
	if err := s.Run(context.Background(), Target{Episodes: eps, Start: eps[0], StartSeconds: 30}); err != nil {
		t.Fatal(err)
	}
	if got := kinds(events); got != "started jumped started finished" {
		t.Fatalf("events: %s", got)
	}
	if m.played[0].StartSeconds != 30 || m.played[1].StartSeconds != 0 || m.played[1].URL != "https://cdn.invalid/ep2.mp4" {
		t.Fatalf("played: %+v", m.played)
	}
}

func TestSessionSkipsUnavailableEpisodes(t *testing.T) {
	eps := episodes(3)
	svc := service(eps)
	svc.errs["ep2"] = sdk.ErrAuthenticationRequired
	m := &fakeMedia{}
	var events []Event
	s := NewSession(Deps{Service: svc, Media: m}, Options{Autoplay: true}, collect(&events))
	if err := s.Run(context.Background(), Target{Episodes: eps, Start: eps[0]}); err != nil {
		t.Fatal(err)
	}
	if got := kinds(events); got != "started finished started skipped started finished" {
		t.Fatalf("events: %s", got)
	}
	if len(m.played) != 2 {
		t.Fatalf("played %d episodes", len(m.played))
	}
}

func TestSessionGivesUpOnLockedRun(t *testing.T) {
	eps := episodes(4)
	svc := service(eps)
	for _, id := range []string{"ep1", "ep2", "ep3"} {
		svc.errs[id] = sdk.ErrAnonymousLimited
	}
	s := NewSession(Deps{Service: svc, Media: &fakeMedia{}}, Options{Autoplay: true}, nil)
	err := s.Run(context.Background(), Target{Episodes: eps, Start: eps[0]})
	if !errors.Is(err, sdk.ErrAnonymousLimited) || !strings.Contains(err.Error(), "3 episodes in a row unavailable") {
		t.Fatalf("err=%v", err)
	}
}

func TestSessionWithoutAutoplayReturnsError(t *testing.T) {
	eps := episodes(1)
	svc := service(eps)
	svc.errs["ep1"] = sdk.ErrAuthenticationRequired
	s := NewSession(Deps{Service: svc, Media: &fakeMedia{}}, Options{}, nil)
	if err := s.Run(context.Background(), Target{Episodes: eps, Start: eps[0]}); !errors.Is(err, sdk.ErrAuthenticationRequired) {
		t.Fatalf("err=%v", err)
	}
}

func TestSessionRetriesBackupURL(t *testing.T) {
	eps := episodes(1)
	m := &fakeMedia{errs: []error{errors.New("cdn down"), nil}}
	var events []Event
	s := NewSession(Deps{Service: service(eps), Media: m}, Options{}, collect(&events))
	if err := s.Run(context.Background(), Target{Episodes: eps, Start: eps[0]}); err != nil {
		t.Fatal(err)
	}
	if got := kinds(events); got != "started retry finished" {
		t.Fatalf("events: %s", got)
	}
	if len(m.played) != 2 || m.played[1].URL != "https://backup.invalid/ep1.mp4" {
		t.Fatalf("played: %+v", m.played)
	}
}

func TestSessionHistoryFailureOnlyWarns(t *testing.T) {
	eps := episodes(1)
	var events []Event
	s := NewSession(Deps{Service: service(eps), Media: &fakeMedia{}, History: &memHistory{fail: true}}, Options{}, collect(&events))
	if err := s.Run(context.Background(), Target{SeriesID: "s1", Episodes: eps, Start: eps[0], Remember: true}); err != nil {
		t.Fatal(err)
	}
	if got := kinds(events); got != "started warning finished warning" {
		t.Fatalf("events: %s", got)
	}
}

func TestSessionNeedsKeyForProtectedStream(t *testing.T) {
	eps := episodes(1)
	svc := service(eps)
	svc.playbacks["ep1"].Protected = true
	s := NewSession(Deps{Service: svc, Media: &fakeMedia{}}, Options{}, nil)
	err := s.Run(context.Background(), Target{Episodes: eps, Start: eps[0]})
	if !errors.Is(err, sdk.ErrDRMProtected) {
		t.Fatalf("err=%v", err)
	}
	s = NewSession(Deps{Service: svc, Media: &fakeMedia{}}, Options{Key: strings.Repeat("ab", 16)}, nil)
	if err := s.Run(context.Background(), Target{Episodes: eps, Start: eps[0]}); err != nil {
		t.Fatalf("with a key err=%v", err)
	}
	svc.playbacks["ep1"].Protected = false
	s = NewSession(Deps{Service: svc, Media: &fakeMedia{}}, Options{Key: strings.Repeat("ab", 16)}, nil)
	if err := s.Run(context.Background(), Target{Episodes: eps, Start: eps[0]}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("key on a clear stream err=%v", err)
	}
}

func TestDownloadSkipsSavedAndReportsEach(t *testing.T) {
	eps := episodes(2)
	dir := t.TempDir()
	existing := EpisodePath(dir, 1, 2)
	if err := os.WriteFile(existing, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	var events []DownloadEvent
	job := DownloadJob{Episodes: eps, Numbers: []int{1, 2}, Dir: dir, Jobs: 2}
	failed, err := Download(context.Background(), Deps{Service: service(eps), Media: &fakeMedia{}}, job, func(e DownloadEvent) {
		events = append(events, e)
	})
	if err != nil || failed != 0 {
		t.Fatalf("failed=%d err=%v", failed, err)
	}
	if len(events) != 2 {
		t.Fatalf("events: %+v", events)
	}
	for _, e := range events {
		switch e.Episode.Number {
		case 1:
			if !e.Skipped || e.Path != existing {
				t.Fatalf("episode 1: %+v", e)
			}
		case 2:
			if e.Skipped || e.Written != 5 || filepath.Base(e.Path) != "第002集.mp4" {
				t.Fatalf("episode 2: %+v", e)
			}
			if _, err := os.Stat(e.Path); err != nil {
				t.Fatal(err)
			}
		}
	}
	if parts, _ := filepath.Glob(filepath.Join(dir, "*.part")); len(parts) != 0 {
		t.Fatalf("leftover parts: %v", parts)
	}
}

func TestDownloadCountsFailures(t *testing.T) {
	eps := episodes(2)
	svc := service(eps)
	svc.errs["ep2"] = sdk.ErrContentUnavailable
	var events []DownloadEvent
	job := DownloadJob{Episodes: eps, Numbers: []int{1, 2}, Dir: t.TempDir(), Jobs: 1}
	failed, err := Download(context.Background(), Deps{Service: svc, Media: &fakeMedia{}}, job, func(e DownloadEvent) {
		events = append(events, e)
	})
	if err != nil || failed != 1 {
		t.Fatalf("failed=%d err=%v", failed, err)
	}
	if len(events) != 2 || events[1].Err == nil || !errors.Is(events[1].Err, sdk.ErrContentUnavailable) {
		t.Fatalf("events: %+v", events)
	}
}

func TestResumePoint(t *testing.T) {
	h := &memHistory{entries: map[string]history.Entry{
		"open": {SeriesID: "open", Episode: 4, Position: 10},
		"near": {SeriesID: "near", Episode: 4, Position: 1},
		"done": {SeriesID: "done", Episode: 2, Finished: true},
		"last": {SeriesID: "last", Episode: 5, Finished: true},
	}}
	ctx := context.Background()
	if n, pos, err := ResumePoint(ctx, h, "new", 5); err != nil || n != 1 || pos != 0 {
		t.Fatalf("new: %d %d %v", n, pos, err)
	}
	if n, pos, err := ResumePoint(ctx, h, "open", 5); err != nil || n != 4 || pos != 7 {
		t.Fatalf("open: %d %d %v", n, pos, err)
	}
	if n, pos, err := ResumePoint(ctx, h, "near", 5); err != nil || n != 4 || pos != 0 {
		t.Fatalf("near: %d %d %v", n, pos, err)
	}
	if n, pos, err := ResumePoint(ctx, h, "done", 5); err != nil || n != 3 || pos != 0 {
		t.Fatalf("done: %d %d %v", n, pos, err)
	}
	if _, _, err := ResumePoint(ctx, h, "last", 5); !errors.Is(err, ErrInvalid) {
		t.Fatalf("last: %v", err)
	}
}

func TestParseEpisodeRange(t *testing.T) {
	got, err := ParseEpisodeRange("3, 1-2,2", 5)
	if err != nil || len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Fatalf("got %v, %v", got, err)
	}
	if all, _ := ParseEpisodeRange("", 4); len(all) != 4 || all[3] != 4 {
		t.Fatalf("all = %v", all)
	}
	for _, bad := range []string{"0", "3-1", "x", "6", "1-9"} {
		if _, err := ParseEpisodeRange(bad, 5); !errors.Is(err, ErrInvalid) {
			t.Fatalf("%q: %v", bad, err)
		}
	}
}

func TestEpisodePathAndDefaultDir(t *testing.T) {
	if got := filepath.Base(EpisodePath("d", 7, 80)); got != "第007集.mp4" {
		t.Fatal(got)
	}
	if got := filepath.Base(EpisodePath("d", 7, 1000)); got != "第0007集.mp4" {
		t.Fatal(got)
	}
	if got := DefaultDir("剧/名: 第一季", "123"); got != "剧_名_ 第一季" {
		t.Fatal(got)
	}
	if got := DefaultDir("...", "123"); got != "123" {
		t.Fatal(got)
	}
}
