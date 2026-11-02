package tui

import (
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/beilunyang/openhongguo/internal/playback"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// resumeFromHistory picks the episode and position to continue a series
// and returns the play message, or a toast when there is nothing to play.
func resumeFromHistory(e *env, id, title string, episodes []model.Episode) tea.Msg {
	if len(episodes) == 0 {
		return toastMsg{text: e.tr.Empty, err: true}
	}
	n, start := 1, 0
	if e.deps.History != nil {
		var err error
		n, start, err = playback.ResumePoint(e.ctx, e.deps.History, id, len(episodes))
		if errors.Is(err, playback.ErrInvalid) {
			// Everything watched: start over from the first episode.
			n, start = 1, 0
		} else if err != nil {
			return toastMsg{text: describeError(err, e.tr), err: true}
		}
	}
	ep, ok := playback.FindEpisode(episodes, n)
	if !ok {
		ep = episodes[0]
	}
	return playMsg{seriesID: id, title: title, episodes: episodes, start: ep, startSeconds: start, autoplay: e.settings.Autoplay}
}
