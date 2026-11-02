package tui

import (
	"context"
	"fmt"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/beilunyang/openhongguo/internal/history"
	"github.com/beilunyang/openhongguo/internal/playback"
	"github.com/beilunyang/openhongguo/sdk/model"
)

type focus int

const (
	focusNav focus = iota
	focusMain
)

const (
	navContinue = iota
	navDiscover
	navHot
	navRank
	navFilter
	navNew
	navHistory
	navDownloads
	navBooks
	navSettings
	navCount
)

// app is the root model: the navigation, the live sections, the pushed
// pages, the overlay, and the two background controllers.
type app struct {
	env          *env
	settingsPath string
	loadErr      error
	width        int
	height       int
	dark         bool
	nav          int
	focus        focus
	sections     [navCount]Screen
	stack        []Screen
	overlay      overlay
	toasts       toasts
	player       *playerCtl
	downloads    *downloadCtl
}

func newApp(ctx context.Context, d Deps, s Settings, settingsPath string, loadErr error) *app {
	theme := newTheme(s.Theme, true)
	e := &env{
		ctx: ctx, deps: d, settings: &s, tr: stringsFor(s.Language), theme: &theme, seq: &seq{},
		details: map[string]*model.Series{}, hist: map[string]history.Entry{},
	}
	if d.HTTP != nil {
		e.covers = newCovers(d.HTTP)
	}
	a := &app{env: e, settingsPath: settingsPath, loadErr: loadErr, dark: true, focus: focusMain}
	a.player = &playerCtl{}
	a.downloads = &downloadCtl{ctx: ctx, deps: a.engine(), jobs: func() int { return e.settings.Jobs }}
	return a
}

func (a *app) engine() playback.Deps {
	d := playback.Deps{Service: a.env.deps.Service, Media: a.env.deps.Media}
	if a.env.deps.History != nil {
		d.History = a.env.deps.History
	}
	return d
}

func (a *app) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.RequestBackgroundColor, a.env.loadHistory(), a.activate(navContinue)}
	if a.env.covers != nil {
		cmds = append(cmds, tea.Raw(kittyQuery))
	}
	if a.loadErr != nil {
		cmds = append(cmds, toastErr(fmt.Sprintf("settings.json: %v", a.loadErr)))
	}
	return tea.Batch(cmds...)
}

// section returns the live screen for a navigation entry, creating it on
// first use.
func (a *app) section(i int) (Screen, tea.Cmd) {
	if a.sections[i] != nil {
		return a.sections[i], nil
	}
	e := a.env
	var s Screen
	switch i {
	case navContinue:
		s = newContinueSection(e, false)
	case navDiscover:
		s = newDiscoverSection(e)
	case navHot:
		s = newHotSection(e)
	case navRank:
		s = newRankSection(e)
	case navFilter:
		s = newFilterSection(e)
	case navNew:
		s = newNewSection(e)
	case navHistory:
		s = newContinueSection(e, true)
	case navDownloads:
		s = newDownloadsSection(e, a.downloads)
	case navBooks:
		s = newBooksSection(e)
	case navSettings:
		s = newSettingsSection(e, a.saveSettings, a.applySettings)
	}
	a.sections[i] = s
	return s, s.Init()
}

func (a *app) navTitle(i int) func() string {
	return func() string {
		tr := a.env.tr
		names := [navCount]string{
			tr.SecContinue, tr.SecDiscover, tr.SecHot, tr.SecRank, tr.SecFilter,
			tr.SecNew, tr.SecHistory, tr.SecDownloads, tr.SecBooks, tr.SecSettings,
		}
		return names[i]
	}
}

func (a *app) activate(i int) tea.Cmd {
	a.nav = i
	a.stack = nil
	_, cmd := a.section(i)
	return cmd
}

// active is the screen that takes keys: the top pushed page, else the
// current section.
func (a *app) active() Screen {
	if n := len(a.stack); n > 0 {
		return a.stack[n-1]
	}
	s, _ := a.section(a.nav)
	return s
}

func (a *app) live() []Screen {
	var out []Screen
	for _, s := range a.sections {
		if s != nil {
			out = append(out, s)
		}
	}
	return append(out, a.stack...)
}

