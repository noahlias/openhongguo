package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func (a *app) View() tea.View {
	v := tea.NewView(a.render())
	v.AltScreen = true
	v.WindowTitle = "ohg"
	return v
}

func (a *app) render() string {
	w, h := a.width, a.height
	th, tr := a.env.theme, a.env.tr
	if w < 20 || h < 6 {
		return tr.Loading
	}
	screen := a.active()
	bodyH := h - 2
	p := layoutPanes(w)
	var cols []string
	if p.nav > 0 {
		cols = append(cols, fit(a.renderNav(p.nav, bodyH), p.nav, bodyH), separator(bodyH, th.Border))
	}
	mainW := p.main
	detail := ""
	if p.detail > 0 {
		detail = screen.Detail(p.detail, bodyH)
		if detail == "" {
			mainW += p.detail + 1
		}
	}
	cols = append(cols, fit(screen.View(mainW, bodyH), mainW, bodyH))
	if detail != "" {
		cols = append(cols, separator(bodyH, th.Border), fit(detail, p.detail, bodyH))
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, cols...)
	title := screen.Title()
	if len(a.stack) > 0 {
		title = a.navTitle(a.nav)() + th.Subtle.Render(" › ") + title
	}
	out := topBar(w, th, tr, title) + "\n" + body + "\n" + a.renderStatus(w)
	if a.overlay != nil {
		box := a.overlay.View(w, h, th)
		out = overlayOn(out, box, w, h)
	}
	if lines := a.toasts.lines(w, th); len(lines) > 0 {
		out = toastsOn(out, lines, w, h)
	}
	return out
}

func (a *app) renderNav(w, h int) string {
	th, tr := a.env.theme, a.env.tr
	var lines []string
	for i := 0; i < navCount; i++ {
		n := (i + 1) % 10
		label := fmt.Sprintf("%d %s", n, a.navTitle(i)())
		switch {
		case i == a.nav && a.focus == focusNav && len(a.stack) == 0:
			label = th.NavFocused.Render(pad(label, w))
		case i == a.nav:
			label = th.NavSelected.Render(pad(label, w))
		default:
			label = th.NavItem.Render(pad(label, w))
		}
		lines = append(lines, label)
	}
	if title, ep, total, running := a.player.status(); running {
		for len(lines) < h-3 {
			lines = append(lines, "")
		}
		lines = append(lines, th.Accent.Render(clip("▶ "+tr.Playing, w)))
		lines = append(lines, clip(title, w))
		lines = append(lines, th.Subtle.Render(clip(fmt.Sprintf("%d / %d", ep, total), w)))
	}
	return strings.Join(lines, "\n")
}

func (a *app) renderStatus(w int) string {
	th, tr := a.env.theme, a.env.tr
	playing := th.Subtle.Render(tr.Idle)
	if title, ep, total, running := a.player.status(); running {
		playing = th.Accent.Render("▶ ") + clip(title, w/3) + th.Subtle.Render(fmt.Sprintf(" %d/%d", ep, total)) + "  " + th.Key.Render("S") + " " + tr.KeyStop
	}
	hint := ""
	if len(a.stack) > 0 {
		hint = th.Key.Render("esc") + " " + tr.KeyBack
	} else if a.focus == focusNav {
		hint = th.Key.Render("enter") + " " + tr.KeyOpen
	}
	return statusBar(w, th, tr, playing, a.downloads.active(), hint)
}

// overlayOn draws box centered over base.
func overlayOn(base, box string, w, h int) string {
	baseLines := strings.Split(base, "\n")
	boxLines := strings.Split(box, "\n")
	bw := 0
	for _, l := range boxLines {
		if n := width(l); n > bw {
			bw = n
		}
	}
	top := (h - len(boxLines)) / 2
	left := (w - bw) / 2
	if top < 0 {
		top = 0
	}
	if left < 0 {
		left = 0
	}
	for i, l := range boxLines {
		y := top + i
		if y >= len(baseLines) {
			break
		}
		baseLines[y] = ansiOverlay(baseLines[y], pad(l, bw), left, w)
	}
	return strings.Join(baseLines, "\n")
}

// toastsOn draws the notices above the status bar, right-aligned.
func toastsOn(base string, lines []string, w, h int) string {
	baseLines := strings.Split(base, "\n")
	for i, l := range lines {
		y := h - 2 - len(lines) + i
		if y < 1 || y >= len(baseLines) {
			continue
		}
		lw := width(l)
		baseLines[y] = ansiOverlay(baseLines[y], l, w-lw-1, w)
	}
	return strings.Join(baseLines, "\n")
}

// ansiOverlay puts piece over line starting at column x.
func ansiOverlay(line, piece string, x, w int) string {
	pw := width(piece)
	if x < 0 {
		x = 0
	}
	if x+pw > w {
		x = w - pw
		if x < 0 {
			x = 0
		}
	}
	left := ansi.Truncate(line, x, "")
	left = pad(left, x)
	rest := ansi.TruncateLeft(line, x+pw, "")
	return left + piece + rest
}
