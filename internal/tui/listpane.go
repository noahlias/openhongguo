package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// row is one entry of a list pane; it may span several lines.
type row interface {
	key() string
	render(w int, selected bool, th *Theme) string
}

// listPane is a cursor over rows that scrolls to keep the cursor visible
// and drops rows it has already seen, like the SDK pager does.
type listPane struct {
	rows   []row
	cursor int
	offset int
	seen   map[string]bool
}

func (l *listPane) clear() {
	l.rows, l.cursor, l.offset = nil, 0, 0
	l.seen = map[string]bool{}
}

// add appends rows whose key is new and returns how many it kept.
func (l *listPane) add(rows []row) int {
	if l.seen == nil {
		l.seen = map[string]bool{}
	}
	n := 0
	for _, r := range rows {
		k := r.key()
		if k != "" && l.seen[k] {
			continue
		}
		l.seen[k] = true
		l.rows = append(l.rows, r)
		n++
	}
	return n
}

func (l *listPane) set(rows []row) {
	l.clear()
	l.add(rows)
}

// setAll replaces the rows and keeps repeats, for lists where one series
// appears under several headings.
func (l *listPane) setAll(rows []row) {
	l.clear()
	l.rows = rows
}

func (l *listPane) len() int { return len(l.rows) }

func (l *listPane) selected() row {
	if l.cursor < 0 || l.cursor >= len(l.rows) {
		return nil
	}
	return l.rows[l.cursor]
}

func (l *listPane) move(delta int) {
	l.cursor += delta
	l.clampCursor()
}

func (l *listPane) top()    { l.cursor = 0 }
func (l *listPane) bottom() { l.cursor = len(l.rows) - 1; l.clampCursor() }

func (l *listPane) clampCursor() {
	if l.cursor >= len(l.rows) {
		l.cursor = len(l.rows) - 1
	}
	if l.cursor < 0 {
		l.cursor = 0
	}
}

// nearEnd reports a cursor within n rows of the last one.
func (l *listPane) nearEnd(n int) bool {
	return len(l.rows) == 0 || l.cursor >= len(l.rows)-1-n
}

// handleKey moves the cursor for the list keys and reports whether it did.
func (l *listPane) handleKey(k string, pageSize int) bool {
	if pageSize < 1 {
		pageSize = 1
	}
	switch k {
	case "j", "down":
		l.move(1)
	case "k", "up":
		l.move(-1)
	case "pgdown", "ctrl+d":
		l.move(pageSize)
	case "pgup", "ctrl+u":
		l.move(-pageSize)
	case "g", "home":
		l.top()
	case "G", "end":
		l.bottom()
	default:
		return false
	}
	return true
}

// view draws h lines of w cells with the cursor's row whole on screen.
// Rows may span several lines; a bar in the two-cell gutter marks the
// selected one.
func (l *listPane) view(w, h int, th *Theme, focused bool) string {
	if h <= 0 || w <= 2 {
		return ""
	}
	if len(l.rows) == 0 {
		return fit("", w, h)
	}
	l.clampCursor()
	if l.cursor < l.offset {
		l.offset = l.cursor
	}
	used := 0
	for i := l.cursor; i >= l.offset; i-- {
		used += strings.Count(l.rows[i].render(w-2, false, th), "\n") + 1
		if used > h {
			l.offset = i + 1
			break
		}
	}
	if l.offset < 0 {
		l.offset = 0
	}
	lines := make([]string, 0, h)
	for i := l.offset; i < len(l.rows) && len(lines) < h; i++ {
		sel := i == l.cursor
		gutter := "  "
		switch {
		case sel && focused:
			gutter = th.Gutter.Render("▌") + " "
		case sel:
			gutter = th.Subtle.Render("▌") + " "
		}
		for _, line := range strings.Split(l.rows[i].render(w-2, sel, th), "\n") {
			if len(lines) == h {
				break
			}
			lines = append(lines, gutter+pad(line, w-2))
		}
	}
	for len(lines) < h {
		lines = append(lines, strings.Repeat(" ", w))
	}
	return strings.Join(lines, "\n")
}

// keyString names a key press the way the key maps spell it.
func keyString(msg tea.KeyPressMsg) string {
	s := msg.String()
	if s == " " {
		return "space"
	}
	return s
}
