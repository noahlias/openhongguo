package tui

import (
	"context"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/beilunyang/openhongguo/internal/history"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// Screen is one section or one pushed page. Screens keep their own state
// and mutate it in Update.
type Screen interface {
	Init() tea.Cmd
	Update(msg tea.Msg) tea.Cmd
	// View draws the main pane. Detail draws the right pane, or returns ""
	// to give the main pane the whole width.
	View(w, h int) string
	Detail(w, h int) string
	Title() string
	Keys() []binding
}

// binding is one key hint shown in help.
type binding struct {
	keys string
	help string
}

// env is what every screen shares: the service, the viewer's settings, the
// words and the colors, and the caches the right pane reads from.
type env struct {
	ctx      context.Context
	deps     Deps
	settings *Settings
	tr       *Strings
	theme    *Theme
	seq      *seq
	covers   *covers

	mu      sync.Mutex
	details map[string]*model.Series
	hist    map[string]history.Entry
}

func (e *env) lang() string { return e.settings.Language }

func (e *env) coversOn() bool {
	return e.covers != nil && e.settings.Covers && e.theme.name != "mono"
}

func (e *env) detail(id string) *model.Series {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.details[id]
}

func (e *env) setDetail(id string, s *model.Series) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.details[id] = s
}

func (e *env) watched(id string) (history.Entry, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	h, ok := e.hist[id]
	return h, ok
}

func (e *env) setHistory(entries []history.Entry) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.hist = map[string]history.Entry{}
	for _, h := range entries {
		e.hist[h.SeriesID] = h
	}
}

// seq numbers requests so a late answer to an old request is dropped.
type seq struct {
	mu sync.Mutex
	n  int
}

func (s *seq) next() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.n++
	return s.n
}

// Messages screens send to the app.

type openSeriesMsg struct{ id string }
type openPersonMsg struct{ id string }
type openSearchMsg struct{ query string }
type popScreenMsg struct{}
type pushScreenMsg struct{ screen Screen }

// capturer is a screen that sometimes takes every key, such as a focused
// text box.
type capturer interface{ capturing() bool }
type toastMsg struct {
	text string
	err  bool
}
type playMsg struct {
	seriesID, title string
	episodes        []model.Episode
	start           model.Episode
	startSeconds    int
	autoplay        bool
}
type downloadMsg struct {
	seriesID, title string
	episodes        []model.Episode
	numbers         []int
}
type promptMsg struct {
	id, label, initial string
}
type promptDoneMsg struct {
	id, value string
	ok        bool
}
type reloadHistoryMsg struct{}
type historyListMsg struct {
	entries []history.Entry
	err     error
}
type seriesDetailMsg struct {
	id     string
	series *model.Series
	err    error
}
type detailTickMsg struct{ id string }
type refocusMsg struct{}

func emit(msg tea.Msg) tea.Cmd {
	return func() tea.Msg { return msg }
}

func toast(text string) tea.Cmd    { return emit(toastMsg{text: text}) }
func toastErr(text string) tea.Cmd { return emit(toastMsg{text: text, err: true}) }

// loadDetail fetches the full series for the right pane.
func (e *env) loadDetail(id string) tea.Cmd {
	ctx, svc := e.ctx, e.deps.Service
	return func() tea.Msg {
		s, err := svc.GetSeries(ctx, id)
		return seriesDetailMsg{id: id, series: s, err: err}
	}
}

// loadHistory reads the watch history for the caches and the continue
// section.
func (e *env) loadHistory() tea.Cmd {
	if e.deps.History == nil {
		return nil
	}
	ctx, h := e.ctx, e.deps.History
	return func() tea.Msg {
		entries, err := h.List(ctx)
		return historyListMsg{entries: entries, err: err}
	}
}
