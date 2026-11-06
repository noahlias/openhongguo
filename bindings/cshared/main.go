// Command cshared 把 bindings/internal/desktop 编译成 C 静态库，供桌面客户端
// （desktop/，Rust）链接：
//
//	go build -buildmode=c-archive -o libohg.a ./bindings/cshared
//
// cgo 不允许把 Go 指针交给 C，所以客户端用整数句柄表示。结果字符串只在
// done 回调期间有效，调用方需要自己复制。
package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef void (*ohg_done_fn)(void *userdata, const char *envelope);

static inline void ohg_invoke(ohg_done_fn f, void *userdata, const char *envelope) {
	f(userdata, envelope);
}
*/
import "C"

import (
	"sync"
	"unsafe"

	"github.com/beilunyang/openhongguo/bindings/internal/desktop"
)

var (
	mu    sync.Mutex
	next  uint64
	hosts = map[uint64]*desktop.Host{}
)

func host(h C.uint64_t) *desktop.Host {
	mu.Lock()
	defer mu.Unlock()
	return hosts[uint64(h)]
}

// ohg_open 打开客户端，失败时返回 0，并把错误对象的 JSON 写进 *errJSON
// （用 ohg_free 释放）。
//
//export ohg_open
func ohg_open(configDir, configJSON *C.char, errJSON **C.char) C.uint64_t {
	h, err := desktop.Open(C.GoString(configDir), C.GoString(configJSON))
	if err != nil {
		if errJSON != nil {
			*errJSON = C.CString(desktop.ErrorJSON(err))
		}
		return 0
	}
	mu.Lock()
	next++
	id := next
	hosts[id] = h
	mu.Unlock()
	return C.uint64_t(id)
}

// ohg_close 取消进行中的调用并释放句柄。
//
//export ohg_close
func ohg_close(h C.uint64_t) {
	mu.Lock()
	x := hosts[uint64(h)]
	delete(hosts, uint64(h))
	mu.Unlock()
	if x != nil {
		x.Close()
	}
}

// ohg_call 在后台调用 method。done 在 Go 的线程里恰好调用一次；句柄无效时
// 立即以 INVALID_OPTIONS 调用。返回值交给 ohg_cancel。
//
//export ohg_call
func ohg_call(h C.uint64_t, method, argsJSON *C.char, done C.ohg_done_fn, userdata unsafe.Pointer) C.uint64_t {
	finish := func(envelope string) {
		s := C.CString(envelope)
		C.ohg_invoke(done, userdata, s)
		C.free(unsafe.Pointer(s))
	}
	x := host(h)
	if x == nil {
		finish(`{"error":{"code":"INVALID_OPTIONS","message":"unknown client handle","retryable":false,"retryAfterMs":0}}`)
		return 0
	}
	return C.uint64_t(x.Start(C.GoString(method), C.GoString(argsJSON), finish))
}

// ohg_cancel 取消一次调用；done 照样调用，结果是 CANCELLED 错误。
//
//export ohg_cancel
func ohg_cancel(h C.uint64_t, call C.uint64_t) {
	if x := host(h); x != nil {
		x.Cancel(uint64(call))
	}
}

// ohg_free 释放 ohg_open 写出的错误字符串。
//
//export ohg_free
func ohg_free(s *C.char) { C.free(unsafe.Pointer(s)) }

func main() {}
