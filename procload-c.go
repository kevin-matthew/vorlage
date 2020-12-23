package vorlage

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"unsafe"
)

// #cgo LDFLAGS: -ldl
// #include "vorlage-interface/shared-library/processors.h"
// #include <string.h>
// #include <stdlib.h>
// #include <dlfcn.h>
// #include <stdio.h>
// typedef void *(*voidfunc) (void *args);
// typedef vorlage_proc_info (*startupwrap)();
// vorlage_proc_info execstartupwrap(startupwrap f) {
//   return f();
// }
//typedef vorlage_proc_actions (*onrequestwrap)(vorlage_proc_requestinfo, void**);
//vorlage_proc_actions execonrequest(onrequestwrap f, vorlage_proc_requestinfo r, void** c) {
//   return f(r,c);
//}
//typedef void* (*definewrap)(vorlage_proc_defineinfo, void*);
//void* execdefine(definewrap f, vorlage_proc_defineinfo r, void* c) {
// return f(r,c);
//}
//typedef void (*vorlage_proc_onfinish_wrap)(vorlage_proc_requestinfo, void*);
//void vorlage_proc_onfinish_exec(vorlage_proc_onfinish_wrap f, vorlage_proc_requestinfo r, void *c) {
// return f(r,c);
//}
//typedef int (*vorlage_proc_shutdown_wrap)();
//int vorlage_proc_shutdown_exec(vorlage_proc_shutdown_wrap f) {
// // todo: send context
// return f();
//}
// typedef int (*vorlage_proc_definer_read_wrap)(void *definer, char *buf, size_t size);
// int vorlage_proc_definer_read_exec(vorlage_proc_definer_read_wrap f, void *definer, void *buf, size_t size) {
// return f(definer, buf, size);
// }
// typedef int (*vorlage_proc_definer_close_wrap)(void *definer);
// int vorlage_proc_definer_close_exec(vorlage_proc_definer_close_wrap f, void *definer) {
// return f(definer);
// }
// typedef size_t (*vorlage_proc_definer_reset_wrap)(void *definer);
// size_t vorlage_proc_definer_reset_exec(vorlage_proc_definer_reset_wrap f, void *definer) {
// return f(definer);
// }
//
// char **mallocPointerArray(int len) {
// return (char **)(malloc(sizeof(char *) * len));
// }
import "C"
import (
	"io"
	"strconv"
)
import "./lmgo/errors"
import "./vorlage-interface/golang/vorlageproc"

type cProc struct {
	libname  string
	procname string
	handle   unsafe.Pointer

	vorlageInterfaceVersion uint32

	// function pointers
	vorlageStartup             unsafe.Pointer
	vorlageOnRequest           unsafe.Pointer
	vorlageDefine              unsafe.Pointer
	vorlage_proc_onfinish      unsafe.Pointer
	vorlageShutdown            unsafe.Pointer
	vorlage_proc_definer_read  unsafe.Pointer
	vorlage_proc_definer_close unsafe.Pointer
	vorlage_proc_definer_reset unsafe.Pointer

	// raw pointers
	volageProcInfo C.vorlage_proc_info
}
var _ vorlageproc.Processor = &cProc{}

