package doccomp

import (
	"fmt"
	"io"
	"strings"
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
}

// helper-function for detectVariables
// looks at the buffer and tries to parse a variable out of it.
// The itself variable must start at the very beginning of the buffer.
func scanVariable(buffer []byte, charsource int64) (pos variablePos, oerr *Error) {

	if len(buffer) < len(VariablePrefix)+len(VariableSuffix) {
		// this buffer isn't big enough to even consider the possibility
		// of having a variable.
		return pos, NewError(errBufferTooShort)
	}

	var length, j, dotIndex int
	for length = 0; length < len(VariablePrefix); length++ {
		if buffer[length] != VariablePrefix[length] {
			// no valid prefix, no variable to be found here!
			return pos, NewError(errVariableMissingPrefix)
		}
	}

	for ; length < len(buffer); length++ {
		// keep scanning through until we find the VariableSuffix
		if length+len(VariableSuffix) >= len(buffer) {
			// The VariableSuffix was not found in this buffer.
			oerr = NewError(errVariableMissingSuffix)
			return pos, oerr
		}

		for j = 0; j < len(VariableSuffix); j++ {
			if buffer[length+j] != VariableSuffix[j] {
				break
			}
		}
		if j == len(VariableSuffix) {
			length = length + j
			break
		}
	}

	varName := buffer[len(VariablePrefix) : length-len(VariableSuffix)]

	if !variableRegexpProc.Match(varName) {
		oerr = NewError(errVariableName)
		oerr.SetSubjectf("'%s'", string(varName))
		return pos, oerr
	}

	dotIndex = strings.Index(string(buffer[:length]),
		VariableProcessorSeporator)
	if dotIndex == -1 {
		dotIndex = 0
	}

	pos = variablePos{
		fullName:     string(buffer[:length]),
		variableName: string(buffer[len(VariablePrefix) : length-len(VariableSuffix)]),
		processorName: string(buffer[len(VariablePrefix) : len(
			VariablePrefix)+dotIndex]),
		charPos: charsource,
		length:  uint(length),
	}
	return pos, nil
}

// returns nil,err if an error happened while parsing
// returns 0,nil,nil if no variable has been found yet
// returns >0,nil,nil if a variable has been found but not completely done
//  scanned.
// returns pos,nil if a variable was found
func drawParseVar(dest []byte, src []byte,
	charsource int64) (pos *variablePos, oerr *Error) {

	// we retain i for 2 reasons: 1) we can check of a loop completed and 2)
	// so if we have just scanned in the start of a new variable from src to
	// dest, we can start the normal scanning proccess from where we left off
	// when we discovered the start of the variable.
	var i = 0

	var j = 0

	// if the dest starts with null (0), then that means we haven't started
	// drawing a variable yet. So look at src to see if (and where) we should
	// start.
	if dest[0] != VariablePrefix[0] {
		for ; i < len(src) && src[i] != VariablePrefix[0]; i++ {
		}
		if i == len(src) {
			// we're not recording a variable, nor did we find the start of one
			// in src.
			return nil, nil
		}
	}

	// at this point we've just found, or have previously found at least
	// the start of a variable that is currently loaded in dest.

	// so lets find where we left off with dest (when dest[j] == 0 that means
	// we havent written to that part of it yet)
	for ; j < len(dest) && dest[j] != 0; j++ {
	}

	// now appended src to dest
	for j < len(dest) && i < len(src) {
		dest[j] = src[i]
		j++
		i++
	}

	// if the scanned in bytes is shorter than the prefix, then
	// we need to wait another scan.
	if j < len(VariablePrefix) {
		return nil, nil
	}

	scannedPos, serr := scanVariable(dest, charsource)
	if serr != nil {
		switch serr.ErrStr {
		case errVariableMissingSuffix:
			// so we didn't scan in a full variable into dest...
			// now we ask: are we out of room in dest?
			if j == len(dest) {
				// if we are, then the caller can't draw anymore. so send em the
				// error
				for j = 0; j < len(dest); j++ {
					dest[j] = 0
				}
				return nil, serr
			}
			// if we're not at the end of dest then the caller can call this
			// function more times until we indeed fill it.
			return nil, nil
		case errVariableMissingPrefix:
			// theres no prefix. which means the buffer is crap if it doesn't
			// even start right. So throw the whole thing away.
			for j = 0; j < len(dest); j++ {
				dest[j] = 0
			}
			return nil, nil

		}
		return nil, serr
	}
	for j = 0; j < len(dest); j++ {
		dest[j] = 0
	}
	return &scannedPos, nil
}

func (v variablePos) ToString() string {
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
}
