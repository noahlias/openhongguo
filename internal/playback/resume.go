package playback

import "context"

// ResumeBackoff rewinds a resumed episode a little so the viewer sees where
// they left off.
const ResumeBackoff = 3

// ResumePoint picks where to resume: inside the episode left unfinished, or
// at the start of the one after the last finished episode. A series never
// played starts at episode 1.
func ResumePoint(ctx context.Context, h History, seriesID string, total int) (episode, start int, err error) {
	e, ok, err := h.Get(ctx, seriesID)
	if err != nil {
		return 0, 0, err
	}
	if !ok {
		return 1, 0, nil
	}
	if !e.Finished {
		start = int(e.Position) - ResumeBackoff
		if start < 0 {
			start = 0
		}
		return e.Episode, start, nil
	}
	if e.Episode >= total {
		return 0, 0, invalid("all %d episodes of %s were watched; pass --episode to replay one", total, seriesID)
	}
	return e.Episode + 1, 0, nil
}
