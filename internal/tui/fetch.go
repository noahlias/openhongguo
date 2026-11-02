package tui

import (
	"context"
	"errors"

	"github.com/beilunyang/openhongguo/sdk"
)

// describeError turns a service error into a short line for a toast.
func describeError(err error, tr *Strings) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.Canceled):
		return ""
	case errors.Is(err, sdk.ErrAnonymousLimited):
		return tr.ErrAnonymous
	case errors.Is(err, sdk.ErrAuthenticationRequired):
		return tr.ErrAuth
	case errors.Is(err, sdk.ErrDRMProtected):
		return tr.ErrDRM
	case errors.Is(err, sdk.ErrContentUnavailable), errors.Is(err, sdk.ErrAlbumUnavailable):
		return tr.ErrGone
	case errors.Is(err, sdk.ErrRateLimited):
		return tr.ErrRate
	case errors.Is(err, sdk.ErrTransport):
		return tr.ErrNetwork
	case errors.Is(err, sdk.ErrDeviceRejected):
		return tr.ErrDevice
	case errors.Is(err, sdk.ErrEndOfList):
		return tr.ErrEnd
	}
	return tr.ErrOther + ": " + err.Error()
}
