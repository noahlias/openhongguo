package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// downloadsSection lists the download queue as the controller sees it.
type downloadsSection struct {
	e      *env
	ctl    *downloadCtl
	cursor int
}

func newDownloadsSection(e *env, ctl *downloadCtl) *downloadsSection {
	return &downloadsSection{e: e, ctl: ctl}
}

func (s *downloadsSection) Init() tea.Cmd { return nil }
func (s *downloadsSection) Title() string { return s.e.tr.SecDownloads }
func (s *downloadsSection) Keys() []binding {
	tr := s.e.tr
	return []binding{{"c", tr.KeyCancel}, {"x", tr.KeyRemove}, {"enter", tr.KeyOpen}}
}

func (s *downloadsSection) Update(msg tea.Msg) tea.Cmd {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	tasks := s.ctl.snapshot()
	s.cursor = min(s.cursor, max(len(tasks)-1, 0))
	switch keyString(key) {
	case "j", "down":
		s.cursor = min(s.cursor+1, max(len(tasks)-1, 0))
	case "k", "up":
		s.cursor = max(s.cursor-1, 0)
	case "c":
		if s.ctl.cancelCurrent() {
			return toast(s.e.tr.KeyCancel)
		}
	case "x":
		if s.cursor < len(tasks) {
			s.ctl.remove(tasks[s.cursor].id)
		}
	case "enter":
		if s.cursor < len(tasks) {
			return emit(openSeriesMsg{id: tasks[s.cursor].seriesID})
		}
	}
	return nil
}

func (s *downloadsSection) stateLabel(t downloadTask) string {
	tr, th := s.e.tr, s.e.theme
	switch t.state {
	case taskWaiting:
		return th.Subtle.Render(tr.Waiting)
	case taskRunning:
		return th.Accent.Render(tr.Saving)
	case taskDone:
		return th.Subtle.Render(tr.Done)
	case taskCancelled:
		return th.Subtle.Render(tr.KeyCancel)
	}
	return th.ToastErr.Render(tr.Failed)
}

// bar draws done/total as a fixed-width bar of ASCII so it lines up.
func bar(done, total, w int) string {
	if total <= 0 || w < 3 {
		return ""
	}
	fill := done * w / total
	return "[" + strings.Repeat("#", fill) + strings.Repeat(".", w-fill) + "]"
}

func (s *downloadsSection) View(w, h int) string {
	th, tr := s.e.theme, s.e.tr
	tasks := s.ctl.snapshot()
	if len(tasks) == 0 {
		return th.Subtle.Render(clip(tr.EmptyDownloads, w))
	}
	s.cursor = min(s.cursor, len(tasks)-1)
	var lines []string
	for i, t := range tasks {
		right := fmt.Sprintf("%s %d/%d  %s", bar(t.done(), t.total(), 16), t.done(), t.total(), s.stateLabel(t))
		line := twoColumns(t.title, right, w)
		if i == s.cursor {
			line = th.RowSelected.Render(pad(line, w))
		}
		lines = append(lines, pad(line, w))
	}
	return strings.Join(lines[:min(len(lines), h)], "\n")
}

func (s *downloadsSection) Detail(w, h int) string {
	tasks := s.ctl.snapshot()
	if s.cursor >= len(tasks) {
		return ""
	}
	t := tasks[s.cursor]
	th, tr := s.e.theme, s.e.tr
	lines := []string{
		th.Title.Render(clip(t.title, w)),
		s.stateLabel(t),
		"",
		labelled(tr.Done, itoa(t.saved), th),
		labelled(tr.Skipped, itoa(t.skipped), th),
		labelled(tr.Failed, itoa(t.failed), th),
		"",
		th.Label.Render(tr.SetDownloadDir),
	}
	lines = append(lines, wrap(t.dir, w)...)
	if t.last != "" {
		lines = append(lines, "", th.Subtle.Render(clip(t.last, w)))
	}
	if t.err != nil {
		lines = append(lines, "")
		lines = append(lines, wrap(describeError(t.err, tr)+": "+t.err.Error(), w)...)
	}
	for i := range lines {
		lines[i] = clip(lines[i], w)
	}
	return strings.Join(lines[:min(len(lines), h)], "\n")
}
