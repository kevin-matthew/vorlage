package vorlage

import (
	vorlageproc "ellem.so/vorlageproc"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sync"
	"sync/atomic"
)

var validProcessorName = regexp.MustCompile(`^[a-z0-9_\-]+$`)

// will also delete any processors in any list that have nil
func (c *Compiler) rebuildProcessors() (err error) {

	// completely rebuild the processor list
	newlist := make([]vorlageproc.Processor, 0, len(c.cprocessors)+len(c.goprocessors))
	newlistinfo := make([]vorlageproc.ProcessorInfo, 0, len(c.cprocessors)+len(c.goprocessors))

	// testarr will be a list of pointers to structures that implement vorlage.ProcessorInfo
	var testarr = make([]interface{}, 0, len(newlist))
	addProc := func(arr interface{}) {
		lenr := reflect.ValueOf(arr).Len()
		for i := 0; i < lenr; i++ {
			v := reflect.ValueOf(arr).Index(i)
			testarr = append(testarr, v.Interface())
		}
	}

	// arrptr must be a pointer to an array to pointers
	removenulls := func(arrptr interface{}) {
		arreml := reflect.ValueOf(arrptr).Elem()
		lenr := arreml.Len()
		replacement := reflect.MakeSlice(arreml.Type(), 0, lenr)
		for i := 0; i < lenr; i++ {
			v := arreml.Index(i)
			if !v.IsNil() {
				replacement = reflect.Append(replacement, v)
			}
		}
		t := replacement.Interface()
		a := arrptr
		c := reflect.ValueOf(arrptr).Elem()
		reflect.ValueOf(arrptr).Elem().Set(replacement)
		_ = a
		_ = c
		_ = t
	}

	// remove null values to clean up
	removenulls(&c.cprocessors)
	removenulls(&c.goprocessors)

	// copy each type of processor into testarr.
	// clang / shared object
	addProc(c.cprocessors)
	addProc(c.goprocessors)

	// find any processors that are no longer in testarr but remain in
	// c.processors, those need to be removed.
	for i := range c.processors {
		var j int
		for j = 0; j < len(testarr); j++ {
			ptr := reflect.ValueOf(c.processors[i]).Pointer()
			ptr2 := reflect.ValueOf(testarr[j]).Pointer()
			if ptr == ptr2 {
				// we still need this one
				break
			}
		}
		if j == len(testarr) {
			// this address in processors was not found in the upstream copies.
			// thus this processor is no longer needed. Shut it down.
			ptr := reflect.ValueOf(c.processors[i]).Pointer()
			Logger.Alertf("%s (@ %x) is no longer needed, shutting down", c.processorInfos[i].Name, ptr)
			err = c.processors[i].Shutdown()
			if err != nil {
				Logger.Alertf("error returned from shutdown.. this shouldn't happen as it will be ignored: %s", err)
			}
		}
	}

	// now carry over old processors and add new ones.
	for i := range testarr {
		// is this processor loaed in c.processors?
		var j int
		for j = 0; j < len(c.processors); j++ {
			ptr := reflect.ValueOf(c.processors[j]).Pointer()
			ptr2 := reflect.ValueOf(testarr[i]).Pointer()
			if ptr == ptr2 {
				// yup. loaded already carrie it over
				newlist = append(newlist, c.processors[j])
				newlistinfo = append(newlistinfo, c.processorInfos[j])
				break
			}
		}
		if j == len(c.processors) {
			// this processor's address was not found in c.processors, put it in
			newlist = append(newlist, testarr[i].(vorlageproc.Processor))
			info, err := startupproc(newlist[len(newlist)-1])
			if err != nil {
				return err
			}
			newlistinfo = append(newlistinfo, info)
		}
	}

	c.processors = newlist
	c.processorInfos = newlistinfo

	return nil
}

