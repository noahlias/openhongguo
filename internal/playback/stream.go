package playback

import (
	"context"
	"errors"

	"github.com/beilunyang/openhongguo/internal/media"
	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// Prepared is a stream chosen from a model, with the key that opens it.
// Key is empty for a clear stream, and for a protected one whose key the
// caller withheld or the model does not carry.
type Prepared struct {
	Stream model.Stream
	Key    string
}

// Prepare selects the stream of the wanted quality and derives its content
// key unless noDecrypt is set or the caller supplied one. A key for a clear
// stream is a caller mistake.
func Prepare(pb *model.Playback, episode model.Episode, quality, key string, noDecrypt bool) (Prepared, error) {
	stream, err := pb.Select(quality)
	if err != nil {
		return Prepared{}, invalid("%s", err.Error())
	}
	if pb.Protected && key == "" && !noDecrypt {
		if k, err := sdk.ContentKey(pb); err == nil {
			key = k.Hex
		}
	}
	if key != "" && !pb.Protected {
		return Prepared{}, invalid("episode %d is not encrypted; drop --key", episode.Number)
	}
	return Prepared{Stream: stream, Key: key}, nil
}

// WithBackup runs op on the stream URL and, when that fails for a reason
// other than the viewer, once more on the backup CDN URL. onRetry, when set,
// learns about the first failure.
func WithBackup(ctx context.Context, stream model.Stream, onRetry func(error), op func(url string) (int64, error)) (int64, error) {
	n, err := op(stream.URL)
	if err == nil || stream.BackupURL == "" || stream.BackupURL == stream.URL || ctx.Err() != nil ||
		errors.Is(err, media.ErrStopped) || errors.Is(err, media.ErrNeedsFile) || errors.Is(err, ErrInvalid) {
		return n, err
	}
	if onRetry != nil {
		onRetry(err)
	}
	return op(stream.BackupURL)
}

// SaveStream writes one stream to path, decrypting a protected one.
func SaveStream(ctx context.Context, m Media, url, audio, key string, protected bool, path string, limit int) (int64, error) {
	if protected {
		return m.Decrypt(ctx, url, audio, key, path, limit)
	}
	return m.Save(ctx, url, path)
}
