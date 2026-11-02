package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// newSection is the release calendar: a strip of dates and the day's
// releases, online and upcoming.
type newSection struct {
	e     *env
	dates tabs
	raw   []string
	date  string
	f     feed
}

type newReleasesMsg struct {
	req  int
	page *model.NewReleasePage
	err  error
}

func newNewSection(e *env) *newSection { return &newSection{e: e} }

func (s *newSection) Init() tea.Cmd { return s.load(true) }
func (s *newSection) Title() string { return s.e.tr.SecNew }
func (s *newSection) Keys() []binding {
	tr := s.e.tr
	return []binding{{"[ ] h l", tr.Date}, {"enter", tr.KeyOpen}, {"R", tr.KeyRefresh}}
}

func (s *newSection) load(fresh bool) tea.Cmd {
	if fresh {
		s.f.reset()
	}
	req := s.f.begin(s.e.seq)
	q := sdk.NewReleaseQuery{Date: s.date, Cursor: s.f.cursor}
	ctx, svc := s.e.ctx, s.e.deps.Service
	return func() tea.Msg {
		page, err := svc.NewReleases(ctx, q)
		return newReleasesMsg{req: req, page: page, err: err}
	}
}

func (s *newSection) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case newReleasesMsg:
		if !s.f.accept(msg.req) {
			return nil
		}
		if msg.err != nil {
			s.f.err = msg.err
			s.f.hasMore = false
			return nil
		}
		s.applyDates(msg.page)
		rows := make([]row, 0, len(msg.page.Items))
		for _, it := range msg.page.Items {
			rows = append(rows, releaseRow{item: it, tr: s.e.tr})
		}
		s.f.list.add(rows)
		s.f.page(msg.page.Cursor, msg.page.HasMore)
		return s.f.afterMove(s.e, s.f.selectedSeries())
	case detailTickMsg:
		return s.f.onDetailTick(s.e, msg.id)
	case tea.KeyPressMsg:
		k := keyString(msg)
		if s.dates.handleKey(k) {
			s.date = s.raw[s.dates.active]
			return s.load(true)
		}
		switch k {
		case "enter":
			if id := s.f.selectedSeries(); id != "" {
				return emit(openSeriesMsg{id: id})
			}
			return nil
		case "R":
			return s.load(true)
		}
		if s.f.list.handleKey(k, 10) {
			var cmd tea.Cmd
			if s.f.wantMore() {
				cmd = s.load(false)
			}
			return tea.Batch(cmd, s.f.afterMove(s.e, s.f.selectedSeries()))
		}
	}
	return nil
}

func (s *newSection) applyDates(page *model.NewReleasePage) {
	if len(page.Dates) == 0 {
		return
	}
	s.raw = page.Dates
	names := make([]string, len(s.raw))
	active := 0
	for i, d := range s.raw {
		names[i] = shortDate(d)
		if d == page.Date {
			active = i
		}
	}
	s.dates.set(names, active)
	s.date = page.Date
}

// shortDate turns 20261026 into 10-26.
func shortDate(d string) string {
	if len(d) == 8 {
		return d[4:6] + "-" + d[6:]
	}
	return d
}

func (s *newSection) View(w, h int) string {
	th := s.e.theme
	lines := []string{s.dates.view(w, th), s.f.list.view(w, max(h-2, 1), th, true), pad(s.f.statusLine(s.e.tr, th), w)}
	return strings.Join(lines, "\n")
}

func (s *newSection) Detail(w, h int) string {
	r, ok := s.f.list.selected().(releaseRow)
	if !ok {
		return ""
	}
	it := r.item
	th, tr := s.e.theme, s.e.tr
	state := tr.Upcoming
	if it.Online {
		state = tr.Online
	}
	when := ""
	if !it.PublishAt.IsZero() {
		when = it.PublishAt.Format("01-02 15:04")
	}
	extra := []string{clip(joinNonEmpty(" · ", th.Accent.Render(state), when, th.Heat.Render(it.Heat)), w)}
	if len(it.Categories) > 0 {
		extra = append(extra, clip(th.Subtle.Render(strings.Join(it.Categories, " · ")), w))
	}
	if it.Note != "" {
		extra = append(extra, clip(th.Subtle.Render(it.Note), w))
	}
	sum := model.SeriesSummary{ID: it.SeriesID, Title: it.Title, Description: it.Description, CoverURL: it.CoverURL, EpisodeCount: it.EpisodeCount}
	body := renderSeriesDetail(s.e, w, max(h-len(extra)-1, 1), sum)
	return strings.Join(extra, "\n") + "\n\n" + body
}
