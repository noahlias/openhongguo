package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// View hands kitty covers to the renderer inside the frame: the renderer
// writes a sequence just before the cell it precedes, so a placement lands
// on the cover's first cell. A frame showing no cover starts by clearing
// the placements, so a cover never outlives the frame that drew it; an
// overlay hides the cover, which would otherwise be drawn above it.
func (a *app) View() tea.View {
	c := a.env.covers
	kitty := c != nil && c.kitty()
	keep := func(seq string) bool { return kitty && a.overlay == nil && c.isPlacement(seq) }
	frame, placed := dropStringSequences(a.render(), keep)
	if kitty && !placed {
		frame = kittyClear + frame
	}
	v := tea.NewView(frame)
	v.AltScreen = true
	v.WindowTitle = "ohg"
	return v
}

// dropStringSequences removes OSC, DCS, APC, SOS and PM sequences from the
// frame except those keep accepts, and reports whether it kept any. Others
// came from service text, and the renderer would pass the latter four to
// the terminal.
func dropStringSequences(frame string, keep func(string) bool) (string, bool) {
	kept := false
	p := ansi.GetParser()
	defer ansi.PutParser(p)
	var b strings.Builder
	var state byte
	for len(frame) > 0 {
		seq, _, n, next := ansi.DecodeSequence(frame, state, p)
		switch {
		case !ansi.HasOscPrefix(seq) && !ansi.HasDcsPrefix(seq) && !ansi.HasApcPrefix(seq) &&
			!ansi.HasSosPrefix(seq) && !ansi.HasPmPrefix(seq):
			b.WriteString(seq)
		case keep(seq):
			b.WriteString(seq)
			kept = true
		}
		state, frame = next, frame[n:]
	}
	return b.String(), kept
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
	mainW := p.main
	detail := ""
	if p.detail > 0 {
		detail = screen.Detail(p.detail-4, bodyH-2)
		if detail == "" {
			mainW = w
		}
	}
	border := th.PanelFocus
	if a.focus == focusNav && len(a.stack) == 0 {
		border = th.Panel
	}
	cols := []string{card(a.panelTitle(screen), screen.View(mainW-4, bodyH-2), mainW, bodyH, border)}
	if detail != "" {
		cols = append(cols, fit("", 1, bodyH), card("", detail, p.detail, bodyH, th.Panel))
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, cols...)
	out := a.renderHeader(w) + "\n" + body + "\n" + a.renderFooter(w)
	if a.overlay != nil {
		box := a.overlay.View(w, h, th)
		out = overlayOn(out, box, w, h)
	}
	if lines := a.toasts.lines(w, th); len(lines) > 0 {
		out = toastsOn(out, lines, w, h)
	}
	return out
}

// panelTitle names the main card, with the section in front of a page.
func (a *app) panelTitle(screen Screen) string {
	th := a.env.theme
	title := " " + th.PanelTitle.Render(screen.Title()) + " "
	if len(a.stack) > 0 {
		title = " " + th.Subtle.Render(a.navTitle(a.nav)()+" › ") + th.PanelTitle.Render(screen.Title()) + " "
	}
	return title
}

// renderHeader is the brand, the sections as pills and the global keys.
func (a *app) renderHeader(w int) string {
	th, tr := a.env.theme, a.env.tr
	brand := th.Brand.Render(tr.AppTitle) + " "
	right := " " + keyHints([]binding{{"/", tr.KeySearch}, {"?", tr.KeyHelp}}, w, th)
	navW := w - width(brand) - width(right)
	if navW < 10 {
		right = ""
		navW = w - width(brand)
	}
	pills := make([]string, navCount)
	for i := range pills {
		label := itoa((i+1)%10) + " " + a.navTitle(i)()
		switch {
		case i == a.nav && a.focus == focusNav && len(a.stack) == 0:
			pills[i] = th.NavFocused.Render(label)
		case i == a.nav:
			pills[i] = th.NavSelected.Render(label)
		default:
			pills[i] = th.NavItem.Render(label)
		}
	}
	nav := strip(pills, a.nav, navW, "", th)
	if w < minNavScreen {
		nav = pills[a.nav]
	}
	return twoColumns(brand+nav, right, w)
}

// renderFooter is the active screen's keys, then playback and downloads.
func (a *app) renderFooter(w int) string {
	th, tr := a.env.theme, a.env.tr
	right := th.Subtle.Render("● " + tr.Idle)
	if title, ep, total, running := a.player.status(); running {
		right = th.Accent.Render("▶ ") + clip(title, w/4) + th.Subtle.Render(fmt.Sprintf(" %d/%d ", ep, total)) + th.KeyCap.Render("S") + " " + th.Subtle.Render(tr.KeyStop)
	}
	if n := a.downloads.active(); n > 0 {
		right = th.Heat.Render(fmt.Sprintf("↓ %d", n)) + "  " + right
	}
	var keys []binding
	switch {
	case len(a.stack) > 0:
		keys = append([]binding{{"esc", tr.KeyBack}}, a.active().Keys()...)
	case a.focus == focusNav:
		keys = []binding{{"← →", tr.KeySection}, {"enter", tr.KeyOpen}}
	default:
		keys = a.active().Keys()
	}
	left := " " + keyHints(keys, w-width(right)-3, th)
	return twoColumns(left, right+" ", w)
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
