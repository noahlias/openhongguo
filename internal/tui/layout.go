package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	navWidth       = 14
	minNavScreen   = 72
	minDetailWidth = 100
)

// panes are the widths of the three columns for a screen width; a zero
// width hides the pane.
type panes struct {
	nav, main, detail int
}

func layoutPanes(w int) panes {
	p := panes{}
	if w >= minNavScreen {
		p.nav = navWidth
	}
	if w >= minDetailWidth {
		p.detail = w * 2 / 5
		if p.detail > 60 {
			p.detail = 60
		}
	}
	p.main = w - p.nav - p.detail
	if p.nav > 0 {
		p.main--
	}
	if p.detail > 0 {
		p.main--
	}
	if p.main < 10 {
		p.main = 10
	}
	return p
}

// fit makes s exactly h lines of w cells.
func fit(s string, w, h int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, l := range lines {
		lines[i] = pad(l, w)
	}
	for len(lines) < h {
		lines = append(lines, strings.Repeat(" ", w))
	}
	return strings.Join(lines, "\n")
}

// separator is a vertical line of h cells.
func separator(h int, st lipgloss.Style) string {
	line := st.Render("│")
	parts := make([]string, h)
	for i := range parts {
		parts[i] = line
	}
	return strings.Join(parts, "\n")
}

// topBar names the app and the current screen and hints the global keys.
func topBar(w int, th *Theme, tr *Strings, title string) string {
	left := th.Title.Render(" "+tr.AppTitle) + th.Subtle.Render(" │ ") + clip(title, max(w/2, 10))
	right := th.Key.Render("/") + " " + tr.KeySearch + "  " + th.Key.Render("?") + " " + tr.KeyHelp + " "
	return twoColumns(left, right, w)
}

// statusBar shows playback and the download queue.
func statusBar(w int, th *Theme, tr *Strings, playing string, downloads int, hint string) string {
	left := " " + playing
	if downloads > 0 {
		left += th.Subtle.Render(fmt.Sprintf("  ↓ %d", downloads))
	}
	return th.StatusBar.Render(twoColumns(left, hint+" ", w))
}
