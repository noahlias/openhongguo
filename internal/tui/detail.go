package tui

import (
	"fmt"
	"strings"

	"github.com/beilunyang/openhongguo/sdk/model"
)

// renderSeriesDetail draws the right pane for a series: the summary from
// the list row at once, and the full record once it is cached.
func renderSeriesDetail(e *env, w, h int, sum model.SeriesSummary) string {
	if w < 8 || h < 2 {
		return ""
	}
	full := e.detail(sum.ID)
	lines := seriesHero(e, w, h, sum, full, nil)
	lines = append(lines, seriesNotes(e, w, sum, full)...)
	return strings.Join(lines[:min(len(lines), h)], "\n")
}

// seriesHero is the headline and extra, beside the cover once it is loaded
// and the width allows, else under it.
func seriesHero(e *env, w, h int, sum model.SeriesSummary, full *model.Series, extra []string) []string {
	var cover []string
	if e.coversOn() && full != nil && h >= 16 && w >= coverCols {
		cover = e.covers.draw(full.CoverURL)
	}
	side := cover != nil && w >= coverCols+2+18
	infoW := w
	if side {
		infoW = w - coverCols - 2
	}
	info := seriesHeadline(e, infoW, sum, full, side)
	for _, l := range extra {
		info = append(info, clip(l, infoW))
	}
	if !side {
		if cover == nil {
			return info
		}
		return append(append(cover, ""), info...)
	}
	lines := make([]string, 0, max(len(cover), len(info)))
	for i := 0; i < max(len(cover), len(info)); i++ {
		left := strings.Repeat(" ", coverCols)
		if i < len(cover) {
			left = cover[i]
		}
		right := ""
		if i < len(info) {
			right = info[i]
		}
		lines = append(lines, left+"  "+right)
	}
	return lines
}

// seriesNotes are the labelled facts and the synopsis.
func seriesNotes(e *env, w int, sum model.SeriesSummary, full *model.Series) []string {
	var lines []string
	if facts := seriesFacts(e, w, full); len(facts) > 0 {
		lines = append(lines, "")
		lines = append(lines, facts...)
	}
	desc := sum.Description
	if full != nil && full.Description != "" {
		desc = full.Description
	}
	if desc = strings.TrimSpace(desc); desc != "" {
		lines = append(lines, "")
		lines = append(lines, wrap(desc, w)...)
	}
	return lines
}

// seriesHeadline is the title, badges, counts and progress. Stacked beside
// a cover, the counts take a line each.
func seriesHeadline(e *env, w int, sum model.SeriesSummary, full *model.Series, stacked bool) []string {
	tr, th, lang := e.tr, e.theme, e.lang()
	var lines []string
	add := func(s string) {
		if s != "" {
			lines = append(lines, clip(s, w))
		}
	}
	title := wrap(sum.Title, w)
	for _, l := range title[:min(len(title), 2)] {
		add(th.Title.Render(l))
	}
	if full == nil {
		if sum.EpisodeCount > 0 {
			add(th.Pill.Render(fmt.Sprintf(tr.Episodes, sum.EpisodeCount)))
		}
		add(labelled(tr.Author, sum.Author, th))
		add(labelled(tr.Category, sum.Category, th))
		if sum.Words > 0 {
			add(labelled(tr.Words, formatCount(sum.Words, lang), th))
		}
		return append(lines, watchedLine(e, sum.ID)...)
	}
	status := statusLabel(full.Status, lang)
	statusPill := th.PillNew.Render(status)
	if full.Status == 1 {
		statusPill = th.PillDone.Render(status)
	}
	if status == "" {
		statusPill = ""
	}
	contentPill := ""
	if c := contentLabel(full.ContentType, lang); c != "" {
		contentPill = th.Pill.Render(c)
	}
	add("")
	episodes := firstNonZero(full.EpisodeCount, sum.EpisodeCount)
	add(joinNonEmpty(" ", statusPill, contentPill, th.Pill.Render(fmt.Sprintf(tr.Episodes, episodes))))
	if full.RightText != "" && !strings.Contains(full.RightText, itoa(episodes)) {
		add(th.Accent.Render(full.RightText))
	}
	counts := []string{
		labelled(tr.Heat, th.Heat.Bold(true).Render(formatCount(full.Heat, lang)), th),
		labelled(tr.Plays, formatCount(full.PlayCount, lang), th),
		labelled(tr.Follows, formatCount(full.FollowCount, lang), th),
		labelled(tr.Likes, formatCount(full.DiggCount, lang), th),
	}
	add("")
	if stacked {
		for _, c := range counts {
			add(c)
		}
	} else {
		add(joinNonEmpty("  ", counts...))
	}
	if w := watchedLine(e, sum.ID); len(w) > 0 {
		add("")
		lines = append(lines, w...)
	}
	return lines
}

func watchedLine(e *env, id string) []string {
	tr, th := e.tr, e.theme
	hist, ok := e.watched(id)
	if !ok {
		return nil
	}
	where := fmt.Sprintf(tr.EpisodeN, hist.Episode)
	if hist.Finished {
		where += " ✓"
	} else if hist.Position > 0 {
		where += " " + formatSeconds(hist.Position)
	}
	return []string{labelled(tr.LastWatched, th.Accent.Render(where), th)}
}

// seriesFacts are the record's labelled details under the headline.
func seriesFacts(e *env, w int, full *model.Series) []string {
	if full == nil {
		return nil
	}
	tr, th, lang := e.tr, e.theme, e.lang()
	var lines []string
	add := func(s string) {
		if s != "" {
			lines = append(lines, clip(s, w))
		}
	}
	for i := 0; i < len(full.Rows); {
		kind := full.Rows[i].Kind
		var texts []string
		for ; i < len(full.Rows) && full.Rows[i].Kind == kind; i++ {
			if t := full.Rows[i].Text; t != "" {
				texts = append(texts, t)
			}
		}
		if len(texts) > 0 {
			add(labelled(infoKindLabel(kind, lang), strings.Join(texts, " · "), th))
		}
	}
	if full.PayType != 0 {
		add(labelled(tr.Pay, payLabel(full.PayType, lang), th))
	}
	if !full.CreatedAt.IsZero() {
		add(labelled(tr.Released, formatDate(full.CreatedAt), th))
	}
	for _, s := range full.Subtitles {
		if s != full.RightText {
			add(th.Subtle.Render(s))
		}
	}
	if full.Novel != nil {
		add(labelled(tr.Novel, joinNonEmpty(" · ", full.Novel.Title, novelStatusLabel(full.Novel.Status, lang)), th))
	}
	if full.Platform == model.PlatformUnknown {
		add(th.Subtle.Render(tr.ToastBook))
	}
	return lines
}

func labelled(label, value string, th *Theme) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return th.Label.Render(label+" ") + value
}

func firstNonZero(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}
