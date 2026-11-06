package desktop

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/bindings/internal/fixtures"
)

func newHost(t *testing.T, handler http.Handler) (*Host, string) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	dir := filepath.Join(t.TempDir(), "cfg")
	cfg, _ := json.Marshal(map[string]any{"apiOrigin": srv.URL, "registerOrigin": srv.URL, "retryAttempts": 1})
	h, err := Open(dir, string(cfg))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	return h, dir
}

func call(t *testing.T, h *Host, method, args string) map[string]json.RawMessage {
	t.Helper()
	done := make(chan string, 1)
	h.Start(method, args, func(s string) { done <- s })
	select {
	case out := <-done:
		var env map[string]json.RawMessage
		if err := json.Unmarshal([]byte(out), &env); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
		return env
	case <-time.After(10 * time.Second):
		t.Fatal("no callback")
	}
	return nil
}

func errCode(t *testing.T, env map[string]json.RawMessage) string {
	t.Helper()
	var e struct{ Code string }
	if raw, ok := env["error"]; ok {
		if err := json.Unmarshal(raw, &e); err != nil {
			t.Fatal(err)
		}
	}
	return e.Code
}

func TestDispatchFallthrough(t *testing.T) {
	h, dir := newHost(t, fixtures.Handler("../../../testdata/protocol"))
	env := call(t, h, "search", `["短剧"]`)
	var page struct {
		Items []struct{ ID string } `json:"items"`
	}
	if err := json.Unmarshal(env["result"], &page); err != nil || len(page.Items) == 0 {
		t.Fatalf("result = %s (%v)", env["result"], err)
	}
	if _, err := os.Stat(filepath.Join(dir, "anonymous.json")); err != nil {
		t.Fatal(err)
	}
	if code := errCode(t, call(t, h, "nope", `[]`)); code != "INVALID_OPTIONS" {
		t.Fatalf("code = %q", code)
	}
}

func TestResolvePlaybackFresh(t *testing.T) {
	h, _ := newHost(t, fixtures.Handler("../../../testdata/protocol"))
	env := call(t, h, "resolvePlaybackFresh", `["7000000000000000001"]`)
	var r struct {
		Playback struct {
			Protected bool `json:"protected"`
			Streams   []struct {
				URL string `json:"url"`
			} `json:"streams"`
		} `json:"playback"`
		ContentKey      *struct{}             `json:"contentKey"`
		ContentKeyError struct{ Code string } `json:"contentKeyError"`
	}
	if err := json.Unmarshal(env["result"], &r); err != nil {
		t.Fatalf("%v: %s", err, env["error"])
	}
	// 样本里的密钥材料是假的：推导失败，但播放模型照常返回。
	if !r.Playback.Protected || len(r.Playback.Streams) == 0 || r.ContentKey != nil || r.ContentKeyError.Code != "DRM_PROTECTED" {
		t.Fatalf("result = %s", env["result"])
	}
}

func TestHistory(t *testing.T) {
	h, dir := newHost(t, fixtures.Handler("../../../testdata/protocol"))

	if env := call(t, h, "historyGet", `["s1"]`); string(env["result"]) != "null" {
		t.Fatalf("empty get = %s", env["result"])
	}
	var rp struct{ Episode, Start int }
	if err := json.Unmarshal(call(t, h, "resumePoint", `["s1", 10]`)["result"], &rp); err != nil || rp.Episode != 1 || rp.Start != 0 {
		t.Fatalf("fresh resume = %+v (%v)", rp, err)
	}

	entry := `[{"series_id":"s1","title":"T","episode":4,"episodes":10,"finished":false,"position":63.5}]`
	if code := errCode(t, call(t, h, "historyRecord", entry)); code != "" {
		t.Fatalf("record: %s", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "history.json")); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(call(t, h, "resumePoint", `["s1", 10]`)["result"], &rp); err != nil || rp.Episode != 4 || rp.Start != 60 {
		t.Fatalf("resume = %+v (%v)", rp, err)
	}

	var list []struct {
		SeriesID string  `json:"series_id"`
		Position float64 `json:"position"`
	}
	if err := json.Unmarshal(call(t, h, "historyList", `[]`)["result"], &list); err != nil || len(list) != 1 || list[0].Position != 63.5 {
		t.Fatalf("list = %+v (%v)", list, err)
	}

	done := `[{"series_id":"s1","episode":10,"episodes":10,"finished":true}]`
	call(t, h, "historyRecord", done)
	if err := json.Unmarshal(call(t, h, "resumePoint", `["s1", 10]`)["result"], &rp); err != nil || rp.Episode != 1 {
		t.Fatalf("all watched resume = %+v (%v)", rp, err)
	}

	call(t, h, "historyClear", `[]`)
	if env := call(t, h, "historyList", `[]`); string(env["result"]) != "[]" {
		t.Fatalf("after clear = %s", env["result"])
	}
}

func TestCancel(t *testing.T) {
	block := make(chan struct{})
	h, _ := newHost(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-block:
		case <-r.Context().Done():
		}
	}))
	defer close(block)
	done := make(chan string, 1)
	id := h.Start("search", `["x"]`, func(s string) { done <- s })
	h.Cancel(id)
	select {
	case out := <-done:
		var env map[string]json.RawMessage
		json.Unmarshal([]byte(out), &env)
		if code := errCode(t, env); code != "CANCELLED" {
			t.Fatalf("code = %q in %s", code, out)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("cancel did not finish the call")
	}
}
