package tui

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Settings are the viewer's choices, kept in settings.json next to the
// anonymous profile.
type Settings struct {
	Version     int    `json:"version"`
	Language    string `json:"language"`
	Theme       string `json:"theme"`
	Player      string `json:"player"`
	Quality     string `json:"quality"`
	Autoplay    bool   `json:"autoplay"`
	DownloadDir string `json:"download_dir"`
	Jobs        int    `json:"jobs"`
	Covers      bool   `json:"covers"`
}

const settingsVersion = 1

var (
	languages = []string{"zh", "en"}
	themes    = []string{"auto", "dark", "light", "mono"}
	players   = []string{"auto", "mpv", "ffplay", "vlc", "open", "xdg-open"}
	qualities = []string{"best", "worst", "360p", "480p", "540p", "720p", "1080p"}
)

func defaultSettings() Settings {
	return Settings{
		Version: settingsVersion, Language: "zh", Theme: "auto", Player: "auto",
		Quality: "best", Jobs: 3, Covers: true,
	}
}

// loadSettings reads path. A missing file gives the defaults; a file that
// cannot be read gives the defaults and the error, so the caller can say so.
func loadSettings(path string) (Settings, error) {
	s := defaultSettings()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return defaultSettings(), err
	}
	s.Version = settingsVersion
	if !contains(languages, s.Language) {
		s.Language = "zh"
	}
	if !contains(themes, s.Theme) {
		s.Theme = "auto"
	}
	if !contains(players, s.Player) {
		s.Player = "auto"
	}
	if !contains(qualities, s.Quality) {
		s.Quality = "best"
	}
	if s.Jobs < 1 || s.Jobs > 16 {
		s.Jobs = 3
	}
	return s, nil
}

// saveSettings writes s through a temp file so a crash never leaves a
// half-written file.
func saveSettings(path string, s Settings) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// cycle returns the entry after v in list, wrapping around; by -1 goes back.
func cycle(list []string, v string, by int) string {
	for i, x := range list {
		if x == v {
			n := len(list)
			return list[((i+by)%n+n)%n]
		}
	}
	return list[0]
}
