// Package desktop 是桌面客户端（Rust，经 C ABI）的 Go 入口。
//
// Host 在 dispatch 的 24 个方法之外，还提供观看历史和续播点，以及与命令行共用的
// resolvePlaybackFresh：这些逻辑在 internal/ 里，其他绑定拿不到。设备档案和
// 观看历史与命令行共用同一个配置目录，所以 ohg continue 能接着桌面端的进度播放。
package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/beilunyang/openhongguo/adapters"
	"github.com/beilunyang/openhongguo/adapters/filestore"
	"github.com/beilunyang/openhongguo/bindings/internal/dispatch"
	"github.com/beilunyang/openhongguo/internal/history"
	"github.com/beilunyang/openhongguo/sdk"
)

// Host 是一个打开的客户端，可以在多个线程里同时使用。
type Host struct {
	client *sdk.Client
	hist   *history.Store

	mu    sync.Mutex
	next  uint64
	calls map[uint64]context.CancelFunc
}

// Open 在 configDir 下打开客户端：anonymous.json 是设备档案，history.json 是
// 观看历史，目录不存在时以 0700 创建。configJSON 是 dispatch.Config，可以为空。
func Open(configDir, configJSON string) (*Host, error) {
	if configDir == "" {
		return nil, fmt.Errorf("%w: config dir is empty", sdk.ErrInvalidOptions)
	}
	cfg, err := dispatch.ParseConfig(configJSON)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, err
	}
	opts := sdk.Options{ProfileStore: filestore.New(filepath.Join(configDir, "anonymous.json"))}
	cfg.Apply(&opts)
	if opts.RetryAttempts == 0 {
		opts.RetryAttempts = 3
	}
	c, err := adapters.NewClient("", opts)
	if err != nil {
		return nil, err
	}
	return &Host{
		client: c,
		hist:   history.NewStore(filepath.Join(configDir, "history.json")),
		calls:  map[uint64]context.CancelFunc{},
	}, nil
}

// Start 在 goroutine 里调用 method，argsJSON 是参数数组的 JSON。done 恰好调用
// 一次，参数是 {"result": …} 或 {"error": {code, message, retryable,
// retryAfterMs}}。返回值交给 Cancel。
func (h *Host) Start(method, argsJSON string, done func(envelope string)) uint64 {
	ctx, cancel := context.WithCancel(context.Background())
	h.mu.Lock()
	h.next++
	id := h.next
	h.calls[id] = cancel
	h.mu.Unlock()
	go func() {
		v, err := h.Call(ctx, method, argsJSON)
		h.mu.Lock()
		delete(h.calls, id)
		h.mu.Unlock()
		cancel()
		done(Envelope(v, err))
	}()
	return id
}

// Cancel 取消进行中的调用；被取消的调用照样调用 done，结果是 CANCELLED 错误。
func (h *Host) Cancel(id uint64) {
	h.mu.Lock()
	cancel := h.calls[id]
	h.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Close 取消所有进行中的调用。
func (h *Host) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, cancel := range h.calls {
		cancel()
	}
}

// Call 同步调用 method：先找桌面端的方法，再交给 dispatch。
func (h *Host) Call(ctx context.Context, method, argsJSON string) (any, error) {
	m, ok := methods[method]
	if !ok {
		return dispatch.Call(ctx, h.client, method, argsJSON)
	}
	var a []json.RawMessage
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return nil, fmt.Errorf("%w: arguments: %v", sdk.ErrInvalidOptions, err)
		}
	}
	return m(ctx, h, a)
}

// Envelope 把结果或错误编码成 dispatch 绑定共用的信封。
func Envelope(v any, err error) string {
	var out any
	if err != nil {
		out = map[string]any{"error": dispatch.Error(err)}
	} else {
		out = map[string]any{"result": v}
	}
	b, merr := json.Marshal(out)
	if merr != nil {
		b, _ = json.Marshal(map[string]any{"error": dispatch.Error(merr)})
	}
	return string(b)
}

// ErrorJSON 把 err 编码成错误对象，供 Open 失败时交给调用方。
func ErrorJSON(err error) string {
	b, _ := json.Marshal(dispatch.Error(err))
	return string(b)
}
