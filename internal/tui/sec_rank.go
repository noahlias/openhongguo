package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// rankSection shows the short-drama rankings with a content switcher and a
// board switcher; celebrity boards list people.
type rankSection struct {
	e        *env
	contents []model.RankContent
	content  tabs
	board    tabs
	f        feed
}

type shortRankMsg struct {
	req  int
	rank *model.ShortRanking
	err  error
}

func newRankSection(e *env) *rankSection { return &rankSection{e: e} }

func (s *rankSection) Init() tea.Cmd { return s.load(true) }
func (s *rankSection) Title() string { return s.e.tr.SecRank }
func (s *rankSection) Keys() []binding {
	tr := s.e.tr
	return []binding{{"c", tr.KeyContent}, {"[ ] b", tr.KeyBoard}, {"enter", tr.KeyOpen}, {"R", tr.KeyRefresh}}
}

func (s *rankSection) contentID() string {
	if s.content.active < len(s.contents) {
		return s.contents[s.content.active].ID
	}
	return ""
}

func (s *rankSection) boardID() string {
	if s.content.active < len(s.contents) {
		boards := s.contents[s.content.active].Boards
		if s.board.active < len(boards) {
			return boards[s.board.active].ID
		}
	}
	return ""
}

func (s *rankSection) load(fresh bool) tea.Cmd {
	if fresh {
		s.f.reset()
	}
	req := s.f.begin(s.e.seq)
	ctx, svc := s.e.ctx, s.e.deps.Service
	q := sdk.ShortRankQuery{Content: s.contentID(), Board: s.boardID(), Cursor: s.f.cursor}
	return func() tea.Msg {
		r, err := svc.ShortRank(ctx, q)
		return shortRankMsg{req: req, rank: r, err: err}
	}
}

func (s *rankSection) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case shortRankMsg:
		if !s.f.accept(msg.req) {
			return nil
		}
		if msg.err != nil {
			s.f.err = msg.err
			s.f.hasMore = false
			return nil
		}
		s.applyCatalog(msg.rank)
		s.f.list.add(rankRows(msg.rank.Items, s.e))
		s.f.page(msg.rank.Cursor, msg.rank.HasMore)
		return s.f.afterMove(s.e, s.f.selectedSeries())
	case detailTickMsg:
		return s.f.onDetailTick(s.e, msg.id)
	case tea.KeyPressMsg:
		return s.key(keyString(msg))
	}
	return nil
}

func (s *rankSection) key(k string) tea.Cmd {
	switch k {
	case "c":
		if s.content.move(1) {
			s.board.set(s.boardNames(), 0)
			return s.load(true)
		}
		return nil
	case "b", "]", "l", "right":
		if s.board.move(1) {
			return s.load(true)
		}
		return nil
	case "[", "h", "left":
		if s.board.move(-1) {
			return s.load(true)
		}
		return nil
	case "R":
		return s.load(true)
	case "enter":
		switch r := s.f.list.selected().(type) {
		case seriesRow:
			return emit(openSeriesMsg{id: r.item.ID})
		case personRow:
			if r.item.UserID != "" {
				return emit(openPersonMsg{id: r.item.UserID})
			}
		}
		return nil
	}
	if s.f.list.handleKey(k, 10) {
		var cmd tea.Cmd
		if s.f.wantMore() {
			cmd = s.load(false)
		}
		return tea.Batch(cmd, s.f.afterMove(s.e, s.f.selectedSeries()))
	}
	return nil
}

func rankRows(items []model.RankItem, e *env) []row {
	rows := make([]row, 0, len(items))
	for _, it := range items {
		if it.ActorID != "" || it.UserID != "" {
			rows = append(rows, personRow{item: it, tr: e.tr, lang: e.lang()})
			continue
		}
		sum := model.SeriesSummary{ID: it.SeriesID, Title: it.Title, CoverURL: it.CoverURL, EpisodeCount: it.EpisodeCount}
		rows = append(rows, seriesRow{
			item: sum, prefix: padLeft(itoa(it.Position), 3), note: it.Score, change: it.Change, lang: e.lang(),
			promoted: it.Promoted, tr: e.tr,
		})
	}
	return rows
}

// applyCatalog takes the content and board catalog from an answer and
// keeps the tabs on what the server says this page is.
func (s *rankSection) applyCatalog(r *model.ShortRanking) {
	if len(r.Contents) == 0 {
		return
	}
	s.contents = r.Contents
	names := make([]string, len(r.Contents))
	ci := 0
	for i, c := range r.Contents {
		names[i] = c.Name
		if c.ID == r.Content || (r.Content == "" && c.Selected) {
			ci = i
		}
	}
	s.content.set(names, ci)
	bi := 0
	for i, b := range r.Contents[ci].Boards {
		if b.ID == r.Board || (r.Board == "" && b.Selected) {
			bi = i
		}
	}
	s.board.set(s.boardNames(), bi)
}

func (s *rankSection) boardNames() []string {
	if s.content.active >= len(s.contents) {
		return nil
	}
	boards := s.contents[s.content.active].Boards
	names := make([]string, len(boards))
	for i, b := range boards {
		names[i] = b.Name
	}
	return names
}

func (s *rankSection) View(w, h int) string {
	th := s.e.theme
	var lines []string
	if len(s.content.names) > 0 {
		lines = append(lines, th.Key.Render("c ")+s.content.view(w-2, th))
	}
	if len(s.board.names) > 0 {
		lines = append(lines, th.Key.Render("b ")+s.board.view(w-2, th))
	}
	lines = append(lines, s.f.list.view(w, max(h-len(lines)-1, 1), th, true), pad(s.f.statusLine(s.e.tr, th), w))
	return strings.Join(lines, "\n")
}

func (s *rankSection) Detail(w, h int) string {
	switch r := s.f.list.selected().(type) {
	case seriesRow:
		return renderSeriesDetail(s.e, w, h, r.item)
	case personRow:
		return renderPersonRow(s.e, w, h, r.item)
	}
	return ""
}

// renderPersonRow shows what a celebrity board row carries.
func renderPersonRow(e *env, w, h int, it model.RankItem) string {
	th, tr := e.theme, e.tr
	lines := []string{th.Title.Render(clip(it.Actor, w))}
	if it.Score != "" {
		lines = append(lines, clip(th.Heat.Render(it.Score), w))
	}
	if mark := changeMark(it.Change, e.lang()); mark != "" {
		lines = append(lines, clip(th.Accent.Render(mark), w))
	}
	if it.Intro != "" {
		lines = append(lines, "")
		lines = append(lines, wrap(it.Intro, w)...)
	}
	for _, d := range it.Details {
		lines = append(lines, clip(th.Subtle.Render(d), w))
	}
	if it.UserID != "" {
		lines = append(lines, "", clip(th.Key.Render("enter")+" "+tr.KeyPerson, w))
	}
	return strings.Join(lines[:min(len(lines), h)], "\n")
}
