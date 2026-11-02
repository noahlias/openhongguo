package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/beilunyang/openhongguo/internal/playback"
	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// seriesPage shows one drama: its record, its episodes as a grid, its
// seasons, and related dramas.
type seriesPage struct {
	e        *env
	id       string
	req      int
	pending  int
	series   *model.Series
	episodes []model.Episode
	album    *model.Album
	related  *model.Recommend
	err      error

	seasons tabs
	grid    episodeGrid
	relList listPane
	// focus is 0 for the grid and 1 for the related list.
	focus int
}

type seriesLoadedMsg struct {
	req      int
	id       string
	series   *model.Series
	episodes []model.Episode
	album    *model.Album
	related  *model.Recommend
	err      error
}

func newSeriesPage(e *env, id string) *seriesPage {
	return &seriesPage{e: e, id: id}
}

func (p *seriesPage) Init() tea.Cmd { return p.load() }

func (p *seriesPage) Title() string {
	if p.series != nil {
		return p.series.Title
	}
	return p.e.tr.Loading
}

func (p *seriesPage) Keys() []binding {
	tr := p.e.tr
	return []binding{
		{"enter", tr.KeyPlay}, {"a", tr.KeyAutoplay}, {"r", tr.KeyResume}, {"d", tr.KeyDownload},
		{"[ ]", tr.Seasons}, {"tab", tr.Related}, {"h j k l", tr.KeyMove}, {"R", tr.KeyRefresh},
	}
}

// load fetches everything the page shows; the pieces that fail are left
// empty, except the series itself.
func (p *seriesPage) load() tea.Cmd {
	p.req = p.e.seq.next()
	req, id := p.req, p.id
	ctx, svc := p.e.ctx, p.e.deps.Service
	p.err = nil
	return func() tea.Msg {
		out := seriesLoadedMsg{req: req, id: id}
		type result struct {
			series   *model.Series
			episodes *sdk.Page[model.Episode]
			album    *model.Album
			related  *model.Recommend
			err      error
		}
		ch := make(chan result, 4)
		go func() {
			s, err := svc.GetSeries(ctx, id)
			ch <- result{series: s, err: err}
		}()
		go func() {
			eps, err := svc.ListEpisodes(ctx, id, sdk.EpisodeOptions{})
			ch <- result{episodes: eps, err: err}
		}()
		go func() {
			a, _ := svc.Seasons(ctx, id)
			ch <- result{album: a}
		}()
		go func() {
			r, _ := svc.SeriesRecommend(ctx, id)
			ch <- result{related: r}
		}()
		for i := 0; i < 4; i++ {
			r := <-ch
			switch {
			case r.series != nil:
				out.series = r.series
			case r.episodes != nil:
				out.episodes = r.episodes.Items
			case r.album != nil:
				out.album = r.album
			case r.related != nil:
				out.related = r.related
			case r.err != nil && out.err == nil:
				out.err = r.err
			}
		}
		return out
	}
}

func (p *seriesPage) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case seriesLoadedMsg:
		if msg.req != p.req {
			return nil
		}
		p.series, p.episodes, p.album, p.related, p.err = msg.series, msg.episodes, msg.album, msg.related, msg.err
		if p.series != nil {
			p.e.setDetail(p.id, p.series)
		}
		p.buildSeasons()
		p.buildRelated()
		p.grid.setCount(len(p.episodes))
		if h, ok := p.e.watched(p.id); ok {
			p.grid.cursor = indexOfEpisode(p.episodes, h.Episode)
		}
		return nil
	case promptDoneMsg:
		if msg.id == "download:"+p.id && msg.ok {
			return p.download(msg.value)
		}
	case tea.KeyPressMsg:
		return p.key(keyString(msg))
	}
	return nil
}

func (p *seriesPage) key(k string) tea.Cmd {
	switch k {
	case "tab":
		if p.relList.len() > 0 {
			p.focus = 1 - p.focus
		}
		return nil
	case "[", "]":
		by := 1
		if k == "[" {
			by = -1
		}
		if p.seasons.move(by) {
			return p.switchSeason()
		}
		return nil
	case "R":
		return p.load()
	case "r":
		if p.series == nil {
			return nil
		}
		if p.series.Platform == model.PlatformUnknown {
			return toastErr(p.e.tr.ToastBook)
		}
		return emit(resumeFromHistory(p.e, p.id, p.series.Title, p.episodes))
	case "d":
		if p.series == nil || len(p.episodes) == 0 {
			return nil
		}
		if p.series.Platform == model.PlatformUnknown {
			return toastErr(p.e.tr.ToastBook)
		}
		return emit(promptMsg{id: "download:" + p.id, label: p.e.tr.PromptEpisodes, initial: fmt.Sprintf("1-%d", len(p.episodes))})
	}
	if p.focus == 1 {
		if p.relList.handleKey(k, 5) {
			return nil
		}
		if k == "enter" {
			if r, ok := p.relList.selected().(seriesRow); ok {
				return emit(openSeriesMsg{id: r.item.ID})
			}
		}
		return nil
	}
	if p.grid.handleKey(k) {
		return nil
	}
	switch k {
	case "enter", "a":
		return p.play(k == "a")
	}
	return nil
}

func (p *seriesPage) play(forceAutoplay bool) tea.Cmd {
	if p.series == nil || len(p.episodes) == 0 {
		return nil
	}
	if p.series.Platform == model.PlatformUnknown {
		return toastErr(p.e.tr.ToastBook)
	}
	ep := p.episodes[p.grid.cursor]
	if !ep.Playable {
		return toastErr(p.e.tr.ToastLocked)
	}
	return emit(playMsg{
		seriesID: p.id, title: p.series.Title, episodes: p.episodes, start: ep,
		autoplay: forceAutoplay || p.e.settings.Autoplay,
	})
}

