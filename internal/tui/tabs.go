package tui

// tabs is a horizontal strip of names with one active.
type tabs struct {
	names  []string
	active int
}

func (t *tabs) set(names []string, active int) {
	t.names = names
	t.active = active
	t.clamp()
}

func (t *tabs) clamp() {
	if t.active >= len(t.names) {
		t.active = len(t.names) - 1
	}
	if t.active < 0 {
		t.active = 0
	}
}

func (t *tabs) move(delta int) bool {
	if len(t.names) < 2 {
		return false
	}
	n := len(t.names)
	t.active = ((t.active+delta)%n + n) % n
	return true
}

// handleKey switches tabs for [ and ] and reports whether it did.
func (t *tabs) handleKey(k string) bool {
	switch k {
	case "]", "l", "right":
		return t.move(1)
	case "[", "h", "left":
		return t.move(-1)
	}
	return false
}

func (t *tabs) name() string {
	if len(t.names) == 0 {
		return ""
	}
	return t.names[t.active]
}

// view draws the strip in w cells, scrolling so the active tab is shown.
func (t *tabs) view(w int, th *Theme) string {
	if len(t.names) == 0 || w <= 0 {
		return ""
	}
	parts := make([]string, len(t.names))
	for i, n := range t.names {
		if i == t.active {
			parts[i] = th.TabActive.Render(n)
		} else {
			parts[i] = th.TabInactive.Render(n)
		}
	}
	return pad(strip(parts, t.active, w, " ", th), w)
}
