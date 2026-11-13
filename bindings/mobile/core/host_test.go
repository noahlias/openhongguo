package core

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/bindings/internal/fixtures"
)

func openTestHost(t *testing.T, origin string) (*Host, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "cfg")
	cfg, _ := json.Marshal(map[string]any{"apiOrigin": origin, "registerOrigin": origin, "retryAttempts": 1})
	h, err := OpenHost(dir, string(cfg))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	return h, dir
}

func TestHostDispatchAndHistory(t *testing.T) {
	srv := httptest.NewServer(fixtures.Handler("../../../testdata/protocol"))
	defer srv.Close()
	h, dir := openTestHost(t, srv.URL)
	done := make(chanCallback, 1)

	h.Start("search", `["短剧"]`, done)
	var page struct {
		Items []struct{ ID string } `json:"items"`
	}
	if err := json.Unmarshal(await(t, done)["result"], &page); err != nil || len(page.Items) == 0 {
		t.Fatalf("search = %+v (%v)", page, err)
	}

	h.Start("historyRecord", `[{"series_id":"s1","episode":4,"episodes":10,"position":63.5}]`, done)
	if env := await(t, done); env["error"] != nil {
		t.Fatalf("record: %s", env["error"])
	}
	if _, err := os.Stat(filepath.Join(dir, "history.json")); err != nil {
		t.Fatal(err)
	}
	h.Start("resumePoint", `["s1", 10]`, done)
	var rp struct{ Episode, Start int }
	if err := json.Unmarshal(await(t, done)["result"], &rp); err != nil || rp.Episode != 4 || rp.Start != 60 {
		t.Fatalf("resume = %+v (%v)", rp, err)
	}
}

func TestHostCancel(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(httptestHandler(block))
	defer srv.Close()
	defer close(block)
	h, _ := openTestHost(t, srv.URL)

	done := make(chanCallback, 1)
	call := h.Start("search", `["短剧"]`, done)
	time.Sleep(100 * time.Millisecond)
	call.Cancel()
	var e struct{ Code string }
	if err := json.Unmarshal(await(t, done)["error"], &e); err != nil || e.Code != "CANCELLED" {
		t.Fatalf("error = %+v (%v)", e, err)
	}
}

func TestOpenHostError(t *testing.T) {
	_, err := OpenHost("", "")
	var e struct{ Code string }
	if err == nil || json.Unmarshal([]byte(err.Error()), &e) != nil || e.Code != "INVALID_OPTIONS" {
		t.Fatalf("err = %v", err)
	}
}
