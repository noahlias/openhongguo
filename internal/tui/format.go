package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func itoa(n int) string { return strconv.Itoa(n) }

// formatCount shortens a raw count the way the app does: 1.2万, 3.4亿 in
// Chinese; 12.3K, 1.2M in English. Zero stays empty so rows do not fill
// with noise.
func formatCount(n int64, lang string) string {
	if n <= 0 {
		return ""
	}
	if lang == "zh" {
		switch {
		case n >= 100_000_000:
			return trimZero(float64(n)/1e8) + "亿"
		case n >= 10_000:
			return trimZero(float64(n)/1e4) + "万"
		}
		return strconv.FormatInt(n, 10)
	}
	switch {
	case n >= 1_000_000_000:
		return trimZero(float64(n)/1e9) + "B"
	case n >= 1_000_000:
		return trimZero(float64(n)/1e6) + "M"
	case n >= 1_000:
		return trimZero(float64(n)/1e3) + "K"
	}
	return strconv.FormatInt(n, 10)
}

func trimZero(f float64) string {
	s := strconv.FormatFloat(f, 'f', 1, 64)
	return strings.TrimSuffix(s, ".0")
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	s := int(d.Seconds() + 0.5)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s%3600/60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

func formatSeconds(sec float64) string {
	return formatDuration(time.Duration(sec * float64(time.Second)))
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// width is the number of terminal cells s takes.
func width(s string) int { return ansi.StringWidth(s) }

// clip cuts s to at most w cells, ending with … when it had to cut.
func clip(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if width(s) <= w {
		return s
	}
	if w == 1 {
		return ansi.Truncate(s, 1, "")
	}
	return ansi.Truncate(s, w, "…")
}

// pad clips s to w cells and fills the rest with spaces.
func pad(s string, w int) string {
	s = clip(s, w)
	if n := w - width(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

// padLeft right-aligns s in w cells.
func padLeft(s string, w int) string {
	s = clip(s, w)
	if n := w - width(s); n > 0 {
		return strings.Repeat(" ", n) + s
	}
	return s
}

// twoColumns lays left and right out on one line of w cells, giving the
// right side what it needs and the left side the rest.
func twoColumns(left, right string, w int) string {
	rw := width(right)
	if rw >= w {
		return clip(right, w)
	}
	lw := w - rw
	if rw > 0 {
		lw--
	}
	out := pad(left, lw)
	if rw > 0 {
		out += " " + right
	}
	return out
}

// wrap breaks s into lines of at most w cells. It breaks at spaces when it
// can and inside a run of CJK text otherwise.
func wrap(s string, w int) []string {
	if w <= 0 {
		return nil
	}
	var lines []string
	for _, para := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		lines = append(lines, strings.Split(ansi.Wrap(para, w, ""), "\n")...)
	}
	return lines
}

func joinNonEmpty(sep string, parts ...string) string {
	var out []string
	for _, p := range parts {
		if strings.TrimSpace(ansi.Strip(p)) != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}
