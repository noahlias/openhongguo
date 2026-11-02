package tui

import (
	"fmt"
	"strings"

	"github.com/beilunyang/openhongguo/internal/history"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// episodeGrid lays episode numbers out in columns and moves a cursor over
// them in two dimensions.
type episodeGrid struct {
	cursor int
	count  int
	cols   int
	offset int
}

func (g *episodeGrid) setCount(n int) {
	g.count = n
	if g.cursor >= n {
		g.cursor = n - 1
	}
	if g.cursor < 0 {
		g.cursor = 0
	}
}

func (g *episodeGrid) handleKey(k string) bool {
	if g.count == 0 {
		return false
	}
	cols := g.cols
	if cols < 1 {
		cols = 1
	}
	switch k {
	case "l", "right":
		g.cursor++
	case "h", "left":
		g.cursor--
	case "j", "down":
		g.cursor += cols
	case "k", "up":
		g.cursor -= cols
	case "pgdown", "ctrl+d":
		g.cursor += cols * 5
	case "pgup", "ctrl+u":
		g.cursor -= cols * 5
	case "g", "home":
		g.cursor = 0
	case "G", "end":
		g.cursor = g.count - 1
	default:
		return false
	}
	if g.cursor >= g.count {
		g.cursor = g.count - 1
	}
	if g.cursor < 0 {
		g.cursor = 0
	}
	return true
}

// view draws the grid in w×h cells and sets the column count for key
// handling.
func (g *episodeGrid) view(items []model.Episode, e *env, seriesID string, w, h int, focused bool) string {
	th := e.theme
	if len(items) == 0 || w < 6 || h < 1 {
		return th.Subtle.Render(clip(e.tr.Empty, w))
	}
	digits := len(fmt.Sprint(items[len(items)-1].Number))
	if digits < 2 {
		digits = 2
	}
	cellW := digits + 3
	g.cols = w / cellW
	if g.cols < 1 {
		g.cols = 1
	}
	rows := (len(items) + g.cols - 1) / g.cols
	curRow := g.cursor / g.cols
	if curRow < g.offset {
		g.offset = curRow
	}
	if curRow >= g.offset+h {
		g.offset = curRow - h + 1
	}
	if g.offset < 0 {
		g.offset = 0
	}
	hist, watched := e.watched(seriesID)
	var b strings.Builder
	for r := g.offset; r < rows && r < g.offset+h; r++ {
		if r > g.offset {
			b.WriteString("\n")
		}
		line := ""
		for c := 0; c < g.cols; c++ {
			i := r*g.cols + c
			if i >= len(items) {
				break
			}
			line += g.cell(items[i], i == g.cursor, focused, hist, watched, digits, th)
		}
		b.WriteString(pad(line, w))
	}
	for r := rows - g.offset; r < h; r++ {
		b.WriteString("\n" + strings.Repeat(" ", w))
	}
	return b.String()
}

func (g *episodeGrid) cell(ep model.Episode, selected, focused bool, hist history.Entry, watched bool, digits int, th *Theme) string {
	mark := " "
	style := th.Row
	switch {
	case !ep.Playable:
		mark, style = "x", th.Locked
	case watched && ep.Number == hist.Episode && !hist.Finished:
		mark, style = ">", th.Current
	case watched && (ep.Number < hist.Episode || (ep.Number == hist.Episode && hist.Finished)):
		mark, style = ".", th.Watched
	case ep.IsNew:
		mark, style = "*", th.New
	}
	text := fmt.Sprintf(" %*d%s ", digits, ep.Number, mark)
	if selected && focused {
		return th.RowSelected.Render(text)
	}
	if selected {
		return th.RowBlurred.Render(text)
	}
	return style.Render(text)
}