func requestInfoToCRinfo(info vorlageproc.RequestInfo, procinfo C.vorlage_proc_info) *C.vorlage_proc_requestinfo {

	// make a reqinfo struct in c memeory
	var reqinfo = (*C.vorlage_proc_requestinfo)(C.malloc(C.sizeof_vorlage_proc_requestinfo))
	// passin the procinfo that C had returned previously
	reqinfo.procinfo = procinfo
	// copy in file path
	reqinfo.filepath = C.CString(info.Filepath)
	// copy in rid
	reqinfo.rid = C.rid(info.Rid)

	// now put the input information into the request.
	reqinfo.inputv = inputToCInput(info.Input)
	reqinfo.streaminputv = streaminputToCInput(info.StreamInput)

	// done
	return reqinfo
}
func streaminputToCInput(streams []vorlageproc.StreamInput) *unsafe.Pointer {
	if len(streams) == 0 {
		return nil
	}
	var p *int
	streamVArrayPtr := (*unsafe.Pointer)(C.malloc(C.ulong(unsafe.Sizeof(p)) * C.ulong(len(streams))))
	streamVArray := (*[1 << 28]unsafe.Pointer)(unsafe.Pointer(streamVArrayPtr))[:len(streams):len(streams)]
	for i := range streamVArray {
		streamVArray[i] = unsafe.Pointer(createCDescriptor(streams[i]))
	}
	return streamVArrayPtr
}
func freeCStreamInput(streaminputs *unsafe.Pointer, c C.int) {
	if int(c) == 0 {
		return
	}
	inputVArray := (*[1 << 28]unsafe.Pointer)(unsafe.Pointer(streaminputs))[:c:c]
	for i := range inputVArray {
		deleteCDescriptor((*C.int)(inputVArray[i]))
	}
	C.free(unsafe.Pointer(streaminputs))
}
func inputToCInput(input []string) **C.char {
	if len(input) == 0 {
		return nil
	}
	// normal (must be freed)
	var p *byte
	inputVArrayPtr := (**C.char)(C.malloc(C.ulong(unsafe.Sizeof(p)) * C.ulong(len(input))))
	inputVArray := (*[1 << 28]*C.char)(unsafe.Pointer(inputVArrayPtr))[:len(input):len(input)]
	for i := range inputVArray {
		inputVArray[i] = C.CString(input[i])
	}

	return inputVArrayPtr
}
func freeCInput(input **C.char, inputc C.int) {
	if int(inputc) == 0 {
		return
	}
	inputVArray := (*[1 << 28]*C.char)(unsafe.Pointer(input))[:inputc:inputc]
	for i := range inputVArray {
		C.free(unsafe.Pointer(inputVArray[i]))
	}
	C.free(unsafe.Pointer(input))
}
func freeCRinfo(info *C.vorlage_proc_requestinfo) {
	C.free(unsafe.Pointer(info.filepath))
	freeCInput(info.inputv, info.procinfo.inputprotoc)
	freeCStreamInput(info.streaminputv, info.procinfo.streaminputprotoc)
	C.free(unsafe.Pointer(info))
}
type requestContext struct {
	rinfoInCMemory   *C.vorlage_proc_requestinfo
	contextInCMemory unsafe.Pointer
}
func (c *cProc) OnRequest(info vorlageproc.RequestInfo, context *interface{}) []vorlageproc.Action {
	var reqinfo = requestInfoToCRinfo(info, c.volageProcInfo)
	// exec the function and prepare the return in gostyle.
	var ccontext unsafe.Pointer

	f := C.onrequestwrap(c.vorlageOnRequest)
	cactions := C.execonrequest(f, *reqinfo, &ccontext)
	cactionsslice := (*[1 << 28]C.vorlage_proc_action)(unsafe.Pointer(cactions.actionv))[:cactions.actionc:cactions.actionc]

	ret := make([]vorlageproc.Action, len(cactionsslice))
	for i := range cactionsslice {
		ret[i].Action = int(cactionsslice[i].action)
		ret[i].Data = C.GoBytes(cactionsslice[i].data, cactionsslice[i].datac)
	}
	*context = requestContext{reqinfo, ccontext}
	return ret
}

// must be *FILE
type descriptorReader struct {
	c   *cProc
	ptr unsafe.Pointer
}

