package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/beilunyang/openhongguo/sdk/model"
)

// hotSection shows the hot searches: the guesses, then each board with its
// series and topics, as one list under headings.
type hotSection struct {
	e *env
	f feed
}

type hotLoadedMsg struct {
	req int
	hot *model.HotSearch
	err error
}

// topicRow is a hot topic; opening it searches for its query.
type topicRow struct {
	topic model.HotTopic
	board int
	index int
}

func (r topicRow) key() string { return "topic:" + itoa(r.board) + ":" + itoa(r.index) }

func (r topicRow) render(w int, _ bool, th *Theme) string {
	left := th.Subtle.Render(padLeft(itoa(r.index), 3)) + " # " + r.topic.Title
	return twoColumns(left, th.Heat.Render(r.topic.Heat), w)
}

func newHotSection(e *env) *hotSection { return &hotSection{e: e} }

func (s *hotSection) Init() tea.Cmd { return s.load() }
func (s *hotSection) Title() string { return s.e.tr.SecHot }
func (s *hotSection) Keys() []binding {
	tr := s.e.tr
	return []binding{{"enter", tr.KeyOpen}, {"R", tr.KeyRefresh}}
}

func (s *hotSection) load() tea.Cmd {
	s.f.reset()
	req := s.f.begin(s.e.seq)
	ctx, svc := s.e.ctx, s.e.deps.Service
	return func() tea.Msg {
		hot, err := svc.HotSearch(ctx)
		return hotLoadedMsg{req: req, hot: hot, err: err}
	}
}

func (s *hotSection) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case hotLoadedMsg:
		if !s.f.accept(msg.req) {
			return nil
		}
		if msg.err != nil {
			s.f.err = msg.err
			return nil
		}
		s.f.list.setAll(hotRows(msg.hot, s.e.tr))
		s.skipHeadings(1)
		return s.f.afterMove(s.e, s.f.selectedSeries())
	case detailTickMsg:
		return s.f.onDetailTick(s.e, msg.id)
	case tea.KeyPressMsg:
		k := keyString(msg)
		before := s.f.list.cursor
		if s.f.list.handleKey(k, 10) {
			dir := 1
			if s.f.list.cursor < before {
				dir = -1
			}
			s.skipHeadings(dir)
			return s.f.afterMove(s.e, s.f.selectedSeries())
		}
		switch k {
		case "enter":
			switch r := s.f.list.selected().(type) {
			case seriesRow:
				return emit(openSeriesMsg{id: r.item.ID})
			case topicRow:
				q := r.topic.Query
				if q == "" {
					q = r.topic.Title
				}
				return emit(openSearchMsg{query: q})
			}
		case "R":
			return s.load()
		}
	}
	return nil
}

// skipHeadings moves the cursor off a heading, in dir when it can.
func (s *hotSection) skipHeadings(dir int) {
	l := &s.f.list
	for range 2 {
		for l.cursor >= 0 && l.cursor < l.len() {
			if _, heading := l.selected().(textRow); !heading {
				return
			}
			l.cursor += dir
		}
		l.clampCursor()
		dir = -dir
	}
}

func hotRows(hot *model.HotSearch, tr *Strings) []row {
	var rows []row
	head := func(id, text string) { rows = append(rows, heading(id, text)) }
	item := func(it model.HotItem, prefix string) {
		sum := model.SeriesSummary{ID: it.SeriesID, Title: it.Title, CoverURL: it.CoverURL, CoverExpiresAt: it.CoverExpiresAt, EpisodeCount: it.EpisodeCount}
		score := it.SearchScore
		if score == "" {
			score = it.Heat
		}
		rows = append(rows, seriesRow{item: sum, prefix: prefix, tag: it.Tag, note: score, tr: tr})
	}
	if len(hot.Guesses) > 0 {
		head("guess", tr.Guesses)
		for _, it := range hot.Guesses {
			item(it, "  ·")
		}
	}
	for bi, b := range hot.Boards {
		if len(b.Items) == 0 && len(b.Topics) == 0 {
			continue
		}
		head(itoa(bi), b.Name)
		for _, it := range b.Items {
			item(it, padLeft(itoa(it.Position), 3))
		}
		for i, t := range b.Topics {
			rows = append(rows, topicRow{topic: t, board: bi, index: i + 1})
		}
	}
	return rows
}

func (s *hotSection) View(w, h int) string {
	th := s.e.theme
	return s.f.list.view(w, h-1, th, true) + "\n" + pad(s.f.statusLine(s.e.tr, th), w)
}

func (s *hotSection) Detail(w, h int) string {
	r, ok := s.f.list.selected().(seriesRow)
	if !ok {
		return ""
	}
	return renderSeriesDetail(s.e, w, h, r.item)
}
