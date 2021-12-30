package main
// This file will just hold all the things needed to work with systemd.

// #include <systemd/sd-daemon.h>
// #include <stdlib.h>
import "C"
import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

// wrappers for sd_notify
// see https://www.freedesktop.org/software/systemd/man/sd_notify.html#


func sdReady(status string, pid uint64) error {
	cstr := C.CString(`READY=1
STATUS=%s
MAINPID=%lu`)
	cstr2 := C.CString(status)
	ret := (int)(C.sd_notifyf(0, cstr, cstr2, C.ulong(pid)))
	C.free(unsafe.Pointer(cstr))
	C.free(unsafe.Pointer(cstr2))
	return _sdhandlerr(ret)
}

func sdError(errorno syscall.Errno, errorstr string) error {
	cstr := C.CString(`STATUS=%s
ERRNO=%i`)
	cstr2 := C.CString(errorstr)
	ret := (int)(C.sd_notifyf(0, cstr, cstr2, C.ulong(uint64(errorno))))
	C.free(unsafe.Pointer(cstr))
	C.free(unsafe.Pointer(cstr2))
	return _sdhandlerr(ret)
}


func _sdhandlerr(err int) error {
	if err <= 0 {
		if err == 0 {
			return errors.New("status failed to send: $NOTIFY_SOCKET was not set, thus status message has no destination")
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
	ret := (int)(C.sd_notifyf(0, cstr))
	C.free(unsafe.Pointer(cstr))
	return _sdhandlerr(ret)
}

func sdStopping() error {
	cstr := C.CString(`STOPPING=1`)
	ret := (int)(C.sd_notify(0, ))
	C.free(unsafe.Pointer(cstr))
	return _sdhandlerr(ret)
}
