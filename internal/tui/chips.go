package tui

import (
	"strings"

	"github.com/beilunyang/openhongguo/sdk/model"
)

// chipRow is one filter dimension with at most one option chosen.
type chipRow struct {
	key      string
	name     string
	options  []model.FilterOption
	selected int
}

// chips is a grid of filter dimensions the cursor moves over.
type chips struct {
	rows []chipRow
	row  int
	col  int
}

func (c *chips) set(rows []chipRow) {
	c.rows = rows
	c.row, c.col = 0, 0
	for i := range c.rows {
		c.rows[i].selected = -1
	}
}

func (c *chips) empty() bool { return len(c.rows) == 0 }

// handleKey moves or toggles and reports whether the selection changed.
func (c *chips) handleKey(k string) (handled, changed bool) {
	if len(c.rows) == 0 {
		return false, false
	}
	switch k {
	case "j", "down":
		c.row = min(c.row+1, len(c.rows)-1)
	case "k", "up":
		c.row = max(c.row-1, 0)
	case "l", "right":
		c.col = min(c.col+1, len(c.rows[c.row].options)-1)
	case "h", "left":
		c.col = max(c.col-1, 0)
	case "space", "enter":
		r := &c.rows[c.row]
		if r.selected == c.col {
			r.selected = -1
		} else {
			r.selected = c.col
		}
		return true, true
	case "x":
		for i := range c.rows {
			c.rows[i].selected = -1
		}
		return true, true
	default:
		return false, false
	}
	if c.col >= len(c.rows[c.row].options) {
		c.col = max(len(c.rows[c.row].options)-1, 0)
	}
	return true, false
}

// selections maps each dimension with a choice to its value.
func (c *chips) selections() map[string]string {
	out := map[string]string{}
	for _, r := range c.rows {
		if r.selected >= 0 && r.selected < len(r.options) {
			out[r.key] = r.options[r.selected].Value
		}
	}
	return out
}

// view draws one line per dimension, scrolling the options so the cursor
// stays visible.
func (c *chips) view(w int, th *Theme, focused bool) []string {
	lines := make([]string, 0, len(c.rows))
	nameW := 0
	for _, r := range c.rows {
		nameW = max(nameW, width(r.name))
	}
	avail := w - nameW - 1
	for i, r := range c.rows {
		start := 0
		if i == c.row {
			used := 0
			for j := 0; j <= c.col && j < len(r.options); j++ {
				used += width(r.options[j].Name) + 3
			}
			for used > avail-2 && start < c.col {
				used -= width(r.options[start].Name) + 3
				start++
			}
		}
		var b strings.Builder
		b.WriteString(th.Label.Render(pad(r.name, nameW)))
		b.WriteString(" ")
		used := 0
		if start > 0 {
			b.WriteString(th.Subtle.Render("‹"))
			used++
		}
		for j := start; j < len(r.options); j++ {
			name := r.options[j].Name
			if used+width(name)+3 > avail {
				b.WriteString(th.Subtle.Render("›"))
				break
			}
			cell := " " + name + " "
			switch {
			case focused && i == c.row && j == c.col:
				cell = th.RowSelected.Render(cell)
			case j == r.selected:
				cell = th.ChipOn.Render(cell)
			default:
				cell = th.ChipOff.Render(cell)
			}
			b.WriteString(cell + " ")
			used += width(name) + 3
		}
		lines = append(lines, pad(b.String(), w))
	}
	return lines
}
