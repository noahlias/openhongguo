package cliapp

import (
	"fmt"

	"github.com/beilunyang/openhongguo/internal/playback"
	"github.com/beilunyang/openhongguo/sdk/model"
)

// engine adapts the command's collaborators for the playback engine.
func (d Dependencies) engine() playback.Deps {
	e := playback.Deps{}
	if d.Service != nil {
		e.Service = d.Service
	}
	if d.Media != nil {
		e.Media = d.Media
	}
	if d.History != nil {
		e.History = d.History
	}
	return e
}

// eventPrinter writes the engine's events as the command's status lines.
type eventPrinter struct {
	d Dependencies
	f flags
}

func (p eventPrinter) event(e playback.Event) {
	switch e.Kind {
	case playback.EventStarted:
		if p.f.autoplay || p.f.resume || e.Episode.Number != p.f.episode {
			fmt.Fprintf(p.d.Stderr, "ohg: playing episode %d of %d\n", e.Episode.Number, e.Total)
		}
	case playback.EventSkipped:
		fmt.Fprintf(p.d.Stderr, "ohg: episode %d is not available anonymously; skipping\n", e.Episode.Number)
	case playback.EventBackupRetry:
		p.retry(e.Err)
	case playback.EventHistoryWarning:
		fmt.Fprintln(p.d.Stderr, "ohg: warning: history not saved:", e.Err)
	}
}

func (p eventPrinter) retry(err error) {
	fmt.Fprintf(p.d.Stderr, "ohg: %v; retrying the backup URL\n", err)
}

func (p eventPrinter) download(e playback.DownloadEvent) {
	switch {
	case e.Err != nil:
		fmt.Fprintf(p.d.Stderr, "ohg: episode %d: %v\n", e.Episode.Number, e.Err)
	case e.Skipped:
		fmt.Fprintf(p.d.Stdout, "%d\t%s\texists\n", e.Episode.Number, e.Path)
	default:
		fmt.Fprintf(p.d.Stdout, "%d\t%s\t%d\n", e.Episode.Number, e.Path, e.Written)
	}
}

func (p eventPrinter) downloadRetry(_ model.Episode, err error) {
	p.retry(err)
}
