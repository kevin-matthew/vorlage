package vorlage

// #cgo LDFLAGS: -ldl
// #include "c.src/processors.h"
// #include <stdlib.h>
// #include <dlfcn.h>
// #include <stdio.h>
// typedef void *(*voidfunc) (void *args);
// typedef vorlage_proc_info (*startupwrap)();
// vorlage_proc_info execstartupwrap(startupwrap f) {
//   return f();
// }
//typedef vorlage_proc_actions (*onrequestwrap)(vorlage_proc_requestinfo);
//vorlage_proc_actions execonrequest(onrequestwrap f, vorlage_proc_requestinfo r) {
//   return f(r);
//}
//
//
// // make sure to free the return array!!!
// const char **gostrings2cstrings(_GoString_ *s, int len) {
// }
//
import "C"
import (
	"unsafe"
)
import "../lmgo/errors"

type cProc struct {
	libname string
	handle  unsafe.Pointer

	vorlageInterfaceVersion uint32

	// function pointers
	vorlageStartup   unsafe.Pointer
	vorlageOnRequest unsafe.Pointer
	vorlageDefine    unsafe.Pointer
	vorlageShutdown  unsafe.Pointer

	// raw pointers
	volageProcInfo C.vorlage_proc_info
}

func (c *cProc) OnRequest(info RequestInfo) []Action {
	var reqinfo = C.vorlage_proc_requestinfo{}
	reqinfo.procinfo = &c.volageProcInfo
	reqinfo.filepath = C.CString(info.Filepath)
	defer C.free(unsafe.Pointer(reqinfo.filepath))
	reqinfo.rid = C.rid(info.Rid)

	inputVArray := make([]*C.char, len(info.Input))
	for i := range inputVArray {
		inputVArray[i] = C.CString(info.Input[i])
	}
	defer func() {
		for i := range inputVArray {
			C.free(unsafe.Pointer(inputVArray[i]))
		}
	}()
	inputStreamArray := make([]C.int, len(info.StreamInput))
	for i := range inputStreamArray {
		inputStreamArray[i] = C.int(info.StreamInput[i].Fd())
	}

	//reqinfo.inputv = C.gostrings2cstrings(&(info.Input[0]), len(info.Input))
	// todo: this may fuck it up
	if len(info.Input) > 0 {
		reqinfo.inputv = &(inputVArray[0])
	}
	if len(info.StreamInput) > 0 {
		reqinfo.streaminputv = &(inputStreamArray[0])
	}
	f := C.onrequestwrap(c.vorlageOnRequest)
	_ = C.execonrequest(f, reqinfo)
	return nil
}

func (c *cProc) DefineVariable(info DefineInfo) Definition {
	panic("implement me")
}

func (c *cProc) Shutdown() ExitInfo {
	panic("implement me")
}

func (c *cProc) Startup() ProcessorInfo {
	f := C.startupwrap(c.vorlageStartup)
	d := C.execstartupwrap(f)
	c.volageProcInfo = d
	p := ProcessorInfo{}
	// description
	p.Description = C.GoString(d.description)

	// input proto
	p.InputProto = parseInputProtoType(int(d.inputprotoc), d.inputprotov)
	p.StreamInputProto = parseInputProtoType(int(d.streaminputprotoc), d.streaminputprotov)
	p.Variables = parseVariables(int(d.variablesc), d.variablesv)
	return p
}
func parseVariables(varsc int, varsv *C.vorlage_proc_variable) []ProcessorVariable {
	if varsc == 0 {
		return nil
	}
	ret := make([]ProcessorVariable, varsc)
	slice := (*[1 << 28]C.vorlage_proc_variable)(unsafe.Pointer(varsv))[:varsc:varsc]
	for i := 0; i < varsc; i++ {
		iproto := slice[i]
		ret[i].Name = C.GoString(iproto.name)
		ret[i].Description = C.GoString(iproto.description)
		ret[i].InputProto = parseInputProtoType(int(iproto.inputprotoc), iproto.inputprotov)
		ret[i].StreamInputProto = parseInputProtoType(int(iproto.streaminputprotoc), iproto.streaminputprotov)
	}
	return ret
}
func parseInputProtoType(protoc int, protov *C.vorlage_proc_inputproto) []InputPrototype {
	if protoc == 0 {
		return nil
	}
	slice := (*[1 << 28]C.vorlage_proc_inputproto)(unsafe.Pointer(protov))[:protoc:protoc]
	ret := make([]InputPrototype, protoc)
	for i := 0; i < protoc; i++ {
		iproto := slice[i]
		ret[i].name = C.GoString(iproto.name)
		ret[i].description = C.GoString(iproto.description)
	}
	return ret
}

var _ Processor = &cProc{}

func LoadCProcessors(libpath string) (error, []Processor) {
	return nil, nil
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
	theirVersion, err := c.getSymbolPointer("vorlage_proc_interfaceversion")
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
	var goodsyms = []struct {
		string
		ptr *unsafe.Pointer
	}{
		{"vorlage_proc_startup", &c.vorlageStartup},
		{"vorlage_proc_onrequest", &c.vorlageOnRequest},
		{"vorlage_proc_define", &c.vorlageDefine},
		{"vorlage_proc_shutdown", &c.vorlageShutdown},
	}

	for _, s := range goodsyms {
		p, err := c.getSymbolPointer(s.string)
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
