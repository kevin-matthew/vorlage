package doccomp

import (
	"io"
)

const reservedPrefix = "__"

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

// todo: put this as a reciver for process
type Compiler struct {
	cache Cache
}

/*
 * The best way to describe this function is by reading through the steps
 * defined in the 'Highlevel Process' chapter in the readme.
 * Be sure you've added the right processors via the Processors field
 */
func Process(filepath string,
	reservedInput map[string]string,
	input Input,
	streamInput StreamInput) (docstream io.ReadCloser, err error) {
	var reqdoc *Document

	// prepare reserved inptu
	// todo: Process need to be a function based on a reciver like 'request'
	// or something. that way I can do this logic only once and not every
	// request.
	for k, _ := range input {
		if len(k) >= len(reservedPrefix) &&
			k[:len(reservedPrefix)] == reservedPrefix {
			logger.Infof("input variable cannot start with " + reservedPrefix + " (" + k + "), ignoring")
			delete(input, k)
		}
	}
	for k, _ := range streamInput {
		if len(k) >= len(reservedPrefix) &&
			k[:len(reservedPrefix)] == reservedPrefix {
			logger.Infof("input variable cannot start with " + reservedPrefix + " (" + k + "), ignoring")
			delete(streamInput, k)
		}
	}
	if input == nil {
		input = make(map[string]string, len(reservedInput))
	}
	for k, v := range reservedInput {
		if len(k) <= len(reservedPrefix) ||
			k[:len(reservedPrefix)] != reservedPrefix {
			cerr := NewError(errBadReservedInput)
			cerr.SetSubject(k)
			return nil, cerr
		}
		input[k] = v
	}

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
