package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/beilunyang/openhongguo/adapters"
	"github.com/beilunyang/openhongguo/adapters/filestore"
	"github.com/beilunyang/openhongguo/internal/cliapp"
	"github.com/beilunyang/openhongguo/internal/history"
	"github.com/beilunyang/openhongguo/internal/media"
	"github.com/beilunyang/openhongguo/internal/tui"
	"github.com/beilunyang/openhongguo/sdk"
)

func main() { os.Exit(run()) }

func run() int {
	dir := os.Getenv("OHG_CONFIG_DIR")
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return cliapp.ExitFailure
		}
		dir = filepath.Join(base, "openhongguo")
		if err := migrateConfig(filepath.Join(base, "hgdrama"), dir); err != nil {
			fmt.Fprintln(os.Stderr, "ohg: warning: old config not migrated:", err)
		}
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return cliapp.ExitFailure
	}
	store := filestore.New(filepath.Join(dir, "anonymous.json"))
	client, err := adapters.NewClient("", sdk.Options{
		ProfileStore:       store,
		APIOrigin:          os.Getenv("OHG_API_ORIGIN"),
		RegisterOrigin:     os.Getenv("OHG_REGISTER_ORIGIN"),
		DistributionOrigin: os.Getenv("OHG_DISTRIBUTION_ORIGIN"),
		RetryAttempts:      3,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return cliapp.ExitFailure
	}
	// Ctrl-C or SIGTERM cancels the context so a player, ffmpeg, and
	// --autoplay stop together and temp files are removed.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	hist := history.NewStore(filepath.Join(dir, "history.json"))
	args := os.Args[1:]
	if wantsTUI(args, os.Stdin, os.Stdout) {
		// The player's own output would land on the screen the interface
		// draws, so it is dropped.
		err := tui.Run(ctx, tui.Deps{
			Service: client, Media: media.NewWithOutput(io.Discard, io.Discard), History: hist,
			DeviceReset: client, ConfigDir: dir, HTTP: &http.Client{Timeout: 10 * time.Second},
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "ohg:", err)
			return cliapp.ExitFailure
		}
		return cliapp.ExitOK
	}
	return cliapp.Run(ctx, args, cliapp.Dependencies{
		Service: client, Profiles: store, Media: media.New(),
		History: hist, Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
	})
}

// wantsTUI reports whether to open the full-screen interface: always for
// `ohg tui`, and for a bare `ohg` when both ends are a terminal, so a
// script that pipes `ohg` still gets the help text.
func wantsTUI(args []string, stdin, stdout *os.File) bool {
	if len(args) == 1 && args[0] == "tui" {
		return true
	}
	return len(args) == 0 && isTerminal(stdin) && isTerminal(stdout)
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// migrateConfig copies the anonymous profile and watch history from the
// config directory of the project's former name, hgdrama, the first time the
// new one is created. Keeping the profile avoids registering a new device.
func migrateConfig(oldDir, newDir string) error {
	if _, err := os.Stat(newDir); err == nil {
		return nil
	}
	if info, err := os.Stat(oldDir); err != nil || !info.IsDir() {
		return nil
	}
	if err := os.MkdirAll(newDir, 0700); err != nil {
		return err
	}
	for _, name := range []string{"anonymous.json", "history.json"} {
		info, err := os.Lstat(filepath.Join(oldDir, name))
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(oldDir, name))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(newDir, name), data, 0600); err != nil {
			return err
		}
	}
	return nil
}
