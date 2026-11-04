package tui

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
)

func coverServer(t *testing.T) *httptest.Server {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 30, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 30; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 8), uint8(y * 6), 90, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/c.png" {
			http.NotFound(w, r)
			return
		}
		w.Write(buf.Bytes())
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCoverDrawnInDetailPane(t *testing.T) {
	srv := coverServer(t)
	svc := discoverService()
	for _, s := range svc.series {
		s.CoverURL = srv.URL + "/c.png"
	}
	a := newTestApp(t, svc)
	a.env.theme.name = "dark"
	a.env.covers = newCovers(srv.Client())
	drive(t, a, a.Init())
	resize(t, a, 120, 40)
	press(t, a, "2")
	frame := checkFrame(t, a)
	if strings.Count(frame, "▀") < coverCols*4 {
		t.Fatalf("cover not drawn:\n%s", frame)
	}
}

func TestKittyCoverTransmitsOnceAndIsPlacedInFrame(t *testing.T) {
	srv := coverServer(t)
	c := newCovers(srv.Client())
	c.useKitty()
	url := srv.URL + "/c.png"
	msg := c.fetch(context.Background(), url)()
	if again := c.fetch(context.Background(), url); again != nil {
		t.Fatalf("a loading cover was fetched twice")
	}
	raw, ok := c.store(msg.(coverMsg))().(tea.RawMsg)
	if !ok {
		t.Fatalf("store did not transmit the image")
	}
	seq := raw.Msg.(string)
	if !strings.HasPrefix(seq, "\x1b_G") || strings.Contains(seq, "a=T") || !strings.Contains(seq, "f=100") || len(seq) < 1000 {
		t.Fatalf("transmit sequence = %.80q", seq)
	}
	lines := c.draw(url)
	if !strings.HasPrefix(lines[0], "\x1b_Ga=p,") || !strings.Contains(lines[0], ",C=1,") || !c.isPlacement(strings.TrimSuffix(lines[0], strings.Repeat(" ", coverCols))) {
		t.Fatalf("first cover line does not place the image: %q", lines[0])
	}
	if len(lines) != msg.(coverMsg).rows {
		t.Fatalf("cover has %d lines, want %d", len(lines), msg.(coverMsg).rows)
	}
	for i, l := range lines {
		if n := width(l); n != coverCols {
			t.Fatalf("line %d is %d cells wide, want %d", i, n, coverCols)
		}
	}
	if !strings.Contains(c.forget(), "a=d") {
		t.Fatalf("exit does not delete the images")
	}
}

func TestFailedCoverIsNotRetried(t *testing.T) {
	srv := coverServer(t)
	c := newCovers(srv.Client())
	url := srv.URL + "/missing.png"
	if c.store(c.fetch(context.Background(), url)().(coverMsg)) != nil {
		t.Fatalf("a failed cover wrote to the terminal")
	}
	if c.fetch(context.Background(), url) != nil || c.draw(url) != nil {
		t.Fatalf("a failed cover was retried or drawn")
	}
}

func TestKittyAnswerSwitchesDrawnCovers(t *testing.T) {
	srv := coverServer(t)
	svc := discoverService()
	for _, s := range svc.series {
		s.CoverURL = srv.URL + "/c.png"
	}
	a := newTestApp(t, svc)
	a.env.theme.name = "dark"
	a.env.covers = newCovers(srv.Client())
	init := a.Init()
	queried := false
	for _, c := range init().(tea.BatchMsg) {
		if raw, ok := c().(tea.RawMsg); ok && strings.Contains(raw.Msg.(string), "a=q") {
			queried = true
		}
	}
	if !queried {
		t.Fatalf("Init does not query kitty graphics support")
	}
	drive(t, a, init)
	resize(t, a, 120, 40)
	press(t, a, "2")
	if !strings.Contains(checkFrame(t, a), "▀") {
		t.Fatalf("no half-block cover before the answer")
	}
	_, cmd := a.Update(uv.KittyGraphicsEvent{Options: kitty.Options{ID: 7}, Payload: []byte("OK")})
	if cmd != nil || strings.Contains(checkFrame(t, a), "a=p") {
		t.Fatalf("an answer to another query switched to kitty")
	}
	_, cmd = a.Update(uv.KittyGraphicsEvent{Options: kitty.Options{ID: kittyQueryID}, Payload: []byte("OK")})
	if cmd == nil || !strings.Contains(cmd().(tea.RawMsg).Msg.(string), "f=100") {
		t.Fatalf("the answer did not transmit the drawn cover")
	}
	frame := checkFrame(t, a)
	if strings.Contains(frame, "▀") || strings.Count(frame, "a=p,") != 1 || strings.HasPrefix(frame, kittyClear) {
		t.Fatalf("cover not placed once as a kitty image:\n%q", frame)
	}
	press(t, a, "?")
	if frame := checkFrame(t, a); strings.Contains(frame, "a=p,") || !strings.HasPrefix(frame, kittyClear) {
		t.Fatalf("the cover stays above an overlay")
	}
	press(t, a, "esc")
	press(t, a, "1")
	if frame := checkFrame(t, a); strings.Contains(frame, "a=p,") || !strings.HasPrefix(frame, kittyClear) {
		t.Fatalf("a frame without a cover does not clear the placed one")
	}
}

