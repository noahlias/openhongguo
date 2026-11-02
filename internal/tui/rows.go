package tui

import (
	"fmt"

	"github.com/beilunyang/openhongguo/internal/history"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// seriesRow lists a drama or a book from any catalog surface.
type seriesRow struct {
	item model.SeriesSummary
	// extra is a short right-aligned note: heat, score, rank change.
	extra string
	// prefix goes before the title: a rank position or a date.
	prefix string
	// promoted marks a server-inserted promotion in a ranking.
	promoted bool
	tr       *Strings
}

func (r seriesRow) key() string { return r.item.ID }

func (r seriesRow) render(w int, _ bool, th *Theme) string {
	left := r.item.Title
	if r.prefix != "" {
		left = th.Subtle.Render(r.prefix) + " " + left
	}
	if r.promoted {
		left += " " + th.Promoted.Render(r.tr.Promoted)
	}
	right := r.extra
	if right == "" && r.item.EpisodeCount > 0 {
		right = fmt.Sprintf(r.tr.Episodes, r.item.EpisodeCount)
	}
	if right == "" && r.item.Author != "" {
		right = r.item.Author
	}
	return twoColumns(left, th.Subtle.Render(right), w)
}

// historyRow lists one watched series.
type historyRow struct {
	entry history.Entry
	tr    *Strings
}

func (r historyRow) key() string { return r.entry.SeriesID }

func (r historyRow) render(w int, _ bool, th *Theme) string {
	title := r.entry.Title
	if title == "" {
		title = r.entry.SeriesID
	}
	var right string
	switch {
	case r.entry.Finished && r.entry.Episodes > 0 && r.entry.Episode >= r.entry.Episodes:
		right = r.tr.Finished
	case r.entry.Finished:
		right = fmt.Sprintf(r.tr.EpisodeN, r.entry.Episode) + " ✓"
	default:
		right = fmt.Sprintf(r.tr.EpisodeN, r.entry.Episode)
		if r.entry.Position > 0 {
			right += " " + formatSeconds(r.entry.Position)
		}
	}
	if r.entry.Episodes > 0 {
		right += th.Subtle.Render(fmt.Sprintf(" /%d", r.entry.Episodes))
	}
	return twoColumns(title, right, w)
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

// personRow lists an actor from a celebrity board.
type personRow struct {
	item model.RankItem
	tr   *Strings
	lang string
}

func (r personRow) key() string { return "person:" + r.item.UserID + ":" + itoa(r.item.Position) }

func (r personRow) render(w int, _ bool, th *Theme) string {
	left := th.Subtle.Render(padLeft(itoa(r.item.Position), 3)) + " " + r.item.Actor
	if r.item.Intro != "" {
		left += " " + th.Subtle.Render(r.item.Intro)
	}
	right := joinNonEmpty(" ", r.item.Score, th.Accent.Render(changeMark(r.item.Change, r.lang)))
	return twoColumns(left, right, w)
}

// workRow lists an actor's work.
type workRow struct {
	item model.Work
	tr   *Strings
	lang string
}

func (r workRow) key() string { return r.item.SeriesID }

func (r workRow) render(w int, _ bool, th *Theme) string {
	right := joinNonEmpty(" · ", th.Heat.Render(formatCount(r.item.Heat, r.lang)), th.Subtle.Render(fmt.Sprintf(r.tr.Episodes, r.item.EpisodeCount)))
	return twoColumns(r.item.Title, right, w)
}

// episodeLine lists one episode in a plain list (used by the detail pane
// when the grid does not fit).
type releaseRow struct {
	item model.NewRelease
	tr   *Strings
}

func (r releaseRow) key() string { return r.item.SeriesID }

func (r releaseRow) render(w int, _ bool, th *Theme) string {
	when := ""
	if !r.item.PublishAt.IsZero() {
		when = r.item.PublishAt.Format("15:04")
	}
	state := th.Subtle.Render(r.tr.Upcoming)
	if r.item.Online {
		state = th.Accent.Render(r.tr.Online)
	}
	left := joinNonEmpty(" ", th.Subtle.Render(when), r.item.Title)
	right := joinNonEmpty(" ", th.Heat.Render(r.item.Heat), state)
	return twoColumns(left, right, w)
}
