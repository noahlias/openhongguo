package core

import "github.com/beilunyang/openhongguo/bindings/internal/desktop"

// Host 是移动端 App 用的客户端：除了 Client 的 24 个方法，还有观看历史、续播点和
// resolvePlaybackFresh，与桌面端走同一个 desktop.Host。Client 留给只要 SDK 的库。
type Host struct {
	h *desktop.Host
}

// OpenHost 在 configDir 下打开客户端（anonymous.json 和 history.json），目录不存在时
// 自动创建；configJSON 是 dispatch.Config 的 JSON，可以为空。
// 失败时，错误信息是 {code, message, retryable, retryAfterMs} 形式的 JSON。
func OpenHost(configDir, configJSON string) (*Host, error) {
	h, err := desktop.Open(configDir, configJSON)
	if err != nil {
		return nil, jsonError(err)
	}
	return &Host{h: h}, nil
}

// Start 在后台开始调用 method，argsJSON 是参数数组的 JSON；callback.Done 恰好调用一次。
func (h *Host) Start(method, argsJSON string, callback Callback) *HostCall {
	id := h.h.Start(method, argsJSON, callback.Done)
	return &HostCall{h: h.h, id: id}
}

// Close 取消所有进行中的调用。
func (h *Host) Close() { h.h.Close() }

// HostCall 是 Host 上一次进行中的调用。gomobile 不能导出 uint64，所以调用号留在 Go 里。
type HostCall struct {
	h  *desktop.Host
	id uint64
}

// Cancel 取消调用；被取消的调用照样调用 Callback.Done，结果是 CANCELLED 错误。
func (c *HostCall) Cancel() { c.h.Cancel(c.id) }
