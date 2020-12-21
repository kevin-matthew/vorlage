package compiler

import (
	"errors"
	"fmt"
	"io"
)

const MaxVariableLength = 32

type Cache interface {

	/*
	 * this is asked every compRequest. If true is returned, a call to AddToCache
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

// stringer
func (v variablePos) String() string {
	return fmt.Sprintf("'%s'", v.fullName)
}

type CachedDocument struct {
	missingDefs    []variablePos
	path           string   // could also be memoery
	dependantPaths []string // use Document.GetDependants
}

func (c CachedDocument) Read(dest []byte) error {
	// use scanVariable()
	// use

	return errors.New("not implemented")
}
