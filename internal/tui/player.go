package tui

import (
	"context"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/beilunyang/openhongguo/internal/playback"
)

// playerCtl runs one playback session at a time in the background and
// turns its events into messages for the program.
type playerCtl struct {
	mu      sync.Mutex
	send    func(tea.Msg)
	cancel  context.CancelFunc
	running bool
	title   string
	episode int
	total   int
}

type playerEventMsg struct {
	ev    playback.Event
	title string
}

type playerDoneMsg struct {
	title string
	err   error
}

// start begins a session and reports false when one is already running.
func (p *playerCtl) start(ctx context.Context, d playback.Deps, opts playback.Options, t playback.Target) bool {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return false
	}
	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.running = true
	p.title = t.Title
	p.episode, p.total = t.Start.Number, len(t.Episodes)
	p.mu.Unlock()
	s := playback.NewSession(d, opts, p.onEvent)
	go func() {
		err := s.Run(ctx, t)
		cancel()
		p.mu.Lock()
		p.running = false
		p.cancel = nil
		p.mu.Unlock()
		p.deliver(playerDoneMsg{title: t.Title, err: err})
	}()
	return true
}

func (p *playerCtl) onEvent(ev playback.Event) {
	p.mu.Lock()
	if ev.Kind == playback.EventStarted {
		p.episode, p.total = ev.Episode.Number, ev.Total
	}
	title := p.title
	p.mu.Unlock()
	p.deliver(playerEventMsg{ev: ev, title: title})
}

func (p *playerCtl) deliver(msg tea.Msg) {
	p.mu.Lock()
	send := p.send
	p.mu.Unlock()
	if send != nil {
		send(msg)
	}
}

// stop ends the running session. The player is killed, so the position of
// the current episode is not saved.
func (p *playerCtl) stop() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel == nil {
		return false
	}
	p.cancel()
	return true
}

func (p *playerCtl) status() (title string, episode, total int, running bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.title, p.episode, p.total, p.running
}
