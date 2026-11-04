package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/beilunyang/openhongguo/sdk/model"
)

// booksSection is the book store's rankings, one tab per list. Books are
// browse-only; c opens a category by ID.
type booksSection struct {
	e         *env
	groups    []model.RankList
	tabs      tabs
	groupsReq int
	groupsErr error
	offset    int
	f         feed
}

type rankGroupsMsg struct {
	req    int
	groups []model.RankList
	err    error
}
type rankRowsMsg struct {
	req     int
	ranking *model.Ranking
	err     error
}

const promptCategory = "books.category"

// bookRow is one book from a ranking or a category page.
type bookRow struct {
	item model.SeriesSummary
	pos  int
	note string
	lang string
}

func (r bookRow) key() string { return "book:" + r.item.ID }

func (r bookRow) render(w int, sel bool, th *Theme) string {
	left := rowTitle(r.item.Title, sel, th)
	indent := ""
	if r.pos > 0 {
		pos := padLeft(itoa(r.pos), 3)
		left = rankMark(pos, th) + " " + left
		indent = "    "
	}
	words := ""
	if r.item.Words > 0 {
		words = formatCount(r.item.Words, r.lang)
	}
	meta := th.Subtle.Render(joinNonEmpty(" · ", r.item.Author, r.item.Category, words))
	return twoColumns(left, th.Heat.Render(r.note), w) + secondLine(indent, meta, r.item.Description, w, th)
}

func newBooksSection(e *env) *booksSection { return &booksSection{e: e} }

func (s *booksSection) Init() tea.Cmd {
	s.groupsReq = s.e.seq.next()
	req, ctx, svc := s.groupsReq, s.e.ctx, s.e.deps.Service
	return tea.Batch(func() tea.Msg {
		groups, err := svc.RankGroups(ctx)
		return rankGroupsMsg{req: req, groups: groups, err: err}
	}, s.load(true))
}

func (s *booksSection) Title() string { return s.e.tr.SecBooks }
func (s *booksSection) Keys() []binding {
	tr := s.e.tr
	return []binding{{"[ ] h l", tr.KeyTabs}, {"enter", tr.KeyOpen}, {"c", tr.Category}, {"R", tr.KeyRefresh}}
}

func (s *booksSection) cell() string {
	if s.tabs.active < len(s.groups) {
		return s.groups[s.tabs.active].ID
	}
	return ""
}

func (s *booksSection) load(fresh bool) tea.Cmd {
	if fresh {
		s.f.reset()
		s.offset = 0
	}
	req := s.f.begin(s.e.seq)
	cell, offset, ctx, svc := s.cell(), s.offset, s.e.ctx, s.e.deps.Service
	return func() tea.Msg {
		r, err := svc.RankRows(ctx, cell, offset)
		return rankRowsMsg{req: req, ranking: r, err: err}
	}
}

func (s *booksSection) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case rankGroupsMsg:
		if msg.req != s.groupsReq {
			return nil
		}
		s.groups, s.groupsErr = msg.groups, msg.err
		names := make([]string, len(s.groups))
		for i, g := range s.groups {
			names[i] = g.Name
		}
		s.tabs.set(names, 0)
	case rankRowsMsg:
		if !s.f.accept(msg.req) {
			return nil
		}
		if msg.err != nil {
			s.f.err = msg.err
			s.f.hasMore = false
			return nil
		}
		rows := make([]row, 0, len(msg.ranking.Items))
		for _, it := range msg.ranking.Items {
			sum := model.SeriesSummary{ID: it.SeriesID, Title: it.Title, CoverURL: it.CoverURL, Author: it.Actor}
			rows = append(rows, bookRow{item: sum, pos: s.f.list.len() + len(rows) + 1, note: it.Score, lang: s.e.lang()})
		}
		s.f.list.add(rows)
		s.offset = msg.ranking.NextOffset
		s.f.hasMore = msg.ranking.HasMore && s.offset > 0
	case promptDoneMsg:
		if msg.id == promptCategory && msg.ok && strings.TrimSpace(msg.value) != "" {
			return emit(pushScreenMsg{screen: newCategoryPage(s.e, strings.TrimSpace(msg.value))})
		}
	case tea.KeyPressMsg:
		k := keyString(msg)
		if s.tabs.handleKey(k) {
			return s.load(true)
		}
		switch k {
		case "enter":
			if r, ok := s.f.list.selected().(bookRow); ok {
				return emit(openSeriesMsg{id: r.item.ID})
			}
			return nil
		case "c":
			return emit(promptMsg{id: promptCategory, label: s.e.tr.PromptCategory})
		case "R":
			return s.load(true)
		}
		if s.f.list.handleKey(k, 10) && s.f.wantMore() {
			return s.load(false)
		}
	}
	return nil
}

func (s *booksSection) View(w, h int) string {
	th, tr := s.e.theme, s.e.tr
	var lines []string
	switch {
	case s.groupsErr != nil:
		lines = append(lines, th.ToastErr.Render(clip(describeError(s.groupsErr, tr), w)))
	case len(s.tabs.names) > 0:
		lines = append(lines, s.tabs.view(w, th))
	}
	lines = append(lines, s.f.list.view(w, max(h-len(lines)-1, 1), th, true), pad(s.f.statusLine(tr, th), w))
	return strings.Join(lines, "\n")
}

func (s *booksSection) Detail(w, h int) string {
	r, ok := s.f.list.selected().(bookRow)
	if !ok {
		return ""
	}
	return renderBookDetail(s.e, w, h, r)
}

func renderBookDetail(e *env, w, h int, r bookRow) string {
	th, tr, lang := e.theme, e.tr, e.lang()
	it := r.item
	lines := []string{th.Title.Render(clip(it.Title, w))}
	for _, kv := range [][2]string{{tr.Author, it.Author}, {tr.Category, it.Category}} {
		if kv[1] != "" {
			lines = append(lines, clip(labelled(kv[0], kv[1], th), w))
		}
	}
	if r.note != "" {
		lines = append(lines, clip(th.Heat.Render(r.note), w))
	}
	if it.Words > 0 {
		lines = append(lines, clip(labelled(tr.Words, formatCount(it.Words, lang), th), w))
	}
	if it.Description != "" {
		lines = append(lines, "")
		lines = append(lines, wrap(it.Description, w)...)
	}
	lines = append(lines, "", th.Subtle.Render(clip(tr.ToastBook, w)))
	return strings.Join(lines[:min(len(lines), h)], "\n")
}
