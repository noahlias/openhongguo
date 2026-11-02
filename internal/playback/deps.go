// Package playback plays, downloads, and remembers episodes. It is the
// engine shared by the command line and the full-screen interface: it never
// prints, it reports what happens through events.
package playback

import (
	"context"
	"errors"
	"fmt"

	"github.com/beilunyang/openhongguo/internal/history"
	"github.com/beilunyang/openhongguo/internal/media"
	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// Service is the part of the catalog the engine reads.
type Service interface {
	ListEpisodes(context.Context, string, sdk.EpisodeOptions) (*sdk.Page[model.Episode], error)
	ResolvePlayback(context.Context, string, sdk.PlaybackOptions) (*model.Playback, error)
	GetSeries(context.Context, string) (*model.Series, error)
}

// Media plays a resolved stream with an external player, saves it, or
// rewrites a protected stream into a clear file with a content key.
type Media interface {
	Open(ctx context.Context, url, player string) error
	Save(ctx context.Context, url, path string) (int64, error)
	Decrypt(ctx context.Context, url, audio, key, path string, limitSeconds int) (int64, error)
	Play(ctx context.Context, req media.Request) (media.Result, error)
}

// History remembers where each series was left.
type History interface {
	Get(ctx context.Context, seriesID string) (history.Entry, bool, error)
	Record(ctx context.Context, e history.Entry) error
}

// DeviceResetter registers a new anonymous device.
type DeviceResetter interface {
	ResetDevice(ctx context.Context) error
}

// Deps are the collaborators of the engine. History may be nil; Media may be
// nil for callers that only resolve.
type Deps struct {
	Service Service
	Media   Media
	History History
}

// ErrInvalid marks a caller mistake: an episode that does not exist, a
// quality the service does not offer, a key for a clear stream. The text
// matches the command line's usage errors so they print the same.
var ErrInvalid = errors.New("invalid arguments")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}
