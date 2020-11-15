package vorlage

// #cgo LDFLAGS: -ldl
// #include "c.src/vorlage.h"
// #include "c.src/processors.h"
// #include <stdlib.h>
// #include <dlfcn.h>
import "C"
import (
	"fmt"
	"unsafe"
)
import "../lmgo/errors"

type cProc struct {
	libname string
	handle  unsafe.Pointer

	// function pointers
	vorlageStartup   unsafe.Pointer
	vorlageOnRequest unsafe.Pointer
	vorlageDefine    unsafe.Pointer
	vorlageShutdown  unsafe.Pointer
}

func (c *cProc) Startup() ProcessorInfo {
	panic("implement me")
}

func (c *cProc) OnRequest(info RequestInfo) []Action {
	panic("implement me")
}

func (c *cProc) DefineVariable(info DefineInfo) Definition {
	panic("implement me")
}

func (c *cProc) Shutdown() ExitInfo {
	panic("implement me")
}

var _ Processor = cProc{}

func LoadCProcessors(libpath string) (error, []Processor) {

}

// dlOpen tries to get a handle to a library (.so), attempting to access it
// by the names specified in libs and returning the first that is successfully
// opened. Callers are responsible for closing the handler. If no library can
// be successfully opened, an error is returned.
func dlOpen(libPath string) (*cProc, error) {
	libname := C.CString(libPath)
	defer C.free(unsafe.Pointer(libname))

	handle := C.dlopen(libname, C.RTL_NOW)
	if handle == nil {
		e := C.dlerror()
		if e == nil {
			return nil, errors.New(0x82acb,
				"dlopen failed but dlerror did not return an error",
				nil,
				"I have no idea what to do.",
				libname)

		}
		return nil, errors.NewCauseString(0x10baa,
			"failed to load in library",
			C.GoString(e),
			"make sure the library exists and it links properly",
			libname)
	}
	h := &cProc{
		handle:  handle,
		libname: libname,
	}
	return h, nil
}

func (c *cProc) loadVorlageSymbols() error {
	// check vorlage_proc_interfaceversion to make sure it's the same as ours.
	// get functions
}

func (c *cProc) getSymbolPointer(symbol string) (unsafe.Pointer, error) {
	sym := C.CString(symbol)
	defer C.free(unsafe.Pointer(sym))
	C.dlerror() // clear last error
	p := C.dlsym(c.handle, sym)
	e := C.dlerror()
	if e != nil {
		return nil, errors.NewCauseStringf(0x10b341,
			"failed to get symbol",
			C.GoString(e),
			"make sure this library has the proper symbol exported",
			"finding symbol '%s' in %s", symbol, c.libname)
	}
	return p, nil
}

func (c *cProc) Close() error {
	C.dlerror() // clear last error
	C.dlclose(c.handle)
	e := C.dlerror()
	if e != nil {
		return errors.NewCauseString(0x584148,
			"dlclose failed to close handle",
			C.GoString(e),
			"",
			c.libname)
	}
	return nil
}
