package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/beilunyang/openhongguo/internal/history"
	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// continueSection lists the watched series, most recent first, and resumes
// one with a key press.
type continueSection struct {
	e       *env
	f       feed
	entries []history.Entry
	// full lists everything; the continue section shows unfinished first.
	full bool
}

func newContinueSection(e *env, full bool) *continueSection {
	return &continueSection{e: e, full: full}
}

func (s *continueSection) Init() tea.Cmd {
	s.f.begin(s.e.seq)
	return s.e.loadHistory()
}

func (s *continueSection) Title() string {
	if s.full {
		return s.e.tr.SecHistory
	}
	return s.e.tr.SecContinue
}

func (s *continueSection) Keys() []binding {
	tr := s.e.tr
	keys := []binding{
		{"enter", tr.KeyOpen}, {"r", tr.KeyResume}, {"R", tr.KeyRefresh},
	}
	if s.full {
		keys = append(keys, binding{"D", tr.KeyClear})
	}
	return keys
}

func (s *continueSection) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case historyListMsg:
		s.f.loading = false
		s.f.err = msg.err
		s.entries = msg.entries
		s.rebuild()
		return s.f.afterMove(s.e, s.f.selectedSeries())
	case detailTickMsg:
		return s.f.onDetailTick(s.e, msg.id)
	case promptDoneMsg:
		if msg.id == "clear-history" && msg.ok && (msg.value == "yes" || msg.value == s.e.tr.Yes) {
			return s.clear()
		}
	case tea.KeyPressMsg:
		k := keyString(msg)
		if s.f.list.handleKey(k, 10) {
			return s.f.afterMove(s.e, s.f.selectedSeries())
		}
		switch k {
		case "enter", "l", "right":
			if id := s.f.selectedSeries(); id != "" {
				return emit(openSeriesMsg{id: id})
			}
		case "r":
			return s.resume()
		case "R":
			s.f.begin(s.e.seq)
			return s.e.loadHistory()
		case "D":
			if s.full {
				return emit(promptMsg{id: "clear-history", label: s.e.tr.PromptConfirmClear})
			}
		}
	}
	return nil
}

func (s *continueSection) rebuild() {
	rows := make([]row, 0, len(s.entries))
	for _, h := range s.entries {
		if !s.full && h.Finished && h.Episodes > 0 && h.Episode >= h.Episodes {
			continue
		}
		rows = append(rows, historyRow{entry: h, tr: s.e.tr})
	}
	cur := s.f.list.cursor
	s.f.list.set(rows)
	s.f.list.cursor = cur
	s.f.list.clampCursor()
}

// resume plays the highlighted series from where it was left.
func (s *continueSection) resume() tea.Cmd {
	r, ok := s.f.list.selected().(historyRow)
	if !ok {
		return nil
	}
	e := s.e
	ctx, svc := e.ctx, e.deps.Service
	id, title := r.entry.SeriesID, r.entry.Title
	return func() tea.Msg {
		page, err := svc.ListEpisodes(ctx, id, sdk.EpisodeOptions{})
		if err != nil {
			return toastMsg{text: describeError(err, e.tr), err: true}
		}
		return resumeFromHistory(e, id, title, page.Items)
	}
}

func (s *continueSection) clear() tea.Cmd {
	e := s.e
	if e.deps.History == nil {
		return nil
	}
	ctx, h := e.ctx, e.deps.History
	return func() tea.Msg {
		if err := h.Clear(ctx); err != nil {
			return toastMsg{text: fmt.Sprintf("%s: %v", e.tr.Failed, err), err: true}
		}
		return clearedHistoryMsg{}
	}
}

type clearedHistoryMsg struct{}

func (s *continueSection) View(w, h int) string {
	tr, th := s.e.tr, s.e.theme
	if s.f.list.len() == 0 && !s.f.loading {
		msg := tr.EmptyContinue
		if s.full {
			msg = tr.EmptyHistory
		}
		if s.f.err != nil {
			msg = describeError(s.f.err, tr)
		}
		return th.Subtle.Render(clip(msg, w))
	}
	body := s.f.list.view(w, h-1, th, true)
	return body + "\n" + pad(s.f.statusLine(tr, th), w)
}

func (s *continueSection) Detail(w, h int) string {
	r, ok := s.f.list.selected().(historyRow)
	if !ok {
		return ""
	}
	sum := model.SeriesSummary{ID: r.entry.SeriesID, Title: r.entry.Title, EpisodeCount: r.entry.Episodes}
	if full := s.e.detail(sum.ID); full != nil {
		sum = full.SeriesSummary
	}
	return renderSeriesDetail(s.e, w, h, sum)
}
