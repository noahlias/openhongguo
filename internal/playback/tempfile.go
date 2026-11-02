package playback

import (
	"context"
	"os"

	"github.com/beilunyang/openhongguo/sdk/model"
)

// playFromTempFile decrypts into a temp file for a player that can only
// open a file, and removes the file once the player exits.
func playFromTempFile(ctx context.Context, d Deps, stream model.Stream, audio, key, player string, preview int, onRetry func(error)) error {
	tmp, err := os.CreateTemp("", "ohg-*.mp4")
	if err != nil {
		return err
	}
	target := tmp.Name()
	_ = tmp.Close()
	// Decrypt never overwrites, so drop the probe file first.
	if err := os.Remove(target); err != nil {
		return err
	}
	if _, err := WithBackup(ctx, stream, onRetry, func(url string) (int64, error) {
		return d.Media.Decrypt(ctx, url, audio, key, target, preview)
	}); err != nil {
		return err
	}
	defer os.Remove(target)
	return d.Media.Open(ctx, target, player)
}
