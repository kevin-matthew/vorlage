package vorlage

import (
	"io"
	"sync/atomic"
)





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
			if _, ok := v.StreamInputProto[k]; ok {
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
func (comp *Compiler) Compile(req *RequestInfo) (docstream io.ReadCloser, err error) {
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