// everything we'd see in both doccomp-http and doccomp-cli and doccomp-pdf
type Compiler struct {

	// processors is technically a list of pointers to items found in cprocessors
	// and goprocessors.
	// If you want to change processors, you must update cprocessors / goprocessors
	// and then run updateProcessors()...
	// these two arrays are associative
	processors     []vorlageproc.Processor
	processorInfos []vorlageproc.ProcessorInfo

	// if you change these, you need to run rebuildProcessors to take effect.
	// if you set the pointers to nil, they will be marked for deletion.
	// if you change the address of the pointer, they will be marked for reload.
	cprocessors  []*cProc
	goprocessors []*goProc

	// access these via the atomic.Load... funcitons
	concurrentCompiles int64
	concurrentReaders  int32

	// set to anything but 0 to have Compile reject requests.
	// 1 = fully shutdown.
	// 2 = shutting down but waiting for other compilers to close
	// 3 = shutting down but waiting for other readers to close
	// 4 = new compiles are being stalled due to a restart/reload of a processor
	//     the stall will continue until unstall is fed something
	//
	// shutdownCompilers0 and shutdownReaders0 will be listened to
	// by shutdown when shutdown is in 2 and 3 states respectively. will be
	// written too. If shutdown not in process, they will be nil
	// todo: rename these.
	atomicShutdown     int32
	shutdownCompilers0 chan bool
	shutdownReaders0   chan bool
	unstall            sync.Mutex

	// used for watching go reloads if AutoReloadGoFiles
	gowatcher *watcher
}

type compileRequest struct {
	compiler       *Compiler
	filepath       string
	allInput       map[string]string
	allStreams     map[string]vorlageproc.StreamInput
	actionsHandler ActionHandler
	rid            vorlageproc.Rid

	// associative array with compiler.vorlageproc
	processorRInfos []vorlageproc.RequestInfo
}

func (c compileRequest) String() string {
	var ret string
	var args []interface{}
	//Rid
	ret += "request #%d:\n"
	args = append(args, c.rid)

	//path
	ret += "\t%-28s: %s\n"
	args = append(args, "filepath")
	args = append(args, c.filepath)

	if len(c.compiler.processorInfos) == 0 {
		ret += "\tno vorlageproc loaded\n"
	}
	for _, v := range c.compiler.processorInfos {
		ret += "\t%-28s: %s\n"
		args = append(args, fmt.Sprintf("processor[%s]", v.Name))
		args = append(args, v.Description)
	}

	// input
	if len(c.allInput) == 0 {
		ret += "\tno input provided\n"
	}
	for k, v := range c.allInput {
		ret += "\t%-28s: %s\n"
		args = append(args, fmt.Sprintf("input[%s]", k))
		args = append(args, v)
	}

	// stream input
	if len(c.allStreams) == 0 {
		ret += "\tno streams provided\n"
	}
	for k := range c.allStreams {
		ret += "\t%-28s: (stream)\n"
		args = append(args, fmt.Sprintf("streamed input[%s]", k))
	}
	str := fmt.Sprintf(ret, args...)
	// remove ending newline
	if str[len(str)-1] == '\n' {
		str = str[0 : len(str)-1]
	}

	return str
}

// see https://github.com/golang/go/issues/20461
var AutoReloadGoFiles bool = false

// will return an error if a processor failed to start and/or is invalid
func NewCompiler() (c *Compiler, err error) {

	// structure set up
	c = new(Compiler)

	// load the go processors
	c.goprocessors, err = loadGoProcessors(GoPluginLoadPath)
	if err != nil {
		return c, err
	}
	defer func() {
		if AutoReloadGoFiles {
			go c.watchGoPath(GoPluginLoadPath)
		}
	}()

	// load the c processors
	c.cprocessors, err = loadCProcessors(CLoadPath)
	if err != nil {
		return c, err
	}

	return c, c.rebuildProcessors()
}

// helper to rebuildProcessors
func startupproc(proc vorlageproc.Processor) (info vorlageproc.ProcessorInfo, err error) {
	ptr := reflect.ValueOf(proc).Pointer()
	info, err = proc.Startup()
	Logger.Debugf("starting %s (@ %x)...", info.Name, ptr)
	if err != nil {
		Logger.Alertf("processor %s (@ %x) failed to start: %s", info.Name, ptr, err)
		return info, err
	}
	err = validate(&(info))
	if err != nil {
		return info, err
	}
	Logger.Infof("successfully loaded processor %s (@ %x)", info.Name, ptr)
	Logger.Debugf("%s information:\n%s", info.Name, info)
	return info, err
}

func validate(info *vorlageproc.ProcessorInfo) error {

	// Name
	if !validProcessorName.MatchString(info.Name) {
		cerr := NewError(errProcessorName)
		cerr.SetSubject(info.Name)
		return cerr
	}

	// make sure stream and static don't have the same Name.
	for _, v := range info.Variables {
		// make sure no statics are also streams
		for k := range v.InputProto {
			for j := range v.StreamInputProto {
				if v.InputProto[k].Name == v.StreamInputProto[j].Name {
					oerr := NewError(errInputInStreamAndStatic)
					oerr.SetSubjectf("\"%s\"", k)
					return oerr
				}
			}
		}
	}

	return nil

}

