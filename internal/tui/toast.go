package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

type toastItem struct {
	text  string
	err   bool
	until time.Time
}

type toastTickMsg struct{}

// toasts is a short queue of notices drawn over the bottom of the screen.
type toasts struct {
	items []toastItem
}

const toastLife = 3500 * time.Millisecond

func (t *toasts) add(text string, isErr bool) tea.Cmd {
	if text == "" {
		return nil
	}
	t.items = append(t.items, toastItem{text: text, err: isErr, until: time.Now().Add(toastLife)})
	if len(t.items) > 3 {
		t.items = t.items[len(t.items)-3:]
	}
	return tea.Tick(toastLife, func(time.Time) tea.Msg { return toastTickMsg{} })
}

// expire drops notices past their time and returns another tick while some
// remain.
func (t *toasts) expire() tea.Cmd {
	now := time.Now()
	kept := t.items[:0]
	for _, it := range t.items {
		if it.until.After(now) {
			kept = append(kept, it)
		}
	}
	t.items = kept
	if len(t.items) == 0 {
		return nil
	}
	return tea.Tick(t.items[0].until.Sub(now)+10*time.Millisecond, func(time.Time) tea.Msg { return toastTickMsg{} })
}

// lines draws the notices, newest last, each at most w cells.
func (t *toasts) lines(w int, th *Theme) []string {
	var out []string
	for _, it := range t.items {
		st := th.ToastInfo
		if it.err {
			st = th.ToastErr
		}
		out = append(out, st.Render(clip(it.text, w-2)))
	}
	return out
}
