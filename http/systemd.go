package main

// This file will just hold all the things needed to work with systemd.

// #cgo LDFLAGS: -lsystemd
// #include <systemd/sd-daemon.h>
// #include <stdlib.h>
import "C"
import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

// wrappers for sd_notify
// see https://www.freedesktop.org/software/systemd/man/sd_notify.html#

func sdReady(status string, pid uint64) error {
	status = strings.Replace(status, `
`, `\n`, -1)
	cstr := C.CString(fmt.Sprintf(`READY=1
STATUS=%s
MAINPID=%d`, status, pid))
	ret := (int)(C.sd_notify(0, cstr))
	C.free(unsafe.Pointer(cstr))
	return _sdhandlerr(ret)
}

func sdError(errorno syscall.Errno, errorstr string) error {
	errorstr = strings.Replace(errorstr, `
`, `\n`, -1)
	cstr := C.CString(fmt.Sprintf(`STATUS=%s
ERRNO=%d`, errorstr, errorno))
	ret := (int)(C.sd_notify(0, cstr))
	C.free(unsafe.Pointer(cstr))
	return _sdhandlerr(ret)
}

func _sdhandlerr(err int) error {
	if err <= 0 {
		if err == 0 {
			// see this error message as to whats going on here.
			// I decided to comment it out becuase its not really an error...
			// if NOTIFY_SOCKET is not set then that just means sd_notify is
			// disabled.
			// return errors.New("status failed to send: $NOTIFY_SOCKET was not set, thus status message has no destination")
			return nil
		}
		var errstr string
		errstr = syscall.Errno(-err).Error()
		return errors.New(fmt.Sprintf("status failed to send: %s", errstr))
	}
	return nil
}

// "Note that a service that sends this notification must also send a "READY=1"
//  notification when it completed reloading its configuration."
// ... other words, Make sure you call sdReady / sdError when youre done.
func sdReloading() error {
	cstr := C.CString(`RELOADING=1`)
	ret := (int)(C.sd_notify(0, cstr))
	C.free(unsafe.Pointer(cstr))
	return _sdhandlerr(ret)
}

func sdStopping() error {
	cstr := C.CString(`STOPPING=1`)
	ret := (int)(C.sd_notify(0, cstr))
	C.free(unsafe.Pointer(cstr))
	return _sdhandlerr(ret)
}