// will be written to in all threads, and in all compilers.
var nextRid uint64 = 0

type CompileStatus struct {

	// if non-nil, compile failed. If nil, you can ignore this struct all
	// together
	Err error

	// if true, the error was because of a processor's action, meaning the
	// ActionHandler's relevant function would have been invoked.
	// If false (and Err is non-nil), then something happened when trying to
	// compile the document itself.
	WasProcessor bool
}

func (cs CompileStatus) Error() string {
	return cs.Err.Error()
}

type ActionHandler interface {

	// ActionCritical should tell the requestor that the compRequest cannot complete
	// due to a backend error as described by err.
	ActionCritical(err error)

	// ActionAccessFail should tell the requestor that they do not have
	// permission to view this file as described in err.
	ActionAccessFail(err error)

	// ActionSee should tell the requestor to make a new compRequest to this
	// other path.
	ActionSee(path string)

	// ActionHTTPHeader is relivent only to http server/requestors. If called,
	// you must add the header to the compRequest before reading from the compiled
	// document. If out of context of HTTP, leave this undefined.
	ActionHTTPHeader(header string)
}

/*
 * The best way to describe this function is by reading through the steps
 * defined in the 'Highlevel Process' chapter in the readme.
 * Be sure you've added the right vorlageproc via the Processors field
 * Will update req.Rid regardless.
 * Do not attempt to use the streams pointed to by req... they'll be read
 * when the docstream is read.
 */
func (comp *Compiler) Compile(filepath string, allInput map[string]string,
	allStreams map[string]vorlageproc.StreamInput, actionsHandler ActionHandler) (docstream io.ReadCloser, err CompileStatus) {

	for i := range comp.processors {
		Logger.Errorf("%v", comp.processors[i])
	}

	if shutdowncode := atomic.LoadInt32(&comp.atomicShutdown); shutdowncode != 0 {
		var erro error
		switch shutdowncode {
		case 1:
			erro = NewError("compiler has shutdown")
			return nil, CompileStatus{erro, false}
		case 2:
			erro = NewError("compiler is shutting down, waiting on other compiliations to finish")
			return nil, CompileStatus{erro, false}
		case 3:
			erro = NewError("compiler is shutting down, waiting on other readers to finish")
			return nil, CompileStatus{erro, false}
		case 4:
			// if 4, then we will try to lock unstall. which will lock this thread
			// until Compiler.cont is called
			comp.unstall.Lock()
			comp.unstall.Unlock()
		}
	}

	atomic.AddUint64(&nextRid, 1)
	atomic.AddInt64(&comp.concurrentCompiles, 1)
	defer func() {
		newi := atomic.AddInt64(&comp.concurrentCompiles, -1)
		shutdowncode := atomic.LoadInt32(&comp.atomicShutdown)
		if newi == 0 && shutdowncode == 2 {
			comp.shutdownCompilers0 <- true
		}
	}()
	compReq := compileRequest{
		compiler:        comp,
		filepath:        filepath,
		allInput:        allInput,
		allStreams:      allStreams,
		actionsHandler:  actionsHandler,
		rid:             vorlageproc.Rid(atomic.LoadUint64(&nextRid)),
		processorRInfos: make([]vorlageproc.RequestInfo, len(comp.processors)),
	}
	Logger.Debugf("new request generated: %s", compReq)

	for i := range comp.processors {
		req := vorlageproc.RequestInfo{}
		req.Filepath = filepath
		req.Rid = compReq.rid
		req.Cookie = new(interface{})
		req.Input = make([]string, len(comp.processorInfos[i].InputProto))
		req.StreamInput = make([]vorlageproc.StreamInput, len(comp.processorInfos[i].StreamInputProto))
		// assigne the req fields so they match the processor's spec
		req.ProcessorInfo = &comp.processorInfos[i]
		// now the input...
		for inpti, inpt := range comp.processorInfos[i].InputProto {
			if str, ok := allInput[inpt.Name]; ok {
				req.Input[inpti] = str
			} else {
				Logger.Debugf("processor %s was given an empty %s", comp.processorInfos[i].Name, inpt.Name)
				req.Input[inpti] = ""
			}
		}
		for inpti, inpt := range comp.processorInfos[i].StreamInputProto {
			if stream, ok := allStreams[inpt.Name]; ok {
				req.StreamInput[inpti] = stream
			} else {
				Logger.Debugf("processor %s was given an empty stream %s", comp.processorInfos[i].Name, inpt.Name)
				req.StreamInput[inpti] = nil
			}
		}
		actions := comp.processors[i].OnRequest(req, req.Cookie)
		for a := range actions {
			switch actions[a].Action {
			case vorlageproc.ActionCritical:
				erro := NewError("processor had critical error")
				errz := NewError(string(actions[a].Data.([]byte)))
				erro.SetBecause(errz)
				erro.SetSubjectf("%s", comp.processorInfos[i].Name)
				actionsHandler.ActionCritical(errz)
				return nil, CompileStatus{erro, true}
			case vorlageproc.ActionAccessFail:
				erro := NewError("processor denied access")
				errz := NewError(string(actions[a].Data.([]byte)))
				erro.SetBecause(errz)
				erro.SetSubjectf("%s", comp.processorInfos[i].Name)
				actionsHandler.ActionAccessFail(errz)
				return nil, CompileStatus{erro, true}
			case vorlageproc.ActionSee:
				erro := NewError("processor redirect")
				path := string(actions[a].Data.([]byte))
				erro.SetSubjectf("%s redirecting compRequest to %s", comp.processorInfos[i].Name, path)
				actionsHandler.ActionSee(path)
				return nil, CompileStatus{erro, true}
			case vorlageproc.ActionHTTPHeader:
				header := string(actions[a].Data.([]byte))
				actionsHandler.ActionHTTPHeader(header)
			case vorlageproc.ActionSet:
				// todo: this is weird compared to how the other actions are handled...
				//       maybe a design flaw... seeing how I rushed to get this action
				//       in here.
				Logger.Debugf("%s called setstream", comp.processorInfos[i].Name)
				header := actions[a].Data.(vorlageproc.SetStream)
				return header, CompileStatus{}
			}
		}
		compReq.processorRInfos[i] = req
	}
	doc, errd := comp.loadDocument(compReq)
	if errd != nil {
		erro := NewError("procload a requested document")
		erro.SetSubject(filepath)
		erro.SetBecause(errd)
		return docstream, CompileStatus{erro, false}
	}

	return doc, CompileStatus{}
}

