package vorlage

import (
	"fmt"
	"io"
	"sync/atomic"
)

// everything we'd see in both doccomp-http and doccomp-cli and doccomp-pdf
type Compiler struct {

	// these two arrays are associative
	processors     []Processor
	processorInfos []ProcessorInfo

	// used for thread safety of shutdown.
	concurrentCompiles int64
}

type compileRequest struct {
	compiler       *Compiler
	filepath       string
	allInput       map[string]string
	allStreams     map[string]StreamInput
	actionsHandler ActionHandler
	rid            Rid

	// associative array with compiler.processors
	processorRInfos []RequestInfo
}

func (c compileRequest) String() string {
	var ret string
	var args []interface{}
	//rid
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
		args = append(args, fmt.Sprintf("processor[%s]", v.name))
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

func (p ProcessorInfo) String() string {
	ret := ""
	var args []interface{}
	//name
	ret += "\t%-28s: %s\n"
	args = append(args, "name")
	args = append(args, p.name)

	//description
	ret += "\t%-28s: %s\n"
	args = append(args, "description")
	args = append(args, p.Description)

	//inputs
	//if len(p.InputProto) == 0 {
	//	ret += "\tno input needed on request\n"
	//}
	printFormatInputProto(p.InputProto, "\t", "inputs", &ret, &args)
	//if len(p.StreamInputProto) == 0 {
	//	ret += "\tno streams needed on request\n"
	//}
	printFormatInputProto(p.StreamInputProto, "\t", "streams", &ret, &args)

	for _,v := range p.Variables {
		ret += "\t%-28s: %s\n"
		varprefix := fmt.Sprintf("variable[%s]", v.Name)
		args = append(args, varprefix)
		args = append(args, v.Description)
			//inputs
		//if len(p.InputProto) == 0 {
		//	ret += "\tno input needed on request\n"
		//}
		printFormatInputProto(v.InputProto, "\t" + varprefix, "input", &ret, &args)
		//if len(p.StreamInputProto) == 0 {
		//	ret += "\tno streams needed on request\n"
		//}
		printFormatInputProto(v.StreamInputProto, "\t" + varprefix, "stream", &ret, &args)
	}

	str := fmt.Sprintf(ret,args...)
		// remove ending newline
	if str[len(str)-1] == '\n' {
		str = str[0 : len(str)-1]
	}
	return str
}

func printFormatInputProto(p []InputPrototype, prefix string, ty string, ret *string, args *[]interface{}) {
	if len(p) == 0 {
		*ret += prefix + "no " + ty + " requested\n"
		return
	}
	for _,s := range p {
		*ret += "%s%-28s: %s\n"
		*args = append(*args, prefix)
		*args = append(*args, fmt.Sprintf("%s[%s]",ty, s.name))
		*args = append(*args, s.description)
	}
}

func NewCompiler(proc []Processor) (c Compiler, err error) {
	c.processors = proc

	// load all the infos
	c.processorInfos = make([]ProcessorInfo, len(proc))
	for i := range c.processors {
		c.processorInfos[i] = c.processors[i].Startup()
		err = c.processorInfos[i].Validate()
		if err != nil {
			return c, err
		}
		logger.Infof("loaded processor %s", c.processorInfos[i].name)
		logger.Debugf("%s information:\n%s", c.processorInfos[i].name, c.processorInfos[i])
	}

	return c, nil
}

func (info *ProcessorInfo) Validate() error {

	// name
	if !validProcessorName.MatchString(info.name) {
		cerr := NewError(errProcessorName)
		cerr.SetSubject(info.name)
		return cerr
	}

	// make sure stream and static don't have the same name.
	for _, v := range info.Variables {
		// make sure no statics are also streams
		for k := range v.InputProto {
			for j := range v.StreamInputProto {
				if v.InputProto[k].name == v.StreamInputProto[j].name {
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
func (comp *Compiler) Compile(filepath string, allInput map[string]string, allStreams map[string]StreamInput, actionsHandler ActionHandler) (docstream io.ReadCloser, err CompileStatus) {
	atomic.AddUint64(&nextRid, 1)
	atomic.AddInt64(&comp.concurrentCompiles, 1)
	defer atomic.AddInt64(&comp.concurrentCompiles, -1)
	compReq := compileRequest{
		compiler:        comp,
		filepath:        filepath,
		allInput:        allInput,
		allStreams:      allStreams,
		actionsHandler:  actionsHandler,
		rid:             Rid(atomic.LoadUint64(&nextRid)),
		processorRInfos: make([]RequestInfo, len(comp.processors)),
	}
	logger.Debugf("new request generated: %s", compReq)

	for i := range comp.processors {
		req := RequestInfo{}
		req.Filepath = filepath
		req.rid = compReq.rid
		req.cookie = new(interface{})
		req.Input = make([]string, len(comp.processorInfos[i].InputProto))
		req.StreamInput = make([]StreamInput, len(comp.processorInfos[i].StreamInputProto))
		// assigne the req fields so they match the processor's spec
		req.ProcessorInfo = &comp.processorInfos[i]
		// now the input...
		for inpti, inpt := range comp.processorInfos[i].InputProto {
			if str, ok := allInput[inpt.name]; ok {
				req.Input[inpti] = str
			} else {
				logger.Debugf("processor %s was given an empty %s", comp.processorInfos[i].name, inpt.name)
				req.Input[inpti] = ""
			}
		}
		for inpti, inpt := range comp.processorInfos[i].StreamInputProto {
			if stream, ok := allStreams[inpt.name]; ok {
				req.StreamInput[inpti] = stream
			} else {
				logger.Debugf("processor %s was given an empty stream %s", comp.processorInfos[i].name, inpt.name)
				req.StreamInput[inpti] = nil
			}
		}
		actions := comp.processors[i].OnRequest(req, req.cookie)
		for a := range actions {
			switch actions[a].Action {
			case ActionCritical:
				erro := NewError("processor had critical error")
				errz := NewError(string(actions[a].Data))
				erro.SetBecause(errz)
				erro.SetSubjectf("%s", comp.processorInfos[i].name)
				actionsHandler.ActionCritical(errz)
				return nil, CompileStatus{erro, true}
			case ActionAccessFail:
				erro := NewError("processor denied access")
				errz := NewError(string(actions[a].Data))
				erro.SetBecause(errz)
				erro.SetSubjectf("%s", comp.processorInfos[i].name)
				actionsHandler.ActionAccessFail(errz)
				return nil, CompileStatus{erro, true}
			case ActionSee:
				erro := NewError("processor redirect")
				path := string(actions[a].Data)
				erro.SetSubjectf("%s redirecting compRequest to %s", comp.processorInfos[i].name, path)
				actionsHandler.ActionSee(path)
				return nil, CompileStatus{erro, true}
			case ActionHTTPHeader:
				header := string(actions[a].Data)
				actionsHandler.ActionHTTPHeader(header)
			}
		}
		compReq.processorRInfos[i] = req
	}
	doc, errd := comp.loadDocument(compReq)
	if errd != nil {
		erro := NewError("loading a requested document")
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
