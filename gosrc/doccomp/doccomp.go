package doccomp

/*
 * This is a definition, they can be made either by using '#define' in a file or
 * if the page processor
 */
type Definition interface {
	GetName() string
}

/*
 * A request is anything that consitutes a client asking for a compiled page.
 */
type Request interface {} // this will probably stay an interface.

/*
 * The best way to describe this function is by reading through the steps
 * defined in the 'Highlevel Process' chapter in the readme.
 */
func HandleRequest(request Request,
	pageProcessor PageProcessor) (docstream DocumentStream, err *Error) {

	// step 2
	doc := GetCached(request)
	if doc == nil {

		// step 3,4,5
		reqdoc,err := LoadRequestedDocument(request)
		if err != nil {
			erro := NewError("loading a requested document")
			erro.SetBecause(err)
			return docstream, erro
		}
		// step 6
		addToCache(reqdoc)
		doc = &reqdoc
	}

	// step 7
	err = pageProcessor.Preprocess(request)
	if err != nil {
		erro := NewError("in pre-processing")
		erro.SetBecause(err)
		return docstream, erro
	}

	// step 8
	definitions,err := pageProcessor.Process(request)
	if err != nil {
		erro := NewError("in processing")
		erro.SetBecause(err)
		return docstream, erro
	}

	for _,d := range definitions {
		doc.addDefinition(d)
	}

	// step 9
	docstream,err = doc.complete()
	if err != nil {
		erro := NewError("failed to open stream to document")
		erro.SetBecause(err)
		return docstream, erro
	}

	// step 10
	err = pageProcessor.Postprocess(request)
	if err != nil {
		erro := NewError("in post-processing")
		erro.SetBecause(err)
		return docstream, erro
	}

	// step 11
	if err != nil {
		erro := NewError("sending document")
		erro.SetBecause(err)
		return docstream, erro
	}
	return docstream,nil
}