func (a *app) push(s Screen) tea.Cmd {
	a.stack = append(a.stack, s)
	a.focus = focusMain
	return s.Init()
}

func (a *app) pop() {
	if n := len(a.stack); n > 0 {
		a.stack = a.stack[:n-1]
	}
}

func (a *app) saveSettings() tea.Cmd {
	if err := saveSettings(a.settingsPath, *a.env.settings); err != nil {
		return toastErr(err.Error())
	}
	return nil
}

func (a *app) applySettings() {
	s := a.env.settings
	a.env.tr = stringsFor(s.Language)
	*a.env.theme = newTheme(s.Theme, a.dark)
}

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		return a, nil
	case tea.BackgroundColorMsg:
		a.dark = msg.IsDark()
		a.applySettings()
		return a, nil
	case tea.KeyPressMsg:
		return a, a.key(msg)
	case toastMsg:
		return a, a.toasts.add(msg.text, msg.err)
	case toastTickMsg:
		return a, a.toasts.expire()
	case historyListMsg:
		if msg.err == nil {
			a.env.setHistory(msg.entries)
		}
	case clearedHistoryMsg:
		a.env.setHistory(nil)
		return a, tea.Batch(toast(a.env.tr.ToastCleared), a.broadcast(historyListMsg{}))
	case seriesDetailMsg:
		if msg.err == nil && msg.series != nil {
			a.env.setDetail(msg.id, msg.series)
			if a.env.coversOn() {
				return a, a.env.covers.fetch(a.env.ctx, msg.series.CoverURL)
			}
		}
		return a, nil
	case coverMsg:
		return a, a.env.covers.store(msg)
	case uv.KittyGraphicsEvent:
		if a.env.covers != nil && isKittyAnswer(msg) {
			return a, a.env.covers.useKitty()
		}
		return a, nil
	case openSeriesMsg:
		return a, a.push(newSeriesPage(a.env, msg.id))
	case openPersonMsg:
		return a, a.push(newPersonPage(a.env, msg.id))
	case openSearchMsg:
		return a, a.push(newSearchPage(a.env, msg.query))
	case pushScreenMsg:
		return a, a.push(msg.screen)
	case popScreenMsg:
		a.pop()
		return a, nil
	case promptMsg:
		a.overlay = newPrompt(msg.id, msg.label, msg.initial, a.env.theme)
		return a, nil
	case playMsg:
		return a, a.play(msg)
	case playerEventMsg:
		return a, a.playerEvent(msg)
	case playerDoneMsg:
		cmd := a.env.loadHistory()
		if msg.err != nil {
			return a, tea.Batch(cmd, toastErr(describeError(msg.err, a.env.tr)))
		}
		return a, tea.Batch(cmd, toast(a.env.tr.ToastPlaybackDone))
	case downloadMsg:
		return a, a.download(msg)
	case downloadDoneMsg:
		tr := a.env.tr
		switch {
		case msg.err != nil && msg.err != context.Canceled:
			return a, toastErr(describeError(msg.err, tr))
		case msg.failed > 0:
			return a, toastErr(fmt.Sprintf(tr.ToastDownloadFailed, msg.failed))
		case msg.err == nil:
			return a, toast(fmt.Sprintf(tr.ToastDownloadDone, msg.title))
		}
		return a, nil
	}
	var overlayCmd tea.Cmd
	if a.overlay != nil {
		var done bool
		done, overlayCmd = a.overlay.Update(msg)
		if done {
			a.overlay = nil
		}
	}
	return a, tea.Batch(overlayCmd, a.broadcast(msg))
}

