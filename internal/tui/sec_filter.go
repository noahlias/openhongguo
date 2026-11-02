package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// filterSection picks a genre and filter options as chips and lists what
// matches.
type filterSection struct {
	e        *env
	genre    tabs
	panel    chips
	panelReq int
	panelErr error
	f        feed
	// focus is 0 for the chips and 1 for the results.
	focus int
}

type filterPanelMsg struct {
	req  int
	rows []model.FilterRow
	err  error
}
type filterPageMsg struct {
	req  int
	page *model.ShortFilterPage
	err  error
}

func newFilterSection(e *env) *filterSection {
	s := &filterSection{e: e}
	s.genre.set(genreNames(e.lang()), 0)
	return s
}

func genreNames(lang string) []string {
	out := make([]string, len(filterGenres))
	for i, g := range filterGenres {
		out[i] = genreLabel(g, lang)
	}
	return out
}

func (s *filterSection) Init() tea.Cmd { return s.loadPanel() }
func (s *filterSection) Title() string { return s.e.tr.SecFilter }
func (s *filterSection) Keys() []binding {
	tr := s.e.tr
	return []binding{
		{"[ ]", tr.KeyTabs}, {"h j k l", tr.KeyMove}, {"space", tr.KeyToggle}, {"x", tr.KeyClear},
		{"tab", tr.KeyFocus}, {"enter", tr.KeyOpen},
	}
}

func (s *filterSection) loadPanel() tea.Cmd {
	s.genre.set(genreNames(s.e.lang()), s.genre.active)
	s.panel.set(nil)
	s.focus = 0
	s.panelReq = s.e.seq.next()
	s.panelErr = nil
	req, genre := s.panelReq, filterGenres[s.genre.active]
	ctx, svc := s.e.ctx, s.e.deps.Service
	return func() tea.Msg {
		rows, err := svc.ShortFilterPanel(ctx, genre)
		return filterPanelMsg{req: req, rows: rows, err: err}
	}
}

func (s *filterSection) loadResults(fresh bool) tea.Cmd {
	if fresh {
		s.f.reset()
	}
	req := s.f.begin(s.e.seq)
	sel := map[string][]string{"genre": {filterGenres[s.genre.active]}}
	for k, v := range s.panel.selections() {
		sel[k] = []string{v}
	}
	q := sdk.ShortFilterQuery{Selections: sel, Cursor: s.f.cursor}
	ctx, svc := s.e.ctx, s.e.deps.Service
	return func() tea.Msg {
		page, err := svc.ShortFilter(ctx, q)
		return filterPageMsg{req: req, page: page, err: err}
	}
}

func (s *filterSection) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case filterPanelMsg:
		if msg.req != s.panelReq {
			return nil
		}
		if msg.err != nil {
			s.panelErr = msg.err
			return nil
		}
		rows := make([]chipRow, 0, len(msg.rows))
		for _, r := range msg.rows {
			if r.Key == "genre" || len(r.Options) == 0 {
				continue
			}
			rows = append(rows, chipRow{key: r.Key, name: r.Name, options: r.Options})
		}
		s.panel.set(rows)
		if s.panel.empty() {
			s.focus = 1
		}
		return s.loadResults(true)
	case filterPageMsg:
		if !s.f.accept(msg.req) {
			return nil
		}
		if msg.err != nil {
			s.f.err = msg.err
			s.f.hasMore = false
			return nil
		}
		rows := make([]row, 0, len(msg.page.Items))
		for _, it := range msg.page.Items {
			rows = append(rows, seriesRow{item: it, tr: s.e.tr})
		}
		s.f.list.add(rows)
		s.f.page(msg.page.Cursor, msg.page.HasMore)
		return s.f.afterMove(s.e, s.f.selectedSeries())
	case detailTickMsg:
		return s.f.onDetailTick(s.e, msg.id)
	case tea.KeyPressMsg:
		return s.key(keyString(msg))
	}
	return nil
}

func (s *filterSection) key(k string) tea.Cmd {
	switch k {
	case "[", "]":
		by := 1
		if k == "[" {
			by = -1
		}
		if s.genre.move(by) {
			s.f.reset()
			return s.loadPanel()
		}
		return nil
	case "tab":
		if !s.panel.empty() {
			s.focus = 1 - s.focus
		}
		return nil
	}
	if s.focus == 0 {
		handled, changed := s.panel.handleKey(k)
		if changed {
			return s.loadResults(true)
		}
		if !handled && (k == "G" || k == "end") {
			s.focus = 1
		}
		return nil
	}
	if k == "enter" {
		if id := s.f.selectedSeries(); id != "" {
			return emit(openSeriesMsg{id: id})
		}
		return nil
	}
	if (k == "k" || k == "up") && s.f.list.cursor == 0 && !s.panel.empty() {
		s.focus = 0
		return nil
	}
	if s.f.list.handleKey(k, 10) {
		var cmd tea.Cmd
		if s.f.wantMore() {
			cmd = s.loadResults(false)
		}
		return tea.Batch(cmd, s.f.afterMove(s.e, s.f.selectedSeries()))
	}
	return nil
}

func (s *filterSection) View(w, h int) string {
	th, tr := s.e.theme, s.e.tr
	lines := []string{s.genre.view(w, th)}
	if s.panelErr != nil {
		lines = append(lines, th.ToastErr.Render(clip(describeError(s.panelErr, tr), w)))
	} else {
		lines = append(lines, s.panel.view(w, th, s.focus == 0)...)
	}
	lines = append(lines, th.Border.Render(strings.Repeat("─", w)))
	lines = append(lines, s.f.list.view(w, max(h-len(lines)-1, 1), th, s.focus == 1), pad(s.f.statusLine(tr, th), w))
	return strings.Join(lines, "\n")
}

func (s *filterSection) Detail(w, h int) string {
	r, ok := s.f.list.selected().(seriesRow)
	if !ok {
		return ""
	}
	return renderSeriesDetail(s.e, w, h, r.item)
}
