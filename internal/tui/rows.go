package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/beilunyang/openhongguo/internal/history"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// seriesRow lists a drama or a book from any catalog surface: the title
// and its note on the first line, badges and a blurb on the second.
type seriesRow struct {
	item model.SeriesSummary
	// prefix goes before the title: a rank position or a marker.
	prefix string
	// tag is the service's badge, such as 新剧 or 爆剧.
	tag string
	// note is a short heat or score figure.
	note   string
	change *model.RankChange
	lang   string
	// promoted marks a server-inserted promotion in a ranking.
	promoted bool
	tr       *Strings
}

func (r seriesRow) key() string { return r.item.ID }

func (r seriesRow) render(w int, sel bool, th *Theme) string {
	left := rowTitle(r.item.Title, sel, th)
	indent := ""
	if r.prefix != "" {
		left = rankMark(r.prefix, th) + " " + left
		indent = strings.Repeat(" ", width(r.prefix)+1)
	}
	if r.promoted {
		left += " " + th.Promoted.Render(r.tr.Promoted)
	}
	episodes := ""
	if r.item.EpisodeCount > 0 {
		episodes = th.Subtle.Render(fmt.Sprintf(r.tr.Episodes, r.item.EpisodeCount))
	}
	right := joinNonEmpty(" ", th.Heat.Render(r.note), changeStyled(r.change, r.lang, th))
	badges := []string{tagPill(r.tag, th)}
	if right == "" {
		right = episodes
	} else {
		badges = append(badges, episodes)
	}
	badges = append(badges, th.Subtle.Render(joinNonEmpty(" · ", r.item.Author, r.item.Category)))
	meta := joinNonEmpty("  ", badges...)
	return twoColumns(left, right, w) + secondLine(indent, meta, r.item.Description, w, th)
}

// secondLine is the row's meta line, filled out with the blurb; it is
// empty when there is neither.
func secondLine(indent, meta, blurb string, w int, th *Theme) string {
	blurb = strings.Join(strings.Fields(blurb), " ")
	if meta == "" && blurb == "" {
		return ""
	}
	line := indent + meta
	if blurb != "" {
		if meta != "" {
			line += "  "
		}
		line += th.Subtle.Render(clip(blurb, max(w-width(line), 0)))
	}
	return "\n" + clip(line, w)
}

func rowTitle(title string, sel bool, th *Theme) string {
	if sel {
		return th.RowTitle.Render(title)
	}
	return th.Row.Render(title)
}

// rankMark draws a position, the first three in medal colors.
func rankMark(prefix string, th *Theme) string {
	if n, err := strconv.Atoi(strings.TrimSpace(prefix)); err == nil && n >= 1 && n <= 3 {
		return th.Medals[n-1].Render(prefix)
	}
	return th.Subtle.Render(prefix)
}

// tagPill draws a service badge: new and hot ones stand out.
func tagPill(tag string, th *Theme) string {
	switch {
	case tag == "":
		return ""
	case strings.Contains(tag, "新"):
		return th.PillNew.Render(tag)
	case strings.Contains(tag, "爆") || strings.Contains(tag, "热") || strings.Contains(tag, "首发") || strings.Contains(tag, "独家"):
		return th.PillHot.Render(tag)
	}
	return th.Pill.Render(tag)
}

func changeStyled(c *model.RankChange, lang string, th *Theme) string {
	mark := changeMark(c, lang)
	if c == nil {
		return ""
	}
	switch c.Kind.String() {
	case "up":
		return th.Up.Render(mark)
	case "down":
		return th.Down.Render(mark)
	case "new":
		return th.New.Render(mark)
	}
	return th.Subtle.Render(mark)
}

// meter is a progress line of w cells.
func meter(done, total, w int, th *Theme) string {
	if total <= 0 || w < 2 {
		return ""
	}
	fill := min(max(done*w/total, 0), w)
	return th.BarFill.Render(strings.Repeat("━", fill)) + th.BarRest.Render(strings.Repeat("─", w-fill))
}