// broadcast hands a message to every live screen; each one keeps only the
// answers to its own requests.
func (a *app) broadcast(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	for _, s := range a.live() {
		if cmd := s.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (a *app) key(msg tea.KeyPressMsg) tea.Cmd {
	k := keyString(msg)
	if a.overlay != nil {
		done, cmd := a.overlay.Update(msg)
		if done {
			a.overlay = nil
		}
		return cmd
	}
	if k == "ctrl+c" {
		a.player.stop()
		return tea.Quit
	}
	if c, ok := a.active().(capturer); ok && c.capturing() {
		return a.active().Update(msg)
	}
	tr := a.env.tr
	switch k {
	case "q":
		if len(a.stack) > 0 {
			a.pop()
			return nil
		}
		a.player.stop()
		return tea.Quit
	case "?":
		a.overlay = &helpOverlay{tr: tr, global: a.globalKeys(), screen: a.active().Keys(), title: a.active().Title()}
		return nil
	case "/":
		if p, ok := a.active().(*searchPage); ok {
			return p.focusInput()
		}
		return a.push(newSearchPage(a.env, ""))
	case "S":
		if a.player.stop() {
			return toast(tr.ToastStopped)
		}
		return nil
	case "ctrl+l":
		a.env.settings.Language = cycle(languages, a.env.settings.Language, 1)
		a.applySettings()
		return a.saveSettings()
	case "esc":
		if len(a.stack) > 0 {
			a.pop()
			return nil
		}
		a.focus = focusNav
		return nil
	case "tab":
		if len(a.stack) == 0 {
			if a.focus == focusNav {
				a.focus = focusMain
			} else {
				a.focus = focusNav
			}
			return nil
		}
	}
	if len(k) == 1 && k[0] >= '0' && k[0] <= '9' && len(a.stack) == 0 {
		i := int(k[0]-'1'+10) % 10
		if i < navCount {
			a.focus = focusMain
			return a.activate(i)
		}
	}
	if a.focus == focusNav && len(a.stack) == 0 {
		switch k {
		case "j", "down":
			return a.activate((a.nav + 1) % navCount)
		case "k", "up":
			return a.activate((a.nav + navCount - 1) % navCount)
		case "enter", "l", "right":
			a.focus = focusMain
			return nil
		}
		return nil
	}
	return a.active().Update(msg)
}

func (a *app) globalKeys() []binding {
	tr := a.env.tr
	return []binding{
		{"j k ↑ ↓", tr.KeyMove}, {"enter", tr.KeyOpen}, {"esc q", tr.KeyBack},
		{"tab", tr.KeyFocus}, {"1-9 0", tr.KeySection}, {"/", tr.KeySearch},
		{"S", tr.KeyStop}, {"ctrl+l", tr.KeyLang}, {"?", tr.KeyHelp}, {"ctrl+c", tr.KeyQuit},
	}
}

func (a *app) play(msg playMsg) tea.Cmd {
	tr, s := a.env.tr, a.env.settings
	opts := playback.Options{
		Quality: s.Quality, Player: s.Player, Autoplay: msg.autoplay, NoTerminal: true,
	}
	target := playback.Target{
		SeriesID: msg.seriesID, Title: msg.title, Episodes: msg.episodes,
		Start: msg.start, StartSeconds: msg.startSeconds, Remember: a.env.deps.History != nil,
	}
	if !a.player.start(a.env.ctx, a.engine(), opts, target) {
		return toastErr(tr.ToastAlreadyPlaying)
	}
	return toast(fmt.Sprintf(tr.ToastPlaying, msg.title, msg.start.Number))
}

func (a *app) playerEvent(msg playerEventMsg) tea.Cmd {
	tr := a.env.tr
	switch msg.ev.Kind {
	case playback.EventStarted, playback.EventFinished, playback.EventStopped, playback.EventJumped:
		return a.env.loadHistory()
	case playback.EventSkipped:
		return toastErr(fmt.Sprintf(tr.ToastSkipped, msg.ev.Episode.Number))
	case playback.EventHistoryWarning:
		return toastErr(fmt.Sprintf(tr.ToastHistory, msg.ev.Err))
	}
	return nil
}

func (a *app) download(msg downloadMsg) tea.Cmd {
	s := a.env.settings
	dir := playback.DefaultDir(msg.title, msg.seriesID)
	if s.DownloadDir != "" {
		dir = filepath.Join(s.DownloadDir, dir)
	}
	a.downloads.enqueue(&downloadTask{
		seriesID: msg.seriesID, title: msg.title, dir: dir, episodes: msg.episodes, numbers: msg.numbers,
	})
	return toast(fmt.Sprintf(a.env.tr.ToastQueued, msg.title))
}
