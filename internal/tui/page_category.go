package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// categoryPage is one book category: audience tabs, filter chips, and the
// books that match.
type categoryPage struct {
	e      *env
	id     string
	name   string
	tabs   tabs
	tabVal []string
	panel  chips
	ready  bool
	offset int
	f      feed
	// focus is 0 for the chips and 1 for the results.
	focus int
}

type landingMsg struct {
	req  int
	page *model.CategoryPage
	err  error
}

func newCategoryPage(e *env, id string) *categoryPage {
	return &categoryPage{e: e, id: id, focus: 1}
}

func (p *categoryPage) Init() tea.Cmd { return p.load(true) }

func (p *categoryPage) Title() string {
	if p.name != "" {
		return p.name
	}
	return p.e.tr.Category + " " + p.id
}

func (p *categoryPage) Keys() []binding {
	tr := p.e.tr
	return []binding{
		{"[ ]", tr.KeyTabs}, {"tab", tr.KeyFocus}, {"h j k l", tr.KeyMove}, {"space", tr.KeyToggle},
		{"x", tr.KeyClear}, {"enter", tr.KeyOpen},
	}
}

func (p *categoryPage) load(fresh bool) tea.Cmd {
	if fresh {
		p.f.reset()
		p.offset = 0
	}
	req := p.f.begin(p.e.seq)
	opts := sdk.LandingOptions{Filters: p.panel.selections()}
	if p.tabs.active < len(p.tabVal) {
		opts.Tab = p.tabVal[p.tabs.active]
	}
	if p.offset > 0 {
		opts.Cursor = itoa(p.offset)
	}
	id, ctx, svc := p.id, p.e.ctx, p.e.deps.Service
	return func() tea.Msg {
		page, err := svc.CategoryLanding(ctx, id, opts)
		return landingMsg{req: req, page: page, err: err}
	}
}

func (p *categoryPage) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case landingMsg:
		if !p.f.accept(msg.req) {
			return nil
		}
		if msg.err != nil {
			p.f.err = msg.err
			p.f.hasMore = false
			return nil
		}
		p.applyPanel(msg.page)
		rows := make([]row, 0, len(msg.page.Items))
		for _, it := range msg.page.Items {
			rows = append(rows, bookRow{item: it, lang: p.e.lang()})
		}
		p.f.list.add(rows)
		p.offset = msg.page.NextOffset
		p.f.hasMore = msg.page.HasMore && p.offset > 0
	case tea.KeyPressMsg:
		return p.key(keyString(msg))
	}
	return nil
}

// applyPanel takes the tabs and filter groups from the first page only, so
// the viewer's choices are not reset by later pages.
func (p *categoryPage) applyPanel(page *model.CategoryPage) {
	if p.ready {
		return
	}
	p.ready = true
	p.name = page.CategoryName
	names := make([]string, len(page.Tabs))
	p.tabVal = make([]string, len(page.Tabs))
	active := 0
	for i, t := range page.Tabs {
		names[i], p.tabVal[i] = t.Name, t.Value
		if t.Value == itoa(page.DefaultTab) {
			active = i
		}
	}
	p.tabs.set(names, active)
	rows := make([]chipRow, 0, len(page.Filters))
	for _, g := range page.Filters {
		if len(g.Options) > 0 {
			rows = append(rows, chipRow{key: g.Key, name: g.Key, options: g.Options})
		}
	}
	p.panel.set(rows)
}

func (p *categoryPage) key(k string) tea.Cmd {
	switch k {
	case "[", "]":
		by := 1
		if k == "[" {
			by = -1
		}
		if p.tabs.move(by) {
			return p.load(true)
		}
		return nil
	case "tab":
		if !p.panel.empty() {
			p.focus = 1 - p.focus
		}
		return nil
	}
	if p.focus == 0 {
		if _, changed := p.panel.handleKey(k); changed {
			return p.load(true)
		}
		return nil
	}
	if k == "enter" {
		if r, ok := p.f.list.selected().(bookRow); ok {
			return emit(openSeriesMsg{id: r.item.ID})
		}
		return nil
	}
	if p.f.list.handleKey(k, 10) && p.f.wantMore() {
		return p.load(false)
	}
	return nil
}

func (p *categoryPage) View(w, h int) string {
	th := p.e.theme
	var lines []string
	if len(p.tabs.names) > 0 {
		lines = append(lines, p.tabs.view(w, th))
	}
	if !p.panel.empty() {
		lines = append(lines, p.panel.view(w, th, p.focus == 0)...)
		lines = append(lines, th.Border.Render(strings.Repeat("─", w)))
	}
	lines = append(lines, p.f.list.view(w, max(h-len(lines)-1, 1), th, p.focus == 1), pad(p.f.statusLine(p.e.tr, th), w))
	return strings.Join(lines, "\n")
}

func (p *categoryPage) Detail(w, h int) string {
	r, ok := p.f.list.selected().(bookRow)
	if !ok {
		return ""
	}
	return renderBookDetail(p.e, w, h, r)
}
