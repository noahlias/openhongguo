// Package tui is the full-screen interface of ohg. It browses every surface
// the service exposes, plays episodes with an external player while it
// stays on screen, and queues downloads.
package tui

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/beilunyang/openhongguo/internal/cliapp"
	"github.com/beilunyang/openhongguo/internal/history"
	"github.com/beilunyang/openhongguo/internal/playback"
)

// Deps are the collaborators the interface needs.
type Deps struct {
	Service     cliapp.Service
	Media       playback.Media
	History     *history.Store
	DeviceReset playback.DeviceResetter
	// ConfigDir holds settings.json.
	ConfigDir string
	// HTTP fetches covers; without it no covers are drawn.
	HTTP *http.Client
}

// Run shows the interface until the viewer quits or ctx ends.
func Run(ctx context.Context, d Deps) error {
	path := filepath.Join(d.ConfigDir, "settings.json")
	settings, loadErr := loadSettings(path)
	a := newApp(ctx, d, settings, path, loadErr)
	p := tea.NewProgram(a, tea.WithContext(ctx))
	a.player.send = p.Send
	a.downloads.send = p.Send
	_, err := p.Run()
	a.player.stop()
	a.downloads.cancelCurrent()
	if a.env.covers != nil {
		if _, werr := os.Stdout.WriteString(a.env.covers.forget()); werr != nil && err == nil {
			err = werr
		}
	}
	return err
}