/*
 * Returns all errors that occour when shutting down each processor.
 * If there is at least 1 Compile function that has not returned, Shutdown
 * will return an error
 */
func (comp *Compiler) Shutdown() {
	if comp.isshutdown() {
		return
	}
	if comp.gowatcher != nil {
		comp.gowatcher.close()
	}
	comp.makestall(1)

	// at this point, all readers and compilers are done.
	for i := range comp.processors {
		err := comp.processors[i].Shutdown()
		if err != nil {
			Logger.Alertf("error returned from shutdown.. this shouldn't happen as it will be ignored: %s", err)
		}
	}
}

func (comp *Compiler) isshutdown() bool {
	return atomic.LoadInt32(&comp.atomicShutdown) == 1
}

// will wait until all readers and compiles on all threads are complete.
// if code is 1, will cause a full shutdown.
// if code is 4, will stall all Compile calls until cont is called. While stalled,
// you can make changes to the processor
func (comp *Compiler) makestall(code int) {
	if comp.isshutdown() {
		return
	}
	comp.shutdownCompilers0 = make(chan bool)
	comp.shutdownReaders0 = make(chan bool)
	if code == 4 {
		Logger.Infof("blocking compiles")
		comp.unstall.Lock()
	}
	defer atomic.StoreInt32(&comp.atomicShutdown, int32(code))

	atomic.StoreInt32(&comp.atomicShutdown, 2)
	compiles := atomic.LoadInt64(&comp.concurrentCompiles)
	if compiles != 0 {
		Logger.Infof("waiting for %d compiles to complete...", compiles)
		<-comp.shutdownCompilers0
	}

	atomic.StoreInt32(&comp.atomicShutdown, 3)
	readers := atomic.LoadInt32(&comp.concurrentReaders)
	if readers != 0 {
		Logger.Infof("waiting for %d readers to close...", readers)
		<-comp.shutdownReaders0
	}

}

// will undo the set-limbo state that makestall made
func (comp *Compiler) cont() {
	Logger.Infof("unblocking compiles")
	atomic.StoreInt32(&comp.atomicShutdown, 0)
	comp.unstall.Unlock()
}
