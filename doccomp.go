package vorlag

import (
	"io"
	"sync/atomic"
)

/*
 * This is a definition, they can be made either by using '#define' in a file or
 * if the page processor
 */
type Definition interface {
	// reset the reader to the beginning,
	// this is called before the every instance of the variable by the loader
	// Thus repetitions of large definitions should be advised against,
	// or at least have a sophisticated caching system.
	Reset() error

	// must return EOF when complete (no more bytes left to read)
	Read(p []byte) (int, error)

	// needed for content-length to be sent.
	// if nil is returned, doccomp will not calculate nor send content-length.
	// however this is not prefered and should be only used for applications
	// that truelly cannot know what their content length will be.
	//Length() *uint64
}

type Input map[string]string
type StreamInput map[string]io.Reader

// Request is all the information needed to be known to compiler a document
type Request struct {
	Filepath string

	// Input can be nil
	Input Input

	// StreamInput can be nil
	StreamInput StreamInput

	// Rid will be set by Compiler.Compile (will be globally unique)
	// treat it as read-only.
	Rid Rid
}

// everything we'd see in both doccomp-http and doccomp-cli and doccomp-pdf
type Compiler struct {

	// these two arrays are associative
	processors     []Processor
	processorInfos []ProcessorInfo
}

func NewCompiler(proc []Processor) (c Compiler, err error) {
	c.processors = proc

	// load all the infos
	c.processorInfos = make([]ProcessorInfo, len(proc))
	for i := range c.processors {
		c.processorInfos[i] = c.processors[i].Info()
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
		for k := range v.Input {
			if _, ok := v.StreamedInput[k]; ok {
				oerr := NewError(errInputInStreamAndStatic)
				oerr.SetSubjectf("\"%s\"", k)
				return oerr
			}
		}
	}

	return nil

}

// will be written to in all threads, and in all compilers.
var nextRid uint64 = 0

/*
 * The best way to describe this function is by reading through the steps
 * defined in the 'Highlevel Process' chapter in the readme.
 * Be sure you've added the right processors via the Processors field
 * Will update req.Rid regardless.
 * Do not attempt to use the streams pointed to by req... they'll be read
 * when the docstream is read.
 */
func (comp *Compiler) Compile(req *Request) (docstream io.ReadCloser, err error) {
	atomic.AddUint64(&nextRid, 1)
	req.Rid = Rid(nextRid)
	doc, errd := comp.loadDocument(*req)
	if errd != nil {
		erro := NewError("loading a requested document")
		erro.SetSubject(req.Filepath)
		erro.SetBecause(errd)
		return docstream, erro
	}

	return &doc, nil
}
