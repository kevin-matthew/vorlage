package doccomp

import "io"

/*
 * This is a definition, they can be made either by using '#define' in a file or
 * if the page processor
 */
type Definition interface {

	// must return 0,EOF when complete.
	Read(p []byte) (int, error)

	// ie: '$myvar'
	GetName() string
}

/*
 * A request is anything that consitutes a client asking for a compiled page.
 */
type Request interface {
	GetFilePath() string
} // this will probably stay an interface.

/*
 * The best way to describe this function is by reading through the steps
 * defined in the 'Highlevel Process' chapter in the readme.
 */
func HandleRequest(request Request,
	cache Cache,
	pageProcessor PageProcessor) (docstream io.ReadCloser, err *Error) {

	shouldCache, cerr := cache.ShouldCache(request.GetFilePath())
	if cerr != nil {
		erro := NewError("querying should cache")
		erro.SetSubject(request.GetFilePath())
		erro.SetBecause(NewError(cerr.Error()))
		return docstream, erro
	}

	var reqdoc *Document
	//step 2
	if shouldCache {
		//step 3,4
		doc, err := LoadDocument(request.GetFilePath())
		if err != nil {
			erro := NewError("loading a requested document")
			erro.SetSubject(request.GetFilePath())
			erro.SetBecause(err)
			return docstream, erro
		}

		//step 5,6
		Debugf("storing '%s' in cache", request.GetFilePath())
		cerr := cache.AddToCache(doc)
		if cerr != nil {
			erro := NewError("adding a document to the cache")
			erro.SetSubject(request.GetFilePath())
			erro.SetBecause(NewError(cerr.Error()))
			doc.Close()
			return docstream, erro
		}

		reqdoc = &doc
	} else {
		Debugf("pulling '%s' from cache", request.GetFilePath())
		reqdoc, cerr = cache.GetFromCache(request.GetFilePath())
		if cerr != nil {
			erro := NewError("adding a document to the cache")
			erro.SetSubject(request.GetFilePath())
			erro.SetBecause(NewError(cerr.Error()))
			return docstream, erro
		}
	}

	// step 7
	Debugf("pre-processing document '%s'", request.GetFilePath())
	err = pageProcessor.Preprocess(request)
	if err != nil {
		erro := NewError("in pre-processing")
		erro.SetBecause(err)
		reqdoc.Close()
		return docstream, erro
	}

	// step 8
	Debugf("processing document '%s'", request.GetFilePath())
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
	Debugf("post-processing document '%s'", request.GetFilePath())
	err = pageProcessor.Postprocess(request)
	if err != nil {
		erro := NewError("in post-processing")
		erro.SetBecause(err)
		reqdoc.Close()
		return docstream, erro
	}

	// step 10,11
	docstream = reqdoc
	return docstream, nil
}
