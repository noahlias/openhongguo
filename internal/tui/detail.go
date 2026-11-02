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
	tr, th, lang := e.tr, e.theme, e.lang()
	full := e.detail(sum.ID)
	var lines []string
	add := func(s string) { lines = append(lines, s) }
	if e.coversOn() && full != nil && w >= coverCols+2 && h >= 24 {
		if cover := e.covers.draw(full.CoverURL); cover != nil {
			lines = append(lines, cover...)
			add("")
		}
	}
	add(th.Title.Render(clip(sum.Title, w)))
	if full != nil {
		kind := joinNonEmpty(" · ",
			contentLabel(full.ContentType, lang),
			statusLabel(full.Status, lang),
			fmt.Sprintf(tr.Episodes, firstNonZero(full.EpisodeCount, sum.EpisodeCount)),
		)
		add(clip(th.Subtle.Render(kind), w))
		if full.RightText != "" {
			add(clip(th.Accent.Render(full.RightText), w))
		}
		counts := joinNonEmpty("  ",
			labelled(tr.Heat, th.Heat.Render(formatCount(full.Heat, lang)), th),
			labelled(tr.Plays, formatCount(full.PlayCount, lang), th),
			labelled(tr.Follows, formatCount(full.FollowCount, lang), th),
			labelled(tr.Likes, formatCount(full.DiggCount, lang), th),
		)
		if counts != "" {
			add(clip(counts, w))
		}
		if full.PayType != 0 {
			add(clip(labelled(tr.Pay, payLabel(full.PayType, lang), th), w))
		}
		if !full.CreatedAt.IsZero() {
			add(clip(labelled(tr.Released, formatDate(full.CreatedAt), th), w))
		}
		if full.Platform == model.PlatformUnknown {
			add(clip(th.Subtle.Render(tr.ToastBook), w))
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
				add(clip(labelled(infoKindLabel(kind, lang), strings.Join(texts, " · "), th), w))
			}
		}
		for _, s := range full.Subtitles {
			if s != full.RightText {
				add(clip(th.Subtle.Render(s), w))
			}
		}
		if full.Novel != nil {
			add(clip(labelled(tr.Novel, joinNonEmpty(" · ", full.Novel.Title, novelStatusLabel(full.Novel.Status, lang)), th), w))
		}
	} else {
		if sum.EpisodeCount > 0 {
			add(th.Subtle.Render(fmt.Sprintf(tr.Episodes, sum.EpisodeCount)))
		}
		if sum.Author != "" {
			add(clip(labelled(tr.Author, sum.Author, th), w))
		}
		if sum.Category != "" {
			add(clip(labelled(tr.Category, sum.Category, th), w))
		}
		if sum.Words > 0 {
			add(clip(labelled(tr.Words, formatCount(sum.Words, lang), th), w))
		}
	}
	if hist, ok := e.watched(sum.ID); ok {
		where := fmt.Sprintf(tr.EpisodeN, hist.Episode)
		if hist.Finished {
			where += " ✓"
		} else if hist.Position > 0 {
			where += " " + formatSeconds(hist.Position)
		}
		add(clip(labelled(tr.LastWatched, th.Accent.Render(where), th), w))
	}
	desc := sum.Description
	if full != nil && full.Description != "" {
		desc = full.Description
	}
	if desc != "" {
		add("")
		for _, l := range wrap(strings.TrimSpace(desc), w) {
			add(l)
		}
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	return strings.Join(lines, "\n")
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
