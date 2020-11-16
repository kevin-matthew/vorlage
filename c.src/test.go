package main

// #cgo LDFLAGS: -ldl
// #include "processors.h"
// #include <stdlib.h>
// #include <dlfcn.h>
// #include <stdio.h>
// typedef void *(*voidfunc) (void *args);
// typedef vorlage_proc_info (*startupwrap)();
// vorlage_proc_info execstartupwrap(startupwrap f) {
// return f();
// }
// typedef int fuckingfuckfuckgodamn;
// void *fuckyou(void *p, fuckingfuckfuckgodamn i, int fuckingsize) {
//      return (void*)((uint64_t)p+i*fuckingsize);
// }
//
import "C"
import (
	"fmt"
	"io"
	"regexp"
	"unsafe"
)
import "../../lmgo/errors"

type cProc struct {
	libname string
	handle  unsafe.Pointer

	// function pointers
	vorlageInterfaceVersion uint32
	vorlageStartup   unsafe.Pointer
	vorlageOnRequest unsafe.Pointer
	vorlageDefine    unsafe.Pointer
	vorlageShutdown  unsafe.Pointer


	test unsafe.Pointer
}

type rawCProcInfo C.vorlage_proc_info

func main() {
	ourVersion   := int(C.vorlage_proc_interfaceversion);
	fmt.Printf("our version: %d\n", ourVersion)
	p,err := dlOpen("./libtest.so")
	if err != nil {
		fmt.Printf("failed to open dl: %#v\n", err)
		return
	}
	err = p.loadVorlageSymbols()
	if err != nil {
		fmt.Printf("failed to load syms: %#v\n", err)
		return
	}
	p.Startup()
	println("done")
}

func (c *cProc) Startup() ProcessorInfo {
	f := C.startupwrap(c.vorlageStartup)
	d := C.execstartupwrap(f)
	p := ProcessorInfo{}
	// description
	p.Description = C.GoString(d.description);

	// input proto
	p.InputProto        = parseInputProtoType(int(d.inputprotoc), d.inputprotov)
	//p.StreamInputProto  = parseInputProtoType(int(d.streaminputprotoc), d.streaminputprotov)
	//p.Variables         = parseVariables(int(d.variablesc), d.variablesv)
	return p
}
func parseVariables(varsc int, varsv unsafe.Pointer) []ProcessorVariable {
	return nil
}
func parseInputProtoType(protoc int, protov *C.vorlage_proc_inputproto) []InputPrototype {
	ret := make([]InputPrototype, protoc)
	fmt.Printf("before : %d\n", protov)
	for i := 0; i < protoc; i++ {
		adr := (*C.vorlage_proc_inputproto)(C.fuckyou(unsafe.Pointer(protov), C.fuckingfuckfuckgodamn(i), C.sizeof_vorlage_proc_inputproto))
		ret[i].name = C.GoString((*adr).name)
		ret[i].description = C.GoString((*adr).description)
	}
	return ret
}

// dlOpen tries to get a handle to a library (.so), attempting to access it
// by the names specified in libs and returning the first that is successfully
// opened. Callers are responsible for closing the handler. If no library can
// be successfully opened, an error is returned.
func dlOpen(libPath string) (*cProc, error) {
	libname := C.CString(libPath)
	defer C.free(unsafe.Pointer(libname))

	handle := C.dlopen(libname, C.RTLD_NOW)
	if handle == nil {
		e := C.dlerror()
		if e == nil {
			return nil, errors.New(0x82acb,
				"dlopen failed but dlerror did not return an error",
				nil,
				"I have no idea what to do.",
				libPath)

		}
		return nil, errors.NewCauseString(0x10baa,
			"failed to load in library",
			C.GoString(e),
			"make sure the library exists and it links properly",
			libPath)
	}
	h := &cProc{
		handle:  handle,
		libname: libPath,
	}
	return h, nil
}

func isInterfaceVersionSupported(ver uint32) bool {
	if ver != uint32(C.vorlage_proc_interfaceversion) {
		return false
	}
	return true
}

func (c *cProc) loadVorlageSymbols() error {
	theirVersion,err := c.getSymbolPointer("vorlage_proc_interfaceversion")
	if err != nil {
		return errors.Newf(0x7852b,
			"failed to find vorlage_proc_interfaceversion symbol",
			err,
			"make sure this is a valid vorlage processor and has been built correctly",
			"")
	}
	tv := (*uint32)(theirVersion)
	c.vorlageInterfaceVersion = *tv
	if !isInterfaceVersionSupported(*tv) {
		return errors.Newf(0x9852b,
			"vorlage processor interface version not supported",
			nil,
			"find a more up-to-date version of this processor or downgrade your vorlage",
			"version %x.8", *tv)
	}


	// make sure it has all the symbols we're interested with.
	var goodsyms = []struct{string;ptr *unsafe.Pointer} {
		{"vorlage_proc_startup", &c.vorlageStartup},
		{"vorlage_proc_onrequest", &c.vorlageOnRequest},
		{"vorlage_proc_define", &c.vorlageDefine},
		{"vorlage_proc_shutdown", &c.vorlageShutdown},
		{"test", &c.test},
	}

	for _,s := range goodsyms {
		p,err := c.getSymbolPointer(s.string)
		if err != nil {
			return errors.New(0xaab151,
				"could not find required symbol in library",
				err,
				"make sure you've implemented all functions found in processor-interface.h",
				s.string)
		}
		*s.ptr = p
	}
	return nil
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

















type Rid uint64

var validProcessorName = regexp.MustCompile(`^[a-z0-9]+$`)

type ProcessorInfo struct {
	// todo: I should probably make this private so I can make sure it loads in
	// via the filename.
	Name string

	Description string

	InputProto []InputPrototype
	StreamInputProto []InputPrototype

	// returns a list ProcessorVariable pointers (that all point to valid
	// memory).
	Variables []ProcessorVariable
}
type ProcessorVariable struct {
	Name        string
	Description string
	InputProto []InputPrototype
	StreamInputProto []InputPrototype
}
const (
	// General
	ActionCritical   = 0x1
	ActionAccessFail = 0xd

	// http only
	ActionHttpRedirect = 0x47790001
	ActionHttpCookie   = 0x47790002
)

type Action struct {
}

type ExitInfo struct {
}
type DefineInfo struct {
	*RequestInfo
	*ProcessorVariable
	Input
	StreamInput
}
// RequestInfo is sent to processors
type RequestInfo struct {
	*ProcessorInfo

	Filepath string

	Input
	StreamInput

	// Rid will be set by Compiler.Compile (will be globally unique)
	// treat it as read-only.
	Rid
}
type Processor interface {
	// called when loaded into the impl
	Startup() ProcessorInfo

	OnRequest(RequestInfo) []Action

	// Called multiple times (after PreProcess and before PostProcess).
	// rid will be the same used in preprocess and post process.
	// variable pointer will be equal to what was provided from Info().Variables.
	//DefineVariable(DefineInfo) Definition

	Shutdown() ExitInfo
}

// simply a list of variables
type InputPrototype struct {
	name string
	description string
}

// Input will be associtive to InputPrototype
type Input []string
type StreamInputPrototype []string
type StreamInput []io.Reader