// historyRow lists one watched series with how far it got.
type historyRow struct {
	entry history.Entry
	tr    *Strings
}

func (r historyRow) key() string { return r.entry.SeriesID }

func (r historyRow) render(w int, sel bool, th *Theme) string {
	title := r.entry.Title
	if title == "" {
		title = r.entry.SeriesID
	}
	var right string
	switch {
	case r.entry.Finished && r.entry.Episodes > 0 && r.entry.Episode >= r.entry.Episodes:
		right = th.PillDone.Render(r.tr.Finished)
	case r.entry.Finished:
		right = fmt.Sprintf(r.tr.EpisodeN, r.entry.Episode) + " ✓"
	default:
		right = th.Accent.Render(fmt.Sprintf(r.tr.EpisodeN, r.entry.Episode))
		if r.entry.Position > 0 {
			right += th.Subtle.Render(" " + formatSeconds(r.entry.Position))
		}
	}
	line := twoColumns(rowTitle(title, sel, th), right, w)
	if r.entry.Episodes <= 0 {
		return line
	}
	count := fmt.Sprintf(" %d/%d", r.entry.Episode, r.entry.Episodes)
	return line + "\n" + meter(r.entry.Episode, r.entry.Episodes, min(24, max(w-width(count), 2)), th) + th.Subtle.Render(count)
}

// textRow is a heading or a plain line inside a list.
type textRow struct {
	id    string
	text  string
	style func(*Theme) string
}

func (r textRow) key() string { return r.id }

func (r textRow) render(w int, _ bool, th *Theme) string {
	if r.style != nil {
		return clip(r.style(th), w)
	}
	return clip(th.Subtle.Render(r.text), w)
}

// heading is a section title inside a list.
func heading(id, text string) textRow {
	return textRow{id: "h:" + id, text: text, style: func(th *Theme) string {
		return th.Accent.Render("━━ ") + th.PanelTitle.Render(text)
	}}
}

// personRow lists an actor from a celebrity board.
type personRow struct {
	item model.RankItem
	tr   *Strings
	lang string
}

func (r personRow) key() string { return "person:" + r.item.UserID + ":" + itoa(r.item.Position) }

func (r personRow) render(w int, sel bool, th *Theme) string {
	pos := padLeft(itoa(r.item.Position), 3)
	left := rankMark(pos, th) + " " + rowTitle(r.item.Actor, sel, th)
	right := joinNonEmpty(" ", th.Heat.Render(r.item.Score), changeStyled(r.item.Change, r.lang, th))
	return twoColumns(left, right, w) + secondLine("    ", "", r.item.Intro, w, th)
}

// workRow lists an actor's work.
type workRow struct {
	item model.Work
	tr   *Strings
	lang string
}

func (r workRow) key() string { return r.item.SeriesID }

func (r workRow) render(w int, sel bool, th *Theme) string {
	right := th.Heat.Render(formatCount(r.item.Heat, r.lang))
	meta := th.Subtle.Render(fmt.Sprintf(r.tr.Episodes, r.item.EpisodeCount))
	return twoColumns(rowTitle(r.item.Title, sel, th), right, w) + secondLine("", meta, "", w, th)
}

// releaseRow lists one show on the release calendar.
type releaseRow struct {
	item model.NewRelease
	tr   *Strings
}

func (r releaseRow) key() string { return r.item.SeriesID }

func (r releaseRow) render(w int, sel bool, th *Theme) string {
	when := "     "
	if !r.item.PublishAt.IsZero() {
		when = r.item.PublishAt.Format("15:04")
	}
	state := th.Pill.Render(r.tr.Upcoming)
	if r.item.Online {
		state = th.PillHot.Render(r.tr.Online)
	}
	left := th.Accent.Render(when) + " " + rowTitle(r.item.Title, sel, th)
	return twoColumns(left, state, w) + secondLine("      ", th.Heat.Render(r.item.Heat), "", w, th)
}
