package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	minNavScreen   = 72
	minDetailWidth = 100
	maxDetailWidth = 64
)

// panes are the widths of the main and detail cards for a screen width; a
// zero width hides the detail card. One column separates them.
type panes struct {
	main, detail int
}

func layoutPanes(w int) panes {
	p := panes{main: w}
	if w >= minDetailWidth {
		p.detail = min(w*2/5, maxDetailWidth)
		p.main = w - p.detail - 1
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

// card frames body in a rounded border of w×h cells with title set into
// the top edge. The body gets w-4 columns and h-2 lines.
func card(title, body string, w, h int, border lipgloss.Style) string {
	if w < 6 || h < 3 {
		return fit(body, w, h)
	}
	inner := w - 4
	top := border.Render("╭" + strings.Repeat("─", w-2) + "╮")
	if title != "" {
		title = clip(title, w-6)
		rest := w - 3 - width(title)
		top = border.Render("╭─") + title + border.Render(strings.Repeat("─", max(rest, 0))+"╮")
	}
	side := border.Render("│")
	lines := strings.Split(fit(body, inner, h-2), "\n")
	out := make([]string, 0, h)
	out = append(out, top)
	for _, l := range lines {
		out = append(out, side+" "+l+" "+side)
	}
	out = append(out, border.Render("╰"+strings.Repeat("─", w-2)+"╯"))
	return strings.Join(out, "\n")
}

// strip lays parts out left to right in w cells, sliding so active stays
// visible and marking what is cut off with ‹ and ›.
func strip(parts []string, active, w int, gap string, th *Theme) string {
	if len(parts) == 0 || w <= 0 {
		return ""
	}
	widths := make([]int, len(parts))
	for i, p := range parts {
		widths[i] = width(p) + width(gap)
	}
	start := 0
	for {
		total := 2
		for i := start; i <= active && i < len(parts); i++ {
			total += widths[i]
		}
		if total <= w || start >= active {
			break
		}
		start++
	}
	var b strings.Builder
	used := 0
	if start > 0 {
		b.WriteString(th.Subtle.Render("‹ "))
		used += 2
	}
	for i := start; i < len(parts); i++ {
		if used+widths[i] > w-1 && i > active {
			b.WriteString(th.Subtle.Render("›"))
			break
		}
		b.WriteString(parts[i])
		b.WriteString(gap)
		used += widths[i]
	}
	return clip(b.String(), w)
}

// keyHints draws bindings as keycaps, as many as fit in w cells.
func keyHints(items []binding, w int, th *Theme) string {
	var b strings.Builder
	used := 0
	for _, it := range items {
		part := th.KeyCap.Render(it.keys) + " " + th.Subtle.Render(it.help)
		n := width(part) + 2
		if used+n > w {
			break
		}
		b.WriteString(part)
		b.WriteString("  ")
		used += n
	}
	return b.String()
}
