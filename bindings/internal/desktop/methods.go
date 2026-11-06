package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/beilunyang/openhongguo/bindings/internal/dispatch"
	"github.com/beilunyang/openhongguo/internal/history"
	"github.com/beilunyang/openhongguo/internal/playback"
	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

type method func(ctx context.Context, h *Host, args []json.RawMessage) (any, error)

// playbackResult 与 dispatch 的 resolvePlayback 结果同形。
type playbackResult struct {
	Playback        *model.Playback   `json:"playback"`
	ContentKey      *model.ContentKey `json:"contentKey,omitempty"`
	ContentKeyError map[string]any    `json:"contentKeyError,omitempty"`
}

type resumePoint struct {
	Episode int `json:"episode"`
	Start   int `json:"start"`
}

var methods = map[string]method{
	// resolvePlaybackFresh 同 resolvePlayback，但走命令行和 TUI 共用的
	// playback.Resolve（受保护的模型照常返回）；换新设备由 SDK 自己完成。
	"resolvePlaybackFresh": func(ctx context.Context, h *Host, a []json.RawMessage) (any, error) {
		var id string
		if err := arg(a, 0, &id); err != nil {
			return nil, err
		}
		pb, err := playback.Resolve(ctx, playback.Deps{Service: h.client}, id)
		if err != nil {
			return nil, err
		}
		if !pb.Protected {
			return playbackResult{Playback: pb}, nil
		}
		key, kerr := sdk.ContentKey(pb)
		if kerr != nil {
			return playbackResult{Playback: pb, ContentKeyError: dispatch.Error(kerr)}, nil
		}
		return playbackResult{Playback: pb, ContentKey: &key}, nil
	},
	"historyList": func(ctx context.Context, h *Host, _ []json.RawMessage) (any, error) {
		return h.hist.List(ctx)
	},
	"historyGet": func(ctx context.Context, h *Host, a []json.RawMessage) (any, error) {
		var id string
		if err := arg(a, 0, &id); err != nil {
			return nil, err
		}
		e, ok, err := h.hist.Get(ctx, id)
		if err != nil || !ok {
			return nil, err
		}
		return e, nil
	},
	"historyRecord": func(ctx context.Context, h *Host, a []json.RawMessage) (any, error) {
		var e history.Entry
		if err := arg(a, 0, &e); err != nil {
			return nil, err
		}
		return nil, h.hist.Record(ctx, e)
	},
	"historyClear": func(ctx context.Context, h *Host, _ []json.RawMessage) (any, error) {
		return nil, h.hist.Clear(ctx)
	},
	// resumePoint 给出续播的集数和秒数；全部看完时从第 1 集重新开始，与 TUI 一致。
	"resumePoint": func(ctx context.Context, h *Host, a []json.RawMessage) (any, error) {
		var id string
		var total int
		if err := arg(a, 0, &id); err != nil {
			return nil, err
		}
		if err := arg(a, 1, &total); err != nil {
			return nil, err
		}
		ep, start, err := playback.ResumePoint(ctx, h.hist, id, total)
		if errors.Is(err, playback.ErrInvalid) {
			return resumePoint{Episode: 1}, nil
		}
		if err != nil {
			return nil, err
		}
		return resumePoint{Episode: ep, Start: start}, nil
	},
}

func arg(a []json.RawMessage, i int, v any) error {
	if i >= len(a) || string(a[i]) == "null" {
		return nil
	}
	if err := json.Unmarshal(a[i], v); err != nil {
		return fmt.Errorf("%w: argument %d: %v", sdk.ErrInvalidOptions, i+1, err)
	}
	return nil
}