func (d descriptorReader) Close() error {
	f := C.vorlage_proc_definer_close_wrap(d.c.vorlage_proc_definer_close)
	returnCode := int(C.vorlage_proc_definer_close_exec(f, d.ptr))
	if returnCode != 0 {
		return errors.NewCauseString(0x983452b,
			"failed to close definer",
			"error code "+strconv.Itoa(returnCode),
			"",
			"")
	}
	return nil
}
func (d descriptorReader) Reset() error {
	f := C.vorlage_proc_definer_reset_wrap(d.c.vorlage_proc_definer_reset)
	returnCode := int(C.vorlage_proc_definer_reset_exec(f, d.ptr))
	if returnCode != 0 {
		return errors.NewCauseString(0x983452a,
			"failed to reset definer",
			"error code "+strconv.Itoa(returnCode),
			"",
			"")
	}
	return nil
}
func (d descriptorReader) Read(p []byte) (int, error) {
	f := C.vorlage_proc_definer_read_wrap(d.c.vorlage_proc_definer_read)
	size := int(C.vorlage_proc_definer_read_exec(f, d.ptr, unsafe.Pointer(&(p[0])), C.size_t(len(p))))
	if size < 0 {
		if size == -2 {
			return 0, io.EOF
		}
		return 0, errors.NewCauseString(0x983452c,
			"failed to read",
			fmt.Sprintf("%d", size),
			"",
			"")
	}
	return int(size), nil
}
func (c *cProc) DefineVariable(info vorlageproc.DefineInfo, context interface{}) vorlageproc.Definition {
	var reqinfoContext = (context).(requestContext)
	reqinfo := reqinfoContext.rinfoInCMemory
	//requestInfoToCRinfo(*info.RequestInfo, &c.volageProcInfo)
	var d C.vorlage_proc_defineinfo
	d.requestinfo = reqinfo
	d.procvarindex = C.int(info.ProcVarIndex)
	inputv := inputToCInput(info.Input)
	streaminputv := streaminputToCInput(info.StreamInput)
	d.inputv = inputv
	d.streaminputv = streaminputv
	defer freeCInput(d.inputv, C.int(len(info.RequestInfo.ProcessorInfo.Variables[info.ProcVarIndex].InputProto)))
	defer freeCStreamInput(d.streaminputv, C.int(len(info.RequestInfo.ProcessorInfo.Variables[info.ProcVarIndex].StreamInputProto)))
	f := C.definewrap(c.vorlageDefine)
	filedes := C.execdefine(f, d, reqinfoContext.contextInCMemory)
	return descriptorReader{c, unsafe.Pointer(filedes)}
}
func (c *cProc) OnFinish(rinfo vorlageproc.RequestInfo, context interface{}) {
	var reqinfoContext = (context).(requestContext)
	var reqinfo = reqinfoContext.rinfoInCMemory
	defer freeCRinfo(reqinfo)
	f := C.vorlage_proc_onfinish_wrap(c.vorlage_proc_onfinish)
	C.vorlage_proc_onfinish_exec(f, *reqinfo, reqinfoContext.contextInCMemory)
}
func (c *cProc) Startup() vorlageproc.ProcessorInfo {
	f := C.startupwrap(c.vorlageStartup)
	d := C.execstartupwrap(f)
	c.volageProcInfo = d
	p := vorlageproc.ProcessorInfo{}
	// description
	p.Name = c.procname
	p.Description = C.GoString(d.description)

	// input proto
	p.InputProto = parseInputProtoType(int(d.inputprotoc), d.inputprotov)
	p.StreamInputProto = parseInputProtoType(int(d.streaminputprotoc), d.streaminputprotov)
	p.Variables = parseVariables(int(d.variablesc), d.variablesv)
	return p
}
func parseVariables(varsc int, varsv *C.vorlage_proc_variable) []vorlageproc.ProcessorVariable {
	if varsc == 0 {
		return nil
	}
	ret := make([]vorlageproc.ProcessorVariable, varsc)
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
func parseInputProtoType(protoc int, protov *C.vorlage_proc_inputproto) []vorlageproc.InputPrototype {
	if protoc == 0 {
		return nil
	}
	slice := (*[1 << 28]C.vorlage_proc_inputproto)(unsafe.Pointer(protov))[:protoc:protoc]
	ret := make([]vorlageproc.InputPrototype, protoc)
	for i := 0; i < protoc; i++ {
		iproto := slice[i]
		ret[i].Name = C.GoString(iproto.name)
		ret[i].Description = C.GoString(iproto.description)
	}
	return ret
}


var libraryFilenameSig = regexp.MustCompile("^lib([^.]+).so")

func LoadCProcessors() ([]vorlageproc.Processor, error) {
	var procs []vorlageproc.Processor
	files, err := ioutil.ReadDir(CLoadPath)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		libnames := libraryFilenameSig.FindStringSubmatch(f.Name())
		if libnames == nil {
			continue
		}
		path := CLoadPath + "/" + f.Name()
		if CLoadPath == "" {
			path = f.Name()
		}
		proc, err := dlOpen(path)
		if err != nil {
			return procs, errors.Newf(0x6134bc1,
				"failed to load library from library path",
				err,
				"",
				"%s", path)
		}
		err = proc.loadVorlageSymbols()
		if err != nil {
			return procs, errors.Newf(0x6134bc2,
				"failed to load library from library path",
				err,
				"",
				"%s", path)
		}
		proc.procname = libnames[1]
		procs = append(procs, proc)
		Logger.Debugf("loaded c processor %s from %s", proc.procname, f.Name())
	}
	return procs, nil
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
			"make sure this is a valid vorlageproc processor and has been built correctly",
			"")
	}
	tv := (*uint32)(theirVersion)
	c.vorlageInterfaceVersion = *tv
	if !isInterfaceVersionSupported(*tv) {
		return errors.Newf(0x9852b,
			"vorlageproc processor interface version not supported",
			nil,
			"find a more up-to-date version of this processor or downgrade your vorlageproc",
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
		{"vorlage_proc_onfinish", &c.vorlage_proc_onfinish},
		{"vorlage_proc_shutdown", &c.vorlageShutdown},
		{"vorlage_proc_definer_close", &c.vorlage_proc_definer_close},
		{"vorlage_proc_definer_read", &c.vorlage_proc_definer_read},
		{"vorlage_proc_definer_reset", &c.vorlage_proc_definer_reset},
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
func (c *cProc) Shutdown() error {
	f := C.vorlage_proc_shutdown_wrap(c.vorlageShutdown)
	ret := int(C.vorlage_proc_shutdown_exec(f))
	if ret != 0 {
		Logger.Errorf("processor shutdown return non-0 exit code (%d)", ret)
	}

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
