package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Theme is every style the interface draws with.
type Theme struct {
	name string
	dark bool

	Title     lipgloss.Style
	Subtle    lipgloss.Style
	Accent    lipgloss.Style
	Heat      lipgloss.Style
	Border    lipgloss.Style
	FocusLine lipgloss.Style

	NavItem     lipgloss.Style
	NavSelected lipgloss.Style
	NavFocused  lipgloss.Style

	Row         lipgloss.Style
	RowSelected lipgloss.Style
	RowBlurred  lipgloss.Style

	TabActive   lipgloss.Style
	TabInactive lipgloss.Style
	ChipOn      lipgloss.Style
	ChipOff     lipgloss.Style

	Watched lipgloss.Style
	Current lipgloss.Style
	Locked  lipgloss.Style
	New     lipgloss.Style

	StatusBar lipgloss.Style
	ToastInfo lipgloss.Style
	ToastErr  lipgloss.Style
	Help      lipgloss.Style
	Key       lipgloss.Style
	Label     lipgloss.Style
	Overlay   lipgloss.Style
	Promoted  lipgloss.Style

	Brand      lipgloss.Style
	Panel      lipgloss.Style
	PanelFocus lipgloss.Style
	PanelTitle lipgloss.Style
	Gutter     lipgloss.Style
	RowTitle   lipgloss.Style
	KeyCap     lipgloss.Style
	Pill       lipgloss.Style
	PillHot    lipgloss.Style
	PillNew    lipgloss.Style
	PillDone   lipgloss.Style
	Up         lipgloss.Style
	Down       lipgloss.Style
	// Medals color the first three places of a ranking.
	Medals  [3]lipgloss.Style
	BarFill lipgloss.Style
	BarRest lipgloss.Style
}

func newTheme(name string, dark bool) Theme {
	if name == "dark" {
		dark = true
	}
	if name == "light" {
		dark = false
	}
	t := Theme{name: name, dark: dark}
	if name == "mono" {
		return monoTheme(t)
	}
	red := hex("#E5484D")
	orange := hex("#F76B15")
	var fg, dim, line, sel, selFg, chip color.Color
	if dark {
		fg, dim, line = hex("#E8E8E8"), hex("#8A8A8A"), hex("#3A3A3A")
		sel, selFg, chip = hex("#4A1C1E"), hex("#FFFFFF"), hex("#2A2A2A")
	} else {
		fg, dim, line = hex("#1F1F1F"), hex("#6F6F6F"), hex("#D0D0D0")
		sel, selFg, chip = hex("#FFE0E1"), hex("#1F1F1F"), hex("#EEEEEE")
	}
	base := lipgloss.NewStyle().Foreground(fg)
	t.Title = base.Bold(true).Foreground(red)
	t.Subtle = lipgloss.NewStyle().Foreground(dim)
	t.Accent = base.Foreground(red)
	t.Heat = lipgloss.NewStyle().Foreground(orange)
	t.Border = lipgloss.NewStyle().Foreground(line)
	t.FocusLine = lipgloss.NewStyle().Foreground(red)
	t.NavItem = base
	t.NavSelected = base.Bold(true).Foreground(red)
	t.NavFocused = lipgloss.NewStyle().Bold(true).Foreground(selFg).Background(red)
	t.Row = base
	t.RowSelected = lipgloss.NewStyle().Bold(true).Foreground(selFg).Background(sel)
	t.RowBlurred = lipgloss.NewStyle().Foreground(fg).Background(chip)
	t.TabActive = base.Bold(true).Foreground(red).Underline(true)
	t.TabInactive = t.Subtle
	t.ChipOn = lipgloss.NewStyle().Foreground(selFg).Background(red)
	t.ChipOff = lipgloss.NewStyle().Foreground(fg).Background(chip)
	t.Watched = t.Subtle
	t.Current = lipgloss.NewStyle().Bold(true).Foreground(selFg).Background(red)
	t.Locked = lipgloss.NewStyle().Foreground(dim).Faint(true)
	t.New = lipgloss.NewStyle().Foreground(orange).Bold(true)
	t.StatusBar = lipgloss.NewStyle().Foreground(fg).Background(chip)
	t.ToastInfo = lipgloss.NewStyle().Foreground(selFg).Background(hex("#2F6F4F")).Padding(0, 1)
	t.ToastErr = lipgloss.NewStyle().Foreground(hex("#FFFFFF")).Background(red).Padding(0, 1)
	t.Help = base
	t.Key = base.Bold(true).Foreground(orange)
	t.Label = t.Subtle
	t.Overlay = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(red).Padding(0, 1)
	t.Promoted = t.Subtle.Italic(true)
	white := hex("#FFFFFF")
	green := hex("#30A46C")
	t.Brand = lipgloss.NewStyle().Bold(true).Foreground(white).Background(red).Padding(0, 1)
	t.Panel = t.Border
	t.PanelFocus = lipgloss.NewStyle().Foreground(red)
	t.PanelTitle = base.Bold(true)
	t.Gutter = lipgloss.NewStyle().Foreground(red)
	t.RowTitle = base.Bold(true).Foreground(selFg)
	t.KeyCap = lipgloss.NewStyle().Bold(true).Foreground(fg).Background(chip).Padding(0, 1)
	t.Pill = lipgloss.NewStyle().Foreground(fg).Background(chip).Padding(0, 1)
	t.PillHot = lipgloss.NewStyle().Bold(true).Foreground(white).Background(red).Padding(0, 1)
	t.PillNew = lipgloss.NewStyle().Bold(true).Foreground(white).Background(orange).Padding(0, 1)
	t.PillDone = lipgloss.NewStyle().Foreground(white).Background(green).Padding(0, 1)
	t.Up = lipgloss.NewStyle().Foreground(green)
	t.Down = lipgloss.NewStyle().Foreground(red)
	t.Medals = [3]lipgloss.Style{
		lipgloss.NewStyle().Bold(true).Foreground(hex("#F5B400")),
		lipgloss.NewStyle().Bold(true).Foreground(hex("#A8B3BD")),
		lipgloss.NewStyle().Bold(true).Foreground(hex("#C77B3B")),
	}
	t.BarFill = lipgloss.NewStyle().Foreground(red)
	t.BarRest = t.Border
	t.TabActive = lipgloss.NewStyle().Bold(true).Foreground(white).Background(red).Padding(0, 1)
	t.TabInactive = t.Subtle.Padding(0, 1)
	t.NavSelected = lipgloss.NewStyle().Bold(true).Foreground(red).Background(chip).Padding(0, 1)
	t.NavFocused = lipgloss.NewStyle().Bold(true).Foreground(white).Background(red).Padding(0, 1)
	t.NavItem = t.Subtle.Padding(0, 1)
	return t
}