func (p *seriesPage) download(spec string) tea.Cmd {
	numbers, err := playback.ParseEpisodeRange(spec, len(p.episodes))
	if err != nil {
		if errors.Is(err, playback.ErrInvalid) {
			return toastErr(strings.TrimPrefix(err.Error(), playback.ErrInvalid.Error()+": "))
		}
		return toastErr(err.Error())
	}
	return emit(downloadMsg{seriesID: p.id, title: p.series.Title, episodes: p.episodes, numbers: numbers})
}

func (p *seriesPage) buildSeasons() {
	if p.album == nil || len(p.album.Seasons) < 2 {
		p.seasons.set(nil, 0)
		return
	}
	names := make([]string, len(p.album.Seasons))
	active := 0
	for i, s := range p.album.Seasons {
		names[i] = s.Title
		if names[i] == "" {
			names[i] = fmt.Sprintf("S%d", s.Index)
		}
		if s.SeriesID == p.id {
			active = i
		}
	}
	p.seasons.set(names, active)
}

func (p *seriesPage) switchSeason() tea.Cmd {
	if p.album == nil || p.seasons.active >= len(p.album.Seasons) {
		return nil
	}
	season := p.album.Seasons[p.seasons.active]
	if season.SeriesID == "" || season.SeriesID == p.id {
		return nil
	}
	p.id = season.SeriesID
	p.series, p.episodes, p.related = nil, nil, nil
	p.grid.setCount(0)
	return p.load()
}

func (p *seriesPage) buildRelated() {
	rows := []row{}
	if p.related != nil {
		for _, it := range p.related.Items {
			if it.ID == p.id {
				continue
			}
			rows = append(rows, seriesRow{item: it, tr: p.e.tr})
		}
	}
	p.relList.set(rows)
	if len(rows) == 0 {
		p.focus = 0
	}
}

func (p *seriesPage) View(w, h int) string {
	tr, th, lang := p.e.tr, p.e.theme, p.e.lang()
	if p.series == nil {
		if p.err != nil {
			return th.ToastErr.Render(clip(describeError(p.err, tr), w))
		}
		return th.Subtle.Render(tr.Loading)
	}
	s := p.series
	var lines []string
	lines = append(lines, th.Title.Render(clip(s.Title, w)))
	meta := joinNonEmpty(" · ",
		contentLabel(s.ContentType, lang), statusLabel(s.Status, lang),
		fmt.Sprintf(tr.Episodes, firstNonZero(len(p.episodes), s.EpisodeCount)), s.RightText,
	)
	if s.PayType != 0 {
		meta = joinNonEmpty(" · ", meta, payLabel(s.PayType, lang))
	}
	lines = append(lines, clip(th.Subtle.Render(meta), w))
	counts := joinNonEmpty("  ",
		labelled(tr.Heat, th.Heat.Render(formatCount(s.Heat, lang)), th),
		labelled(tr.Plays, formatCount(s.PlayCount, lang), th),
		labelled(tr.Follows, formatCount(s.FollowCount, lang), th),
		labelled(tr.Likes, formatCount(s.DiggCount, lang), th),
	)
	if counts != "" {
		lines = append(lines, clip(counts, w))
	}
	if hist, ok := p.e.watched(p.id); ok {
		where := fmt.Sprintf(tr.EpisodeN, hist.Episode)
		if hist.Finished {
			where += " ✓"
		} else if hist.Position > 0 {
			where += " " + formatSeconds(hist.Position)
		}
		lines = append(lines, clip(labelled(tr.LastWatched, th.Accent.Render(where), th), w))
	}
	if len(p.seasons.names) > 0 {
		lines = append(lines, p.seasons.view(w, th))
	}
	lines = append(lines, "")
	used := len(lines) + 1
	gridH := h - used
	if gridH < 1 {
		gridH = 1
	}
	lines = append(lines, p.grid.view(p.episodes, p.e, p.id, w, gridH, p.focus == 0))
	lines = append(lines, p.legend(w))
	return strings.Join(lines, "\n")
}

func (p *seriesPage) legend(w int) string {
	tr, th := p.e.tr, p.e.theme
	parts := []string{
		th.Current.Render(" > ") + " " + tr.LegendCurrent,
		th.Watched.Render(" . ") + " " + tr.LegendWatched,
		th.New.Render(" * ") + " " + tr.LegendNew,
		th.Locked.Render(" x ") + " " + tr.LegendLocked,
	}
	return pad(strings.Join(parts, "  "), w)
}

func (p *seriesPage) Detail(w, h int) string {
	if p.series == nil {
		return ""
	}
	tr, th := p.e.tr, p.e.theme
	top := renderSeriesDetail(p.e, w, h, p.series.SeriesSummary)
	if p.relList.len() == 0 {
		return top
	}
	topLines := strings.Split(top, "\n")
	relH := min(p.relList.len()+1, h/2)
	if relH < 3 {
		relH = min(3, h-1)
	}
	keep := h - relH - 1
	if keep < 0 {
		keep = 0
	}
	if len(topLines) > keep {
		topLines = topLines[:keep]
	}
	for len(topLines) < keep {
		topLines = append(topLines, "")
	}
	head := th.Accent.Render(tr.Related)
	if p.focus == 1 {
		head = th.NavFocused.Render(" " + tr.Related + " ")
	}
	return strings.Join(topLines, "\n") + "\n" + head + "\n" + p.relList.view(w, relH-1, th, p.focus == 1)
}

func indexOfEpisode(items []model.Episode, number int) int {
	for i, e := range items {
		if e.Number == number {
			return i
		}
	}
	return 0
}
