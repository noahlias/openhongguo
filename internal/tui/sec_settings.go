package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// settingsSection edits the settings in place and saves on every change.
type settingsSection struct {
	e      *env
	cursor int
	save   func() tea.Cmd
	apply  func()
}

const (
	setLanguage = iota
	setTheme
	setPlayer
	setQuality
	setAutoplay
	setDownloadDir
	setJobs
	setCovers
	setResetDevice
	setCount
)

func newSettingsSection(e *env, save func() tea.Cmd, apply func()) *settingsSection {
	return &settingsSection{e: e, save: save, apply: apply}
}

func (s *settingsSection) Init() tea.Cmd          { return nil }
func (s *settingsSection) Title() string          { return s.e.tr.SecSettings }
func (s *settingsSection) Detail(int, int) string { return "" }

func (s *settingsSection) Keys() []binding {
	tr := s.e.tr
	return []binding{{"←/→ enter", tr.KeyChange}, {"e", tr.SetDownloadDir}}
}

func (s *settingsSection) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case promptDoneMsg:
		if msg.id == "download-dir" && msg.ok {
			s.e.settings.DownloadDir = strings.TrimSpace(msg.value)
			return s.save()
		}
	case tea.KeyPressMsg:
		k := keyString(msg)
		switch k {
		case "j", "down":
			s.cursor = (s.cursor + 1) % setCount
		case "k", "up":
			s.cursor = (s.cursor + setCount - 1) % setCount
		case "l", "right", "enter", "space":
			return s.change(1)
		case "h", "left":
			return s.change(-1)
		case "e":
			if s.cursor == setDownloadDir {
				return emit(promptMsg{id: "download-dir", label: s.e.tr.SetDownloadDir, initial: s.e.settings.DownloadDir})
			}
		}
	}
	return nil
}

func (s *settingsSection) change(by int) tea.Cmd {
	st := s.e.settings
	switch s.cursor {
	case setLanguage:
		st.Language = cycle(languages, st.Language, by)
	case setTheme:
		st.Theme = cycle(themes, st.Theme, by)
	case setPlayer:
		st.Player = cycle(players, st.Player, by)
	case setQuality:
		st.Quality = cycle(qualities, st.Quality, by)
	case setAutoplay:
		st.Autoplay = !st.Autoplay
	case setDownloadDir:
		return emit(promptMsg{id: "download-dir", label: s.e.tr.SetDownloadDir, initial: st.DownloadDir})
	case setJobs:
		st.Jobs += by
		if st.Jobs < 1 {
			st.Jobs = 16
		}
		if st.Jobs > 16 {
			st.Jobs = 1
		}
	case setCovers:
		st.Covers = !st.Covers
	case setResetDevice:
		return s.resetDevice()
	}
	s.apply()
	return s.save()
}

func (s *settingsSection) resetDevice() tea.Cmd {
	e := s.e
	if e.deps.DeviceReset == nil {
		return nil
	}
	ctx, r := e.ctx, e.deps.DeviceReset
	return func() tea.Msg {
		if err := r.ResetDevice(ctx); err != nil {
			return toastMsg{text: describeError(err, e.tr), err: true}
		}
		return toastMsg{text: e.tr.ToastDeviceReset}
	}
}

func (s *settingsSection) View(w, h int) string {
	tr, th, st := s.e.tr, s.e.theme, s.e.settings
	onOff := func(b bool) string {
		if b {
			return tr.On
		}
		return tr.Off
	}
	dir := st.DownloadDir
	if dir == "" {
		dir = tr.SetDefaultDir
	}
	items := []struct{ label, value string }{
		{tr.SetLanguage, stringsFor(st.Language).Language},
		{tr.SetTheme, st.Theme},
		{tr.SetPlayer, st.Player},
		{tr.SetQuality, st.Quality},
		{tr.SetAutoplay, onOff(st.Autoplay)},
		{tr.SetDownloadDir, dir},
		{tr.SetJobs, fmt.Sprint(st.Jobs)},
		{tr.SetCovers, onOff(st.Covers)},
		{tr.SetResetDevice, "↵"},
	}
	lw := 0
	for _, it := range items {
		if n := width(it.label); n > lw {
			lw = n
		}
	}
	lines := make([]string, 0, h)
	for i, it := range items {
		line := "  " + th.Label.Render(pad(it.label, lw)) + "   " + it.value
		if i == s.cursor {
			line = th.Gutter.Render("▌") + " " + th.RowTitle.Render(pad(it.label, lw)) + "  " +
				th.Subtle.Render("‹ ") + th.Accent.Bold(true).Render(it.value) + th.Subtle.Render(" ›")
		}
		lines = append(lines, pad(line, w), "")
	}
	for len(lines) < h {
		lines = append(lines, strings.Repeat(" ", w))
	}
	return strings.Join(lines[:min(h, len(lines))], "\n")
}
