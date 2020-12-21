package compiler

import (
	"fmt"
	"io"
	"regexp"
	"sync/atomic"
	".."
)

var validProcessorName = regexp.MustCompile(`^[a-z0-9_\-]+$`)



// everything we'd see in both doccomp-http and doccomp-cli and doccomp-pdf
type Compiler struct {

	// these two arrays are associative
	processors     []vorlage.Processor
	processorInfos []vorlage.ProcessorInfo

	// used for thread safety of shutdown.
	concurrentCompiles int64
}

type compileRequest struct {
	compiler       *Compiler
	filepath       string
	allInput       map[string]string
	allStreams     map[string]vorlage.StreamInput
	actionsHandler ActionHandler
	rid            vorlage.Rid

	// associative array with compiler.processors
	processorRInfos []vorlage.RequestInfo
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
		ret += "\tno processors loaded\n"
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


func NewCompiler(proc []vorlage.Processor) (c Compiler, err error) {
	c.processors = proc

	// load all the infos
	c.processorInfos = make([]vorlage.ProcessorInfo, len(proc))
	for i := range c.processors {
		c.processorInfos[i] = c.processors[i].Startup()
		err = validate(&(c.processorInfos[i]))
		if err != nil {
			return c, err
		}
		vorlage.Logger.Infof("loaded processor %s", c.processorInfos[i].Name)
		vorlage.Logger.Debugf("%s information:\n%s", c.processorInfos[i].Name, c.processorInfos[i])
	}

	return c, nil
}

func validate(info *vorlage.ProcessorInfo) error {

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
 * Be sure you've added the right processors via the Processors field
 * Will update req.Rid regardless.
 * Do not attempt to use the streams pointed to by req... they'll be read
 * when the docstream is read.
 */
func (comp *Compiler) Compile(filepath string, allInput map[string]string, allStreams map[string]vorlage.StreamInput, actionsHandler ActionHandler) (docstream io.ReadCloser, err CompileStatus) {
	atomic.AddUint64(&nextRid, 1)
	atomic.AddInt64(&comp.concurrentCompiles, 1)
	defer atomic.AddInt64(&comp.concurrentCompiles, -1)
	compReq := compileRequest{
		compiler:        comp,
		filepath:        filepath,
		allInput:        allInput,
		allStreams:      allStreams,
		actionsHandler:  actionsHandler,
		rid:             vorlage.Rid(atomic.LoadUint64(&nextRid)),
		processorRInfos: make([]vorlage.RequestInfo, len(comp.processors)),
	}
	vorlage.Logger.Debugf("new request generated: %s", compReq)

	for i := range comp.processors {
		req := vorlage.RequestInfo{}
		req.Filepath = filepath
		req.Rid = compReq.rid
		req.Cookie = new(interface{})
		req.Input = make([]string, len(comp.processorInfos[i].InputProto))
		req.StreamInput = make([]vorlage.StreamInput, len(comp.processorInfos[i].StreamInputProto))
		// assigne the req fields so they match the processor's spec
		req.ProcessorInfo = &comp.processorInfos[i]
		// now the input...
		for inpti, inpt := range comp.processorInfos[i].InputProto {
			if str, ok := allInput[inpt.Name]; ok {
				req.Input[inpti] = str
			} else {
				vorlage.Logger.Debugf("processor %s was given an empty %s", comp.processorInfos[i].Name, inpt.Name)
				req.Input[inpti] = ""
			}
		}
		for inpti, inpt := range comp.processorInfos[i].StreamInputProto {
			if stream, ok := allStreams[inpt.Name]; ok {
				req.StreamInput[inpti] = stream
			} else {
				vorlage.Logger.Debugf("processor %s was given an empty stream %s", comp.processorInfos[i].Name, inpt.Name)
				req.StreamInput[inpti] = nil
			}
		}
		actions := comp.processors[i].OnRequest(req, req.Cookie)
		for a := range actions {
			switch actions[a].Action {
			case vorlage.ActionCritical:
				erro := NewError("processor had critical error")
				errz := NewError(string(actions[a].Data))
				erro.SetBecause(errz)
				erro.SetSubjectf("%s", comp.processorInfos[i].Name)
				actionsHandler.ActionCritical(errz)
				return nil, CompileStatus{erro, true}
			case vorlage.ActionAccessFail:
				erro := NewError("processor denied access")
				errz := NewError(string(actions[a].Data))
				erro.SetBecause(errz)
				erro.SetSubjectf("%s", comp.processorInfos[i].Name)
				actionsHandler.ActionAccessFail(errz)
				return nil, CompileStatus{erro, true}
			case vorlage.ActionSee:
				erro := NewError("processor redirect")
				path := string(actions[a].Data)
				erro.SetSubjectf("%s redirecting compRequest to %s", comp.processorInfos[i].Name, path)
				actionsHandler.ActionSee(path)
				return nil, CompileStatus{erro, true}
			case vorlage.ActionHTTPHeader:
				header := string(actions[a].Data)
				actionsHandler.ActionHTTPHeader(header)
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

	return &doc, CompileStatus{}
}

/*
 * Returns all errors that occour when shutting down each processor.
 * If there is at least 1 Compile function that has not returned, Shutdown
 * will return an error
 */
func (comp *Compiler) Shutdown() []error {
	compiles := atomic.LoadInt64(&comp.concurrentCompiles)
	if compiles != 0 {
		erro := NewError("compiles still running")
		erro.SetSubjectf("%d compile compRequest still processing", compiles)
		return []error{erro}
	}
	var ret []error
	for i := range comp.processors {
		err := comp.processors[i].Shutdown()
		if err != nil {
			ret = append(ret, err)
		}
	}
	return ret
}
