package tui

import (
	"context"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/beilunyang/openhongguo/internal/playback"
	"github.com/beilunyang/openhongguo/sdk/model"
)

type taskState int

const (
	taskWaiting taskState = iota
	taskRunning
	taskDone
	taskFailed
	taskCancelled
)

// downloadTask is one series' download in the queue.
type downloadTask struct {
	id       int
	seriesID string
	title    string
	dir      string
	episodes []model.Episode
	numbers  []int
	state    taskState
	saved    int
	skipped  int
	failed   int
	last     string
	err      error
}

func (t *downloadTask) total() int { return len(t.numbers) }
func (t *downloadTask) done() int  { return t.saved + t.skipped + t.failed }

type downloadEventMsg struct{ taskID int }
type downloadDoneMsg struct {
	taskID int
	title  string
	failed int
	err    error
}

// downloadCtl saves series one after another in the background.
type downloadCtl struct {
	mu      sync.Mutex
	send    func(tea.Msg)
	ctx     context.Context
	deps    playback.Deps
	tasks   []*downloadTask
	running *downloadTask
	cancel  context.CancelFunc
	nextID  int
	jobs    func() int
}

func (c *downloadCtl) enqueue(t *downloadTask) {
	c.mu.Lock()
	c.nextID++
	t.id = c.nextID
	c.tasks = append(c.tasks, t)
	c.mu.Unlock()
	c.startNext()
}

func (c *downloadCtl) startNext() {
	c.mu.Lock()
	if c.running != nil {
		c.mu.Unlock()
		return
	}
	var next *downloadTask
	for _, t := range c.tasks {
		if t.state == taskWaiting {
			next = t
			break
		}
	}
	if next == nil {
		c.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(c.ctx)
	c.running, c.cancel = next, cancel
	next.state = taskRunning
	jobs := 3
	if c.jobs != nil {
		jobs = c.jobs()
	}
	c.mu.Unlock()
	job := playback.DownloadJob{Episodes: next.episodes, Numbers: next.numbers, Dir: next.dir, Jobs: jobs}
	go func() {
		failed, err := playback.Download(ctx, c.deps, job, func(ev playback.DownloadEvent) { c.onEvent(next, ev) })
		cancel()
		c.mu.Lock()
		switch {
		case err != nil && ctx.Err() != nil && c.ctx.Err() == nil:
			next.state = taskCancelled
		case err != nil:
			next.state, next.err = taskFailed, err
		case failed > 0:
			next.state = taskFailed
		default:
			next.state = taskDone
		}
		c.running, c.cancel = nil, nil
		c.mu.Unlock()
		c.deliver(downloadDoneMsg{taskID: next.id, title: next.title, failed: failed, err: err})
		c.startNext()
	}()
}

func (c *downloadCtl) onEvent(t *downloadTask, ev playback.DownloadEvent) {
	c.mu.Lock()
	switch {
	case ev.Err != nil:
		t.failed++
		t.err = ev.Err
	case ev.Skipped:
		t.skipped++
	default:
		t.saved++
	}
	t.last = ev.Path
	c.mu.Unlock()
	c.deliver(downloadEventMsg{taskID: t.id})
}

func (c *downloadCtl) deliver(msg tea.Msg) {
	c.mu.Lock()
	send := c.send
	c.mu.Unlock()
	if send != nil {
		send(msg)
	}
}

// cancelCurrent stops the running task; the queue moves on.
func (c *downloadCtl) cancelCurrent() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel == nil {
		return false
	}
	c.cancel()
	return true
}

// remove drops a finished or waiting task from the queue.
func (c *downloadCtl) remove(id int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	kept := c.tasks[:0]
	for _, t := range c.tasks {
		if t.id == id && t.state != taskRunning {
			continue
		}
		kept = append(kept, t)
	}
	c.tasks = kept
}

// snapshot copies the queue for drawing.
func (c *downloadCtl) snapshot() []downloadTask {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]downloadTask, len(c.tasks))
	for i, t := range c.tasks {
		out[i] = *t
	}
	return out
}

func (c *downloadCtl) active() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, t := range c.tasks {
		if t.state == taskWaiting || t.state == taskRunning {
			n++
		}
	}
	return n
}
