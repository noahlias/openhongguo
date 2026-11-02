package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// searchPage types a query with live suggestions and lists the results.
type searchPage struct {
	e          *env
	input      textinput.Model
	typing     bool
	tabs       tabs
	filters    [5]bool
	query      string
	f          feed
	sugg       []model.Suggestion
	suggCursor int
	suggReq    int
	cueReq     int
}

type searchPageMsg struct {
	req  int
	page *sdk.Page[model.SeriesSummary]
	err  error
}
type suggestMsg struct {
	req   int
	items []model.Suggestion
}
type suggestTickMsg struct {
	req    int
	prefix string
}
type cuesMsg struct {
	req  int
	cues []string
}

const suggestDelay = 250 * time.Millisecond

var searchFilterValues = [5]string{
	sdk.SearchFilterShortPlay, sdk.SearchFilterComic, sdk.SearchFilterAIComic, sdk.SearchFilterFilm, sdk.SearchFilterUnwatched,
}

func newSearchPage(e *env, query string) *searchPage {
	ti := textinput.New()
	ti.Prompt = "/ "
	ti.Placeholder = e.tr.SearchPlaceholder
	ti.SetValue(query)
	p := &searchPage{e: e, input: ti, typing: true, suggCursor: -1}
	p.tabs.set(searchTabs, 0)
	return p
}

func (p *searchPage) Init() tea.Cmd {
	cmds := []tea.Cmd{p.input.Focus(), p.loadCues()}
	if strings.TrimSpace(p.input.Value()) != "" {
		cmds = append(cmds, p.search())
	}
	return tea.Batch(cmds...)
}

func (p *searchPage) Title() string { return p.e.tr.KeySearch }

// capturing tells the app to hand every key to the text input.
func (p *searchPage) capturing() bool { return p.typing }

func (p *searchPage) Keys() []binding {
	tr := p.e.tr
	return []binding{
		{"enter", tr.KeySearch}, {"↑ ↓", tr.KeyMove}, {"tab", tr.KeyFocus}, {"[ ]", tr.KeyTabs},
		{"1-5", tr.KeyToggle}, {"i /", tr.KeySearch}, {"enter", tr.KeyOpen},
	}
}

// focusInput puts the cursor back in the query box.
func (p *searchPage) focusInput() tea.Cmd {
	p.typing = true
	return p.input.Focus()
}

func (p *searchPage) loadCues() tea.Cmd {
	p.cueReq = p.e.seq.next()
	req, ctx, svc := p.cueReq, p.e.ctx, p.e.deps.Service
	return func() tea.Msg {
		cues, err := svc.SearchCues(ctx)
		if err != nil {
			// The placeholder is decoration; the generic one stays.
			return nil
		}
		return cuesMsg{req: req, cues: cues}
	}
}

func (p *searchPage) search() tea.Cmd {
	q := strings.TrimSpace(p.input.Value())
	if q == "" {
		q = p.input.Placeholder
		if q == p.e.tr.SearchPlaceholder {
			return nil
		}
		p.input.SetValue(q)
	}
	p.query = q
	p.f.reset()
	p.sugg, p.suggCursor = nil, -1
	p.suggReq = p.e.seq.next()
	p.typing = false
	p.input.Blur()
	return p.loadPage()
}

func (p *searchPage) loadPage() tea.Cmd {
	req := p.f.begin(p.e.seq)
	ctx, svc := p.e.ctx, p.e.deps.Service
	opts := sdk.SearchOptions{Cursor: p.f.cursor, Tab: p.tabs.name(), PageSize: 20}
	if p.tabs.active == 0 {
		for i, on := range p.filters {
			if on {
				opts.Filters = append(opts.Filters, searchFilterValues[i])
			}
		}
	}
	q := p.query
	return func() tea.Msg {
		page, err := svc.Search(ctx, q, opts)
		return searchPageMsg{req: req, page: page, err: err}
	}
}

func (p *searchPage) scheduleSuggest() tea.Cmd {
	prefix := strings.TrimSpace(p.input.Value())
	p.suggReq = p.e.seq.next()
	req := p.suggReq
	if prefix == "" {
		p.sugg, p.suggCursor = nil, -1
		return nil
	}
	return tea.Tick(suggestDelay, func(time.Time) tea.Msg { return suggestTickMsg{req: req, prefix: prefix} })
}

func (p *searchPage) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case cuesMsg:
		if msg.req == p.cueReq && len(msg.cues) > 0 {
			p.input.Placeholder = msg.cues[0]
		}
	case suggestTickMsg:
		if msg.req != p.suggReq {
			return nil
		}
		ctx, svc := p.e.ctx, p.e.deps.Service
		return func() tea.Msg {
			items, err := svc.Suggest(ctx, msg.prefix)
			if err != nil {
				// Suggestions are a convenience; typing still works without them.
				return nil
			}
			return suggestMsg{req: msg.req, items: items}
		}
	case suggestMsg:
		if msg.req == p.suggReq && p.typing {
			p.sugg = msg.items[:min(len(msg.items), 8)]
			p.suggCursor = -1
		}
	case searchPageMsg:
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
			rows = append(rows, seriesRow{item: it, tr: p.e.tr})
		}
		p.f.list.add(rows)
		p.f.page(msg.page.Cursor, msg.page.HasMore)
		return p.f.afterMove(p.e, p.f.selectedSeries())
	case detailTickMsg:
		return p.f.onDetailTick(p.e, msg.id)
	case tea.KeyPressMsg:
		if p.typing {
			return p.typingKey(msg)
		}
		return p.resultsKey(keyString(msg))
	default:
		if p.typing {
			var cmd tea.Cmd
			p.input, cmd = p.input.Update(msg)
			return cmd
		}
	}
	return nil
}