func monoTheme(t Theme) Theme {
	plain := lipgloss.NewStyle()
	t.Title = plain.Bold(true)
	t.Subtle = plain.Faint(true)
	t.Accent = plain.Bold(true)
	t.Heat = plain
	t.Border = plain.Faint(true)
	t.FocusLine = plain.Bold(true)
	t.NavItem = plain
	t.NavSelected = plain.Bold(true)
	t.NavFocused = plain.Reverse(true)
	t.Row = plain
	t.RowSelected = plain.Reverse(true)
	t.RowBlurred = plain.Underline(true)
	t.TabActive = plain.Bold(true).Underline(true)
	t.TabInactive = plain.Faint(true)
	t.ChipOn = plain.Reverse(true)
	t.ChipOff = plain
	t.Watched = plain.Faint(true)
	t.Current = plain.Reverse(true)
	t.Locked = plain.Faint(true)
	t.New = plain.Bold(true)
	t.StatusBar = plain.Reverse(true)
	t.ToastInfo = plain.Reverse(true).Padding(0, 1)
	t.ToastErr = plain.Reverse(true).Bold(true).Padding(0, 1)
	t.Help = plain
	t.Key = plain.Bold(true)
	t.Label = plain.Faint(true)
	t.Overlay = plain.Border(lipgloss.NormalBorder()).Padding(0, 1)
	t.Promoted = plain.Faint(true)
	t.Brand = plain.Bold(true).Reverse(true).Padding(0, 1)
	t.Panel = plain.Faint(true)
	t.PanelFocus = plain.Bold(true)
	t.PanelTitle = plain.Bold(true)
	t.Gutter = plain.Bold(true)
	t.RowTitle = plain.Bold(true)
	t.KeyCap = plain.Reverse(true).Padding(0, 1)
	t.Pill = plain.Padding(0, 1)
	t.PillHot = plain.Bold(true).Reverse(true).Padding(0, 1)
	t.PillNew = plain.Bold(true).Padding(0, 1)
	t.PillDone = plain.Reverse(true).Padding(0, 1)
	t.Up = plain.Bold(true)
	t.Down = plain
	t.Medals = [3]lipgloss.Style{plain.Bold(true), plain.Bold(true), plain.Bold(true)}
	t.BarFill = plain
	t.BarRest = plain.Faint(true)
	t.TabActive = plain.Bold(true).Reverse(true).Padding(0, 1)
	t.TabInactive = plain.Faint(true).Padding(0, 1)
	t.NavSelected = plain.Bold(true).Underline(true).Padding(0, 1)
	t.NavFocused = plain.Bold(true).Reverse(true).Padding(0, 1)
	t.NavItem = plain.Faint(true).Padding(0, 1)
	return t
}

func hex(s string) color.Color { return lipgloss.Color(s) }
