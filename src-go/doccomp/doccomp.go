package doccomp

import (
	"io"
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
	Length() *uint64

	// returns the fulle variable name ie '$(myvar)'
	GetFullName() string
}

/*
 * A request is anything that consitutes a client asking for a compiled page.
 */
type Request interface {
	GetFilePath() string
} // this will probably stay an interface.

type Compiler struct {
	cache Cache
}

/*
 * The best way to describe this function is by reading through the steps
 * defined in the 'Highlevel Process' chapter in the readme.
 * Be sure you've added the right processors via the Processors field
 */
func Process(filepath string, input map[string]string, streamInput map[string]io.Reader) (docstream io.ReadCloser, err error) {
	var reqdoc *Document
	//step 2
	//step 3,4
	doc, errd := LoadDocument(filepath, input, streamInput)
	if errd != nil {
		erro := NewError("loading a requested document")
		erro.SetSubject(filepath)
		erro.SetBecause(errd)
		return docstream, erro
	}

	//step 5,6
	/*verbosef("storing '%s' in cache", request.GetFilePath())
	cerr := cache.AddToCache(doc)
	if cerr != nil {
		erro := NewError("adding a document to the cache")
		erro.SetSubject(request.GetFilePath())
		erro.SetBecause(NewError(cerr.Error()))
		doc.Close()
		return docstream, erro
	}*/

	reqdoc = &doc

	// step 7
	return reqdoc, nil

	/*

		definitions, err := pageProcessor.Process(request)
		if err != nil {
			erro := NewError("in processing")
			erro.SetBecause(err)
			reqdoc.Close()
			return docstream, erro
		}

		for _, d := range definitions {
			reqdoc.AddDefinition(d)
		}
		docstream = reqdoc

		// step 9
		verbosef("post-processing document '%s'", request.GetFilePath())
		err = pageProcessor.Postprocess(request)
		if err != nil {
			erro := NewError("in post-processing")
			erro.SetBecause(err)
			reqdoc.Close()
			return docstream, erro
		}

		// step 10,11
		docstream = reqdoc
		return docstream, nil*/
}
