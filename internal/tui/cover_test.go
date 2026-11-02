package tui

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
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

func TestKittyCoverTransmitsOnceAndUsesPlaceholders(t *testing.T) {
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
	if !strings.HasPrefix(seq, "\x1b_G") || !strings.Contains(seq, "U=1") || !strings.Contains(seq, "f=100") {
		t.Fatalf("transmit sequence = %.80q", seq)
	}
	lines := c.draw(url)
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
	if cmd != nil || strings.ContainsRune(checkFrame(t, a), kitty.Placeholder) {
		t.Fatalf("an answer to another query switched to kitty")
	}
	_, cmd = a.Update(uv.KittyGraphicsEvent{Options: kitty.Options{ID: kittyQueryID}, Payload: []byte("OK")})
	if cmd == nil || !strings.Contains(cmd().(tea.RawMsg).Msg.(string), "U=1") {
		t.Fatalf("the answer did not transmit the drawn cover")
	}
	frame := checkFrame(t, a)
	if strings.Contains(frame, "▀") || !strings.ContainsRune(frame, kitty.Placeholder) {
		t.Fatalf("cover still drawn as half-blocks:\n%s", frame)
	}
}
