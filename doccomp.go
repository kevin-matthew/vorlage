package vorlage

import (
	"io"
	"sync/atomic"
)

// everything we'd see in both doccomp-http and doccomp-cli and doccomp-pdf
type Compiler struct {

	// these two arrays are associative
	processors         []Processor
	processorInfos     []ProcessorInfo
	concurrentCompiles int64
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
		logger.Infof("new compiler: loaded processor %s - %s",
			c.processorInfos[i].Name,
			c.processorInfos[i].Description)
	}

	return c, nil
}

func (info *ProcessorInfo) Validate() error {

	// name
	if !validProcessorName.MatchString(info.Name) {
		cerr := NewError(errProcessorName)
		cerr.SetSubject(info.Name)
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

type ActionHandler interface {

	// ActionCritical should tell the requestor that the request cannot complete
	// due to a backend error as described by err.
	ActionCritical(err error)

	// ActionAccessFail should tell the requestor that they do not have
	// permission to view this file as described in err.
	ActionAccessFail(err error)

	// ActionSee should tell the requestor to make a new request to this
	// other path.
	ActionSee(path string)

	// ActionHTTPHeader is relivent only to http server/requestors. If called,
	// you must add the header to the request before reading from the compiled
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
func (comp *Compiler) Compile(req *RequestInfo, actionsHandler ActionHandler) (docstream io.ReadCloser, err CompileStatus) {
	atomic.AddUint64(&nextRid, 1)
	atomic.AddInt64(&comp.concurrentCompiles, 1)
	defer atomic.AddInt64(&comp.concurrentCompiles, -1)
	req.rid = Rid(atomic.LoadUint64(&nextRid))
	req.cookie = new(interface{})
	for i := range comp.processors {
		actions := comp.processors[i].OnRequest(*req, req.cookie)
		for a := range actions {
			switch actions[a].Action {
			// General
			case ActionCritical:
				erro := NewError("processor had critical error")
				errz := NewError(string(actions[a].Data))
				erro.SetBecause(errz)
				erro.SetSubjectf("%s", comp.processorInfos[i].Name)
				actionsHandler.ActionCritical(errz)
				return nil, CompileStatus{erro, true}

			case ActionAccessFail:
				erro := NewError("processor denied access")
				errz := NewError(string(actions[a].Data))
				erro.SetBecause(errz)
				erro.SetSubjectf("%s", comp.processorInfos[i].Name)
				actionsHandler.ActionAccessFail(errz)
				return nil, CompileStatus{erro, true}

			case ActionSee:
				erro := NewError("processor redirect")
				path := string(actions[a].Data)
				erro.SetSubjectf("%s redirecting request to %s", comp.processorInfos[i].Name, path)
				actionsHandler.ActionSee(path)
				return nil, CompileStatus{erro, true}

			case ActionHTTPHeader:
				header := string(actions[a].Data)
				actionsHandler.ActionHTTPHeader(header)
			}
		}
	}
	doc, errd := comp.loadDocument(*req)
	if errd != nil {
		erro := NewError("loading a requested document")
		erro.SetSubject(req.Filepath)
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
		erro.SetSubjectf("%d compile request still processing", compiles)
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
