package doccomp

import (
	"fmt"
	"io"
)

type Cache interface {

	/*
	 * this is asked every request. If true is returned, a call to AddToCache
	 * will follow. If false is returned, a call to GetFromCache will follow.
	 * On error, neither is called.
	 */
	ShouldCache(path string) (bool, error)

	/*
	 * add a document to the cache. it should be able to be indexed by using
	 * it's path from d.GetFilePath
	 */
	AddToCache(d Document) error

	/*
	 * Load the document from the cache by using its path.
	 */
	GetFromCache(path string) (io.ReadCloser, error)
}

type variablePos struct {
	fullName     string
	variableName string // this will be the Processor-Variable Name if
	// processorName is not ""
	processorName string // if "" then it is not a processed variable
	charPos       int64
	length        uint
	linenum       uint // used for debugging
	colnum        uint // used for debugging
}

func (v variablePos) ToString() string {
	return fmt.Sprintf("'%s', line %d, col %d", v.fullName, v.linenum, v.colnum)
}

type CachedDocument struct {
	missingDefs    []variablePos
	path           string   // could also be memoery
	dependantPaths []string // use Document.GetDependants
}

func (c CachedDocument) Read(dest []byte) error {
	// use scanVariable()
	// use
}