func TestHostileTitleSequencesNeverReachTheTerminal(t *testing.T) {
	svc := discoverService()
	svc.discover["推荐:"].Items[0].Title = "evil\x1b_Ga=d\x1b\\\x1bP+q544e\x1b\\\x1b]52;c;aGk=\x07end"
	a := newTestApp(t, svc)
	drive(t, a, a.Init())
	resize(t, a, 120, 40)
	press(t, a, "2")
	frame := a.View().Content
	for _, intro := range []string{"\x1b_", "\x1bP", "\x1b]"} {
		if strings.Contains(frame, intro) {
			t.Fatalf("frame carries %q from a title", intro)
		}
	}
	if !strings.Contains(frame, "evil") || !strings.Contains(frame, "end") {
		t.Fatalf("title text was lost:\n%s", ansi.Strip(frame))
	}
}

// ffmpeg seeks back to a HEIC's image data once it has read the boxes; a
// pipe cannot seek once the file outgrows its buffer.
func TestHEICCoverIsHandedToFFmpegAsAFile(t *testing.T) {
	dir := t.TempDir()
	var pic bytes.Buffer
	if err := png.Encode(&pic, image.NewGray(image.Rect(0, 0, 4, 6))); err != nil {
		t.Fatal(err)
	}
	picPath := filepath.Join(dir, "out.png")
	if err := os.WriteFile(picPath, pic.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	ffmpeg := filepath.Join(dir, "ffmpeg")
	script := "#!/bin/sh\nwhile [ $# -gt 0 ]; do [ \"$1\" = -i ] && in=$2; shift; done\n[ -f \"$in\" ] || exit 7\ncat " + picPath + "\n"
	if err := os.WriteFile(ffmpeg, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	heic := append([]byte("\x00\x00\x00\x1cftypheic"), make([]byte, 64<<10)...)
	img, err := decodeWithFFmpeg(context.Background(), ffmpeg, heic)
	if err != nil || img.Bounds().Dx() != 4 {
		t.Fatalf("decode = %v, %v", img, err)
	}
}

func TestOversizedAndUnknownCoversAreRefused(t *testing.T) {
	var big bytes.Buffer
	if err := png.Encode(&big, image.NewGray(image.Rect(0, 0, 5000, 5000))); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/big.png" {
			w.Write(big.Bytes())
			return
		}
		w.Write([]byte("#EXTM3U\n#EXT-X-MEDIA-SEQUENCE:0\nfile:///etc/passwd\n"))
	}))
	t.Cleanup(srv.Close)
	ffmpeg := filepath.Join(t.TempDir(), "ffmpeg")
	if err := os.WriteFile(ffmpeg, []byte("#!/bin/sh\nexit 9\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := getImage(context.Background(), srv.Client(), ffmpeg, srv.URL+"/big.png"); err == nil || !strings.Contains(err.Error(), "5000x5000") {
		t.Fatalf("oversized cover: err = %v", err)
	}
	_, err := getImage(context.Background(), srv.Client(), ffmpeg, srv.URL+"/list.m3u8")
	if !errors.Is(err, image.ErrFormat) {
		t.Fatalf("a playlist was handed to ffmpeg: err = %v", err)
	}
}
