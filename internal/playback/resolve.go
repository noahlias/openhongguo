package playback

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// Resolve loads an episode's model; the SDK client moves to a fresh
// anonymous device on its own when the service answers with an empty one. A
// protected model is returned with a nil error; Playback.Protected tells it
// apart.
func Resolve(ctx context.Context, d Deps, episodeID string) (*model.Playback, error) {
	pb, err := d.Service.ResolvePlayback(ctx, episodeID, sdk.PlaybackOptions{})
	if errors.Is(err, sdk.ErrDRMProtected) && pb != nil {
		return pb, nil
	}
	if err == nil && pb == nil {
		return nil, errors.New("empty playback model")
	}
	return pb, err
}

// Expired reports a model whose URLs expire within a minute.
func Expired(pb *model.Playback) bool {
	return pb.ExpiresAt > 0 && time.Unix(pb.ExpiresAt, 0).Before(time.Now().Add(time.Minute))
}

// Unavailable reports an episode the anonymous surface will not play.
func Unavailable(err error) bool {
	return errors.Is(err, sdk.ErrAnonymousLimited) || errors.Is(err, sdk.ErrAuthenticationRequired) || errors.Is(err, sdk.ErrDRMProtected)
}

// FindEpisode returns the episode with the given number.
func FindEpisode(items []model.Episode, number int) (model.Episode, bool) {
	for _, e := range items {
		if e.Number == number {
			return e, true
		}
	}
	return model.Episode{}, false
}

// RequireVideo refuses a title with no video platform: books and audiobooks
// share the series detail endpoint and list their chapters as episodes, but
// their playback model answers 101000 (observed 2026-10-24 on 25 videos with
// platform 3 or 4 and 7 books and audiobooks with platform 0).
func RequireVideo(series *model.Series) error {
	if series.Platform != model.PlatformUnknown {
		return nil
	}
	return invalid("%s (%s) is a book or audiobook, not a video; play and download take short dramas and films", series.Title, series.ID)
}

// DRMError names the protection scheme so the caller can tell a protected
// stream from a missing one. It never includes keys or per-segment data.
func DRMError(p *model.Playback) error {
	scheme := ""
	for _, s := range p.Streams {
		if s.Encryption != "" {
			scheme = s.Encryption
			break
		}
	}
	if scheme == "" {
		return fmt.Errorf("%w: pass --key HEX, set OHG_CENC_KEY, or leave decryption enabled", sdk.ErrDRMProtected)
	}
	return fmt.Errorf("%w (%s): pass --key HEX, set OHG_CENC_KEY, or leave decryption enabled", sdk.ErrDRMProtected, scheme)
}
