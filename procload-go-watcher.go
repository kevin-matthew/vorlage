package vorlage

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

type watcher struct {
	fd        int
	wd        int
	dir       string
	closed    bool
	closederr error
}

func newwatcher(dir string) (w watcher, err error) {
	w.fd, err = syscall.InotifyInit()
	if err != nil {
		return w, err
	}
	w.wd, err = syscall.InotifyAddWatch(w.fd, dir, syscall.IN_CLOSE_WRITE)
	w.dir = dir
	return w, err
}

// will hang until one of the files in filenames had been updated, then will
// return which file was updated.
func (w watcher) waitForUpdate() (string, error) {

	buffer := make([]byte, syscall.SizeofInotifyEvent*1+syscall.NAME_MAX+1)
	_, err := syscall.Read(w.fd, buffer)
	if err != nil {
		return "", err
	}
	evt := (*syscall.InotifyEvent)(unsafe.Pointer(&buffer[0]))
	fmt.Printf(`%#v
`, evt)
	nameLen := uint32(evt.Len)
	if nameLen > 0 {
		// Point "bytes" at the first byte of the filename
		bytes := (*[syscall.PathMax]byte)(unsafe.Pointer(&buffer[syscall.SizeofInotifyEvent]))[:nameLen:nameLen]
		// The filename is padded with NULL bytes. TrimRight() gets rid of those.
		filename := strings.TrimRight(string(bytes[0:nameLen]), "\000")
		return filename, nil
	}

	return "", nil
}

func (w watcher) close() {
	syscall.InotifyRmWatch(w.fd, uint32(w.wd))
	syscall.Close(w.fd)
}
