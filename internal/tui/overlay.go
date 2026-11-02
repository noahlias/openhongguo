package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// overlay is a box drawn over the screen that takes every key until it
// closes.
type overlay interface {
	Update(msg tea.Msg) (done bool, cmd tea.Cmd)
	View(w, h int, th *Theme) string
}

// helpOverlay lists the key bindings.
type helpOverlay struct {
	tr     *Strings
	global []binding
	screen []binding
	title  string
}

func (h *helpOverlay) Update(msg tea.Msg) (bool, tea.Cmd) {
	if _, ok := msg.(tea.KeyPressMsg); ok {
		return true, nil
	}
	return false, nil
}

func (h *helpOverlay) View(w, _ int, th *Theme) string {
	inner := min(w-6, 70)
	if inner < 20 {
		inner = w - 4
	}
	var b strings.Builder
	b.WriteString(th.Title.Render(h.tr.HelpTitle))
	b.WriteString("\n\n")
	b.WriteString(th.Accent.Render(h.tr.HelpGlobal))
	b.WriteString("\n")
	writeBindings(&b, h.global, inner, th)
	if len(h.screen) > 0 {
		b.WriteString("\n")
		b.WriteString(th.Accent.Render(h.tr.HelpScreen + " · " + h.title))
		b.WriteString("\n")
		writeBindings(&b, h.screen, inner, th)
	}
	return th.Overlay.Width(inner).Render(b.String())
}

func writeBindings(b *strings.Builder, items []binding, w int, th *Theme) {
	kw := 0
	for _, it := range items {
		if n := width(it.keys); n > kw {
			kw = n
		}
	}
	for _, it := range items {
		b.WriteString(th.Key.Render(pad(it.keys, kw)))
		b.WriteString("  ")
		b.WriteString(clip(it.help, w-kw-2))
		b.WriteString("\n")
	}
}

// promptOverlay asks for one line of text and answers with promptDoneMsg.
type promptOverlay struct {
	id    string
	label string
	input textinput.Model
}

func newPrompt(id, label, initial string, th *Theme) *promptOverlay {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.SetValue(initial)
	ti.Focus()
	return &promptOverlay{id: id, label: label, input: ti}
}

func (p *promptOverlay) Update(msg tea.Msg) (bool, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch keyString(k) {
		case "esc", "ctrl+c":
			return true, emit(promptDoneMsg{id: p.id})
		case "enter":
			return true, emit(promptDoneMsg{id: p.id, value: strings.TrimSpace(p.input.Value()), ok: true})
		}
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return false, cmd
}

func (p *promptOverlay) View(w, _ int, th *Theme) string {
	inner := min(w-6, 60)
	if inner < 20 {
		inner = w - 4
	}
	p.input.SetWidth(inner - 2)
	body := th.Accent.Render(clip(p.label, inner)) + "\n" + p.input.View()
	return th.Overlay.Width(inner).Render(body)
}
