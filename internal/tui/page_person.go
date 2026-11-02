package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/beilunyang/openhongguo/sdk/model"
)

// personPage is an actor's profile and their works.
type personPage struct {
	e         *env
	id        string
	personReq int
	person    *model.Person
	personErr error
	f         feed
}

type personMsg struct {
	req    int
	person *model.Person
	err    error
}
type worksMsg struct {
	req  int
	page *model.WorkPage
	err  error
}

func newPersonPage(e *env, id string) *personPage { return &personPage{e: e, id: id} }

func (p *personPage) Init() tea.Cmd {
	p.personReq = p.e.seq.next()
	req, id, ctx, svc := p.personReq, p.id, p.e.ctx, p.e.deps.Service
	load := func() tea.Msg {
		person, err := svc.Person(ctx, id)
		return personMsg{req: req, person: person, err: err}
	}
	return tea.Batch(load, p.loadWorks())
}

func (p *personPage) loadWorks() tea.Cmd {
	req := p.f.begin(p.e.seq)
	id, cursor, ctx, svc := p.id, p.f.cursor, p.e.ctx, p.e.deps.Service
	return func() tea.Msg {
		page, err := svc.PersonWorks(ctx, id, cursor)
		return worksMsg{req: req, page: page, err: err}
	}
}

func (p *personPage) Title() string {
	if p.person != nil {
		return p.person.Name
	}
	return p.e.tr.Loading
}

func (p *personPage) Keys() []binding {
	return []binding{{"enter", p.e.tr.KeyOpen}}
}

func (p *personPage) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case personMsg:
		if msg.req == p.personReq {
			p.person, p.personErr = msg.person, msg.err
		}
	case worksMsg:
		if !p.f.accept(msg.req) {
			return nil
		}
		if msg.err != nil {
			p.f.err = msg.err
			p.f.hasMore = false
			return nil
		}
		rows := make([]row, 0, len(msg.page.Items))
		for _, it := range msg.page.Items {
			rows = append(rows, workRow{item: it, tr: p.e.tr, lang: p.e.lang()})
		}
		p.f.list.add(rows)
		p.f.page(msg.page.Cursor, msg.page.HasMore)
		return p.f.afterMove(p.e, p.f.selectedSeries())
	case detailTickMsg:
		return p.f.onDetailTick(p.e, msg.id)
	case tea.KeyPressMsg:
		k := keyString(msg)
		if k == "enter" {
			if id := p.f.selectedSeries(); id != "" {
				return emit(openSeriesMsg{id: id})
			}
			return nil
		}
		if p.f.list.handleKey(k, 10) {
			var cmd tea.Cmd
			if p.f.wantMore() {
				cmd = p.loadWorks()
			}
			return tea.Batch(cmd, p.f.afterMove(p.e, p.f.selectedSeries()))
		}
	}
	return nil
}

func (p *personPage) header(w int) []string {
	th, tr, lang := p.e.theme, p.e.tr, p.e.lang()
	if p.personErr != nil {
		return []string{th.ToastErr.Render(clip(describeError(p.personErr, tr), w))}
	}
	if p.person == nil {
		return []string{th.Subtle.Render(tr.Loading)}
	}
	pr := p.person
	lines := []string{th.Title.Render(clip(pr.Name, w))}
	if tags := joinNonEmpty(" · ", strings.Join(pr.Tags, " · "), strings.Join(pr.Labels, " · ")); tags != "" {
		lines = append(lines, clip(th.Subtle.Render(tags), w))
	}
	counts := joinNonEmpty("  ",
		labelled(tr.Followers, formatCount(pr.Followers, lang), th),
		labelled(tr.Likes, formatCount(pr.Likes, lang), th),
	)
	if counts != "" {
		lines = append(lines, clip(counts, w))
	}
	if pr.Description != "" {
		desc := wrap(pr.Description, w)
		lines = append(lines, desc[:min(len(desc), 3)]...)
	}
	lines = append(lines, th.Accent.Render(clip(fmt.Sprintf("%s · %d", tr.Works, p.f.list.len()), w)))
	return lines
}

func (p *personPage) View(w, h int) string {
	th := p.e.theme
	lines := p.header(w)
	lines = append(lines, p.f.list.view(w, max(h-len(lines)-1, 1), th, true), pad(p.f.statusLine(p.e.tr, th), w))
	return strings.Join(lines, "\n")
}

func (p *personPage) Detail(w, h int) string {
	r, ok := p.f.list.selected().(workRow)
	if !ok {
		return ""
	}
	it := r.item
	sum := model.SeriesSummary{ID: it.SeriesID, Title: it.Title, Description: it.Description, CoverURL: it.CoverURL, CoverExpiresAt: it.CoverExpiresAt, EpisodeCount: it.EpisodeCount}
	return renderSeriesDetail(p.e, w, h, sum)
}