func (p *searchPage) leaveInput() {
	p.typing = false
	p.sugg, p.suggCursor = nil, -1
	p.input.Blur()
}

func (p *searchPage) typingKey(msg tea.KeyPressMsg) tea.Cmd {
	switch keyString(msg) {
	case "esc":
		if p.f.list.len() > 0 {
			p.leaveInput()
			return nil
		}
		return emit(popScreenMsg{})
	case "enter":
		if p.suggCursor >= 0 && p.suggCursor < len(p.sugg) {
			s := p.sugg[p.suggCursor]
			if directHit(s) {
				p.leaveInput()
				return emit(openSeriesMsg{id: s.Keyword})
			}
			text := s.Keyword
			if text == "" {
				text = s.Text
			}
			p.input.SetValue(text)
		}
		return p.search()
	case "down", "ctrl+n":
		if len(p.sugg) > 0 {
			p.suggCursor = min(p.suggCursor+1, len(p.sugg)-1)
		} else if p.f.list.len() > 0 {
			p.leaveInput()
		}
		return nil
	case "up", "ctrl+p":
		if p.suggCursor >= 0 {
			p.suggCursor--
		}
		return nil
	case "tab":
		if p.f.list.len() > 0 {
			p.leaveInput()
		}
		return nil
	}
	before := p.input.Value()
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	if p.input.Value() != before {
		return tea.Batch(cmd, p.scheduleSuggest())
	}
	return cmd
}

func (p *searchPage) resultsKey(k string) tea.Cmd {
	switch k {
	case "i", "/", "tab":
		return p.focusInput()
	case "[", "]":
		by := 1
		if k == "[" {
			by = -1
		}
		if p.tabs.move(by) && p.query != "" {
			p.f.reset()
			return p.loadPage()
		}
		return nil
	case "1", "2", "3", "4", "5":
		if p.tabs.active != 0 {
			return nil
		}
		i := int(k[0] - '1')
		p.filters[i] = !p.filters[i]
		if p.query != "" {
			p.f.reset()
			return p.loadPage()
		}
		return nil
	case "enter":
		if id := p.f.selectedSeries(); id != "" {
			return emit(openSeriesMsg{id: id})
		}
		return nil
	}
	if p.f.list.handleKey(k, 10) {
		var cmd tea.Cmd
		if p.f.wantMore() {
			cmd = p.loadPage()
		}
		return tea.Batch(cmd, p.f.afterMove(p.e, p.f.selectedSeries()))
	}
	return nil
}

func (p *searchPage) View(w, h int) string {
	tr, th := p.e.tr, p.e.theme
	p.input.SetWidth(max(w-4, 1))
	lines := []string{pad(clip(p.input.View(), w), w), p.tabs.view(w, th)}
	if p.tabs.active == 0 {
		var cells []string
		for i, name := range searchFilters {
			cell := fmt.Sprintf(" %d %s ", i+1, name)
			if p.filters[i] {
				cells = append(cells, th.ChipOn.Render(cell))
			} else {
				cells = append(cells, th.ChipOff.Render(cell))
			}
		}
		lines = append(lines, pad(clip(strings.Join(cells, " "), w), w))
	}
	if p.typing {
		for i, s := range p.sugg {
			text := "  " + s.Text
			if directHit(s) {
				text = th.Accent.Render("→ ") + s.Text
			}
			if i == p.suggCursor {
				text = th.RowSelected.Render(pad(clip(text, w), w))
			}
			lines = append(lines, pad(clip(text, w), w))
		}
	}
	rest := max(h-len(lines)-1, 1)
	status := p.f.statusLine(tr, th)
	switch {
	case p.query == "":
		status = th.Subtle.Render(tr.SearchPlaceholder)
	case !p.f.loading && p.f.err == nil && p.f.list.len() > 0:
		status = th.Subtle.Render(fmt.Sprintf("%s · %d", tr.Results, p.f.list.len()))
	}
	lines = append(lines, p.f.list.view(w, rest, th, !p.typing), pad(status, w))
	return strings.Join(lines, "\n")
}

func (p *searchPage) Detail(w, h int) string {
	r, ok := p.f.list.selected().(seriesRow)
	if !ok {
		return ""
	}
	return renderSeriesDetail(p.e, w, h, r.item)
}

// directHit reports a suggestion that names one series: its keyword is the
// series ID rather than words to search for.
func directHit(s model.Suggestion) bool {
	if s.Keyword == "" {
		return false
	}
	for _, r := range s.Keyword {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
