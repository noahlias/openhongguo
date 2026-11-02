package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// discoverSection shows a channel's front page and pages through it.
type discoverSection struct {
	e        *env
	channels tabs
	f        feed
}

type discoverPageMsg struct {
	req  int
	page *sdk.Page[model.SeriesSummary]
	err  error
}

func newDiscoverSection(e *env) *discoverSection {
	s := &discoverSection{e: e}
	s.channels.set(discoverChannels, 0)
	return s
}

func (s *discoverSection) Init() tea.Cmd { return s.load(true) }

func (s *discoverSection) Title() string { return s.e.tr.SecDiscover }

func (s *discoverSection) Keys() []binding {
	tr := s.e.tr
	return []binding{{"[ ]", tr.KeyTabs}, {"enter", tr.KeyOpen}, {"R", tr.KeyRefresh}}
}

func (s *discoverSection) load(fresh bool) tea.Cmd {
	if fresh {
		s.f.reset()
	}
	req := s.f.begin(s.e.seq)
	ctx, svc := s.e.ctx, s.e.deps.Service
	opts := sdk.DiscoverOptions{Channel: s.channels.name(), Cursor: s.f.cursor}
	return func() tea.Msg {
		page, err := svc.Discover(ctx, opts)
		return discoverPageMsg{req: req, page: page, err: err}
	}
}

func (s *discoverSection) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case discoverPageMsg:
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
		added := s.f.list.add(rows)
		s.f.page(msg.page.Cursor, msg.page.HasMore)
		if added == 0 && s.f.hasMore && s.f.list.len() < 60 {
			// A page of only repeats: keep going, like the SDK pager.
			return s.load(false)
		}
		return s.f.afterMove(s.e, s.f.selectedSeries())
	case detailTickMsg:
		return s.f.onDetailTick(s.e, msg.id)
	case tea.KeyPressMsg:
		k := keyString(msg)
		if s.channels.handleKey(k) {
			return s.load(true)
		}
		if s.f.list.handleKey(k, 10) {
			var cmd tea.Cmd
			if s.f.wantMore() {
				cmd = s.load(false)
			}
			return tea.Batch(cmd, s.f.afterMove(s.e, s.f.selectedSeries()))
		}
		switch k {
		case "enter":
			if id := s.f.selectedSeries(); id != "" {
				return emit(openSeriesMsg{id: id})
			}
		case "R":
			return s.load(true)
		}
	}
	return nil
}

func (s *discoverSection) View(w, h int) string {
	th := s.e.theme
	head := s.channels.view(w, th)
	body := s.f.list.view(w, h-2, th, true)
	return head + "\n" + body + "\n" + pad(s.f.statusLine(s.e.tr, th), w)
}

func (s *discoverSection) Detail(w, h int) string {
	r, ok := s.f.list.selected().(seriesRow)
	if !ok {
		return ""
	}
	return renderSeriesDetail(s.e, w, h, r.item)
}
