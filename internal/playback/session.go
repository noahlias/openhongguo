package playback

import (
	"context"
	"errors"
	"fmt"

	"github.com/beilunyang/openhongguo/internal/history"
	"github.com/beilunyang/openhongguo/internal/media"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// MaxUnavailable is how many episodes in a row autoplay skips before it
// gives up: one locked episode is skipped, a locked series is not walked.
const MaxUnavailable = 3

// Options say how a session plays.
type Options struct {
	Quality string
	Player  string
	// Key is a caller-supplied content key for the first episode only.
	Key       string
	Autoplay  bool
	Preview   int
	NoDecrypt bool
	// NoTerminal keeps the player away from the terminal, for a caller that
	// owns the screen.
	NoTerminal bool
}

// Target is what a session plays: the series, its episodes, and where to
// start. Remember turns history recording on.
type Target struct {
	SeriesID     string
	Title        string
	Episodes     []model.Episode
	Start        model.Episode
	StartSeconds int
	Remember     bool
}

// EventKind tells what happened in a session.
type EventKind int

const (
	// EventStarted: Episode is about to play; Total is the episode count.
	EventStarted EventKind = iota
	// EventFinished: Episode played to the end.
	EventFinished
	// EventStopped: the viewer closed the player at Position.
	EventStopped
	// EventJumped: the viewer asked for another episode at Position.
	EventJumped
	// EventSkipped: Episode is unavailable (Err); autoplay moves on.
	EventSkipped
	// EventBackupRetry: the first CDN URL failed (Err); trying the backup.
	EventBackupRetry
	// EventHistoryWarning: the watch position was not saved (Err).
	EventHistoryWarning
)

// Event is one thing that happened in a session.
type Event struct {
	Kind     EventKind
	Episode  model.Episode
	Total    int
	Position float64
	Err      error
}

// Session plays one or more episodes of a series.
type Session struct {
	d       Deps
	opts    Options
	onEvent func(Event)
	target  Target
	// prefetch resolves the episode that will most likely play next while
	// the current one plays.
	prefetch struct {
		number int
		ch     <-chan resolved
	}
}

type resolved struct {
	playback *model.Playback
	err      error
}

// NewSession makes a session. onEvent may be nil.
func NewSession(d Deps, opts Options, onEvent func(Event)) *Session {
	if onEvent == nil {
		onEvent = func(Event) {}
	}
	return &Session{d: d, opts: opts, onEvent: onEvent}
}

// Run plays t.Start and, with Autoplay, the episodes after it. It returns
// nil when the viewer stops, and the episode's error otherwise.
func (s *Session) Run(ctx context.Context, t Target) error {
	s.target = t
	items, episode, start := t.Episodes, t.Start, t.StartSeconds
	total := len(items)
	key := s.opts.Key
	unavailable := 0
	for {
		s.onEvent(Event{Kind: EventStarted, Episode: episode, Total: total})
		s.record(ctx, history.Entry{Episode: episode.Number, Position: float64(start)})
		pending := s.resolve(ctx, episode)
		if next, ok := FindEpisode(items, episode.Number+1); ok && s.opts.Autoplay {
			s.startPrefetch(ctx, next)
		}
		res, err := s.play(ctx, episode, pending, key, start)
		start = 0
		key = "" // a caller key belongs to the first episode only
		switch {
		case res.Jump != 0:
			s.onEvent(Event{Kind: EventJumped, Episode: episode, Position: res.Position})
			s.record(ctx, history.Entry{Episode: episode.Number, Position: res.Position})
			target, ok := FindEpisode(items, episode.Number+res.Jump)
			if !ok {
				return nil
			}
			episode = target
			continue
		case errors.Is(err, media.ErrStopped) || errors.Is(err, context.Canceled):
			// Closing the player or pressing Ctrl-C ends playback quietly.
			s.onEvent(Event{Kind: EventStopped, Episode: episode, Position: res.Position})
			s.record(ctx, history.Entry{Episode: episode.Number, Position: res.Position})
			return nil
		case err != nil && s.opts.Autoplay && Unavailable(err):
			unavailable++
			if unavailable >= MaxUnavailable {
				return fmt.Errorf("episode %d: %w (%d episodes in a row unavailable; stopping)", episode.Number, err, unavailable)
			}
			s.onEvent(Event{Kind: EventSkipped, Episode: episode, Err: err})
		case err != nil:
			if s.opts.Autoplay {
				return fmt.Errorf("episode %d: %w", episode.Number, err)
			}
			return err
		default:
			unavailable = 0
			s.onEvent(Event{Kind: EventFinished, Episode: episode})
			s.record(ctx, history.Entry{Episode: episode.Number, Finished: true})
		}
		if !s.opts.Autoplay {
			return nil
		}
		next, ok := FindEpisode(items, episode.Number+1)
		if !ok {
			return nil
		}
		episode = next
	}
}

// record saves the watch position. A failure only warns, because it must
// not interrupt playback.
func (s *Session) record(ctx context.Context, e history.Entry) {
	if !s.target.Remember || s.d.History == nil {
		return
	}
	e.SeriesID, e.Title, e.Episodes = s.target.SeriesID, s.target.Title, len(s.target.Episodes)
	if err := s.d.History.Record(ctx, e); err != nil && ctx.Err() == nil {
		s.onEvent(Event{Kind: EventHistoryWarning, Err: err})
	}
}

// resolve returns the prefetched model of episode when there is one, and
// starts resolving it otherwise.
func (s *Session) resolve(ctx context.Context, episode model.Episode) <-chan resolved {
	if s.prefetch.ch != nil && s.prefetch.number == episode.Number {
		ch := s.prefetch.ch
		s.prefetch.ch = nil
		return ch
	}
	return s.fetch(ctx, episode)
}

func (s *Session) startPrefetch(ctx context.Context, episode model.Episode) {
	s.prefetch.number, s.prefetch.ch = episode.Number, s.fetch(ctx, episode)
}

func (s *Session) fetch(ctx context.Context, episode model.Episode) <-chan resolved {
	ch := make(chan resolved, 1)
	go func() {
		pb, err := Resolve(ctx, s.d, episode.ID)
		ch <- resolved{pb, err}
	}()
	return ch
}

// play waits for the episode's model and plays it.
func (s *Session) play(ctx context.Context, episode model.Episode, pending <-chan resolved, key string, start int) (media.Result, error) {
	r := <-pending
	if r.err == nil && Expired(r.playback) {
		// A prefetched model can outlive its CDN URLs if playback paused.
		r = <-s.fetch(ctx, episode)
	}
	if r.err != nil {
		return media.Result{}, r.err
	}
	return s.playResolved(ctx, episode, r.playback, key, start)
}

func (s *Session) playResolved(ctx context.Context, episode model.Episode, pb *model.Playback, key string, start int) (media.Result, error) {
	p, err := Prepare(pb, episode, s.opts.Quality, key, s.opts.NoDecrypt)
	if err != nil {
		return media.Result{}, err
	}
	if pb.Protected && p.Key == "" {
		return media.Result{}, DRMError(pb)
	}
	if s.d.Media == nil {
		return media.Result{}, errors.New("media launcher is not configured")
	}
	req := media.Request{
		URL: p.Stream.URL, Audio: pb.AudioURL, Key: p.Key, Player: s.opts.Player,
		StartSeconds: start, LimitSeconds: s.opts.Preview, DurationSeconds: pb.DurationSeconds,
		NoTerminal: s.opts.NoTerminal,
	}
	var res media.Result
	_, err = WithBackup(ctx, p.Stream, s.backupRetry, func(url string) (int64, error) {
		req.URL = url
		var playErr error
		res, playErr = s.d.Media.Play(ctx, req)
		return 0, playErr
	})
	if !errors.Is(err, media.ErrNeedsFile) || s.opts.Autoplay {
		return res, err
	}
	// open and xdg-open return at once, so autoplay never gets here.
	return media.Result{}, playFromTempFile(ctx, s.d, p.Stream, pb.AudioURL, p.Key, s.opts.Player, s.opts.Preview, s.backupRetry)
}

func (s *Session) backupRetry(err error) {
	s.onEvent(Event{Kind: EventBackupRetry, Err: err})
}
