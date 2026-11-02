package playback

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// DownloadJob says which episodes of a series to save, and where.
type DownloadJob struct {
	// Episodes is the full list; Numbers picks the ones to save.
	Episodes []model.Episode
	Numbers  []int
	Dir      string
	// Jobs is how many episodes download at once.
	Jobs    int
	Quality string
	// OnRetry, when set, learns about a CDN URL that failed before the
	// backup URL is tried.
	OnRetry func(model.Episode, error)
}

// DownloadEvent reports one episode: saved (Written), already present
// (Skipped), or failed (Err).
type DownloadEvent struct {
	Episode model.Episode
	Path    string
	Written int64
	Skipped bool
	Err     error
}

// Download saves the job's episodes, job.Jobs at a time, and reports each
// one through onEvent, which is never called concurrently. It returns how
// many failed, and ctx.Err() when cancelled.
func Download(ctx context.Context, d Deps, job DownloadJob, onEvent func(DownloadEvent)) (failed int, err error) {
	if onEvent == nil {
		onEvent = func(DownloadEvent) {}
	}
	if err := os.MkdirAll(job.Dir, 0o755); err != nil {
		return 0, err
	}
	workers := job.Jobs
	if workers < 1 {
		workers = 1
	}
	total := len(job.Episodes)
	jobs := make(chan model.Episode)
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for episode := range jobs {
				path := EpisodePath(job.Dir, episode.Number, total)
				var onRetry func(error)
				if job.OnRetry != nil {
					onRetry = func(err error) { job.OnRetry(episode, err) }
				}
				written, skipped, err := DownloadEpisode(ctx, d, episode, path, job.Quality, onRetry)
				mu.Lock()
				if err != nil && ctx.Err() == nil {
					failed++
				}
				if err == nil || ctx.Err() == nil {
					onEvent(DownloadEvent{Episode: episode, Path: path, Written: written, Skipped: skipped, Err: err})
				}
				mu.Unlock()
			}
		}()
	}
feed:
	for _, n := range job.Numbers {
		episode, ok := FindEpisode(job.Episodes, n)
		if !ok {
			continue
		}
		select {
		case jobs <- episode:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()
	return failed, ctx.Err()
}

// DownloadEpisode saves one episode to path through a .part file, so path
// only ever holds a complete episode. An existing path is skipped.
func DownloadEpisode(ctx context.Context, d Deps, episode model.Episode, path, quality string, onRetry func(error)) (written int64, skipped bool, err error) {
	if _, err := os.Stat(path); err == nil {
		return 0, true, nil
	}
	pb, err := Resolve(ctx, d, episode.ID)
	if err != nil {
		return 0, false, err
	}
	stream, err := pb.Select(quality)
	if err != nil {
		return 0, false, invalid("%s", err.Error())
	}
	key := ""
	if pb.Protected {
		k, err := sdk.ContentKey(pb)
		if err != nil {
			return 0, false, DRMError(pb)
		}
		key = k.Hex
	}
	part := path + ".part"
	// A .part file is what an interrupted run left behind.
	_ = os.Remove(part)
	written, err = WithBackup(ctx, stream, onRetry, func(url string) (int64, error) {
		return SaveStream(ctx, d.Media, url, pb.AudioURL, key, pb.Protected, part, 0)
	})
	if err != nil {
		_ = os.Remove(part)
		return 0, false, err
	}
	if err := os.Rename(part, path); err != nil {
		return 0, false, err
	}
	return written, false, nil
}

// EpisodePath names episode number of a series with total episodes:
// <dir>/第NNN集.mp4, padded to at least three digits.
func EpisodePath(dir string, number, total int) string {
	width := len(strconv.Itoa(total))
	if width < 3 {
		width = 3
	}
	return filepath.Join(dir, fmt.Sprintf("第%0*d集.mp4", width, number))
}

// ParseEpisodeRange reads "1-10,12" into sorted, distinct episode numbers.
// An empty spec means every episode.
func ParseEpisodeRange(spec string, total int) ([]int, error) {
	if strings.TrimSpace(spec) == "" {
		all := make([]int, total)
		for i := range all {
			all[i] = i + 1
		}
		return all, nil
	}
	seen := map[int]bool{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		lo, hi, isRange := strings.Cut(part, "-")
		from, err1 := strconv.Atoi(strings.TrimSpace(lo))
		to := from
		var err2 error
		if isRange {
			to, err2 = strconv.Atoi(strings.TrimSpace(hi))
		}
		if err1 != nil || err2 != nil || from < 1 || to < from {
			return nil, invalid("episodes must look like 1-10,12, got %q", part)
		}
		if to > total {
			return nil, invalid("the series has %d episodes, got %q", total, part)
		}
		for n := from; n <= to; n++ {
			seen[n] = true
		}
	}
	out := make([]int, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Ints(out)
	return out, nil
}

// SafeFileName drops characters that are not allowed in a file name on
// common file systems.
func SafeFileName(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		if r < 0x20 {
			return -1
		}
		return r
	}, s)
	return strings.Trim(strings.TrimSpace(s), ".")
}

// DefaultDir is where a series downloads to when the caller names no
// directory: the title, or the ID when the title leaves nothing usable.
func DefaultDir(title, id string) string {
	if dir := SafeFileName(title); dir != "" {
		return dir
	}
	return id
}
