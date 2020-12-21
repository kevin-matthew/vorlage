package compiler

import "strings"

// variablePos is a struct that shows where a varible is, and it's componennts.
type variablePos struct {
	fullName     string
	variableName string

	processorName         string // if "" then it is not a processed variable
	processorVariableName string // if "" then it is not a processed variable
	charPos               int64
	length                uint
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

	pos = variablePos{
		fullName:     string(buffer[:length]),
		variableName: string(buffer[len(VariablePrefix) : length-len(VariableSuffix)]),
		charPos:      charsource,
		length:       uint(length),
	}

	dotIndex = strings.Index(pos.variableName,
		VariableProcessorSeporator)
	if dotIndex != -1 {
		pos.processorName = string(pos.variableName[:dotIndex])
		pos.processorVariableName = string(pos.variableName[dotIndex+len(VariableProcessorSeporator):])
	}

	return pos, nil
}

// reads a variable into dest from src. Returns how many bytes were ignored
// at the start of src. len(dest) must be >= MaxVariableLength.
// make sure you use the same dest buffer for subsequent reads.
//
// returns _,nil,err if an error happened while parsing
// returns len(src),nil,nil if no variable has been found yet
// returns 0,nil,nil if the VariablePrefix hasn't been fully read
// returns >0,nil,nil if a variable has been found but not completely done scanned, send the next block of src over. Be sure to add n to charsource next call.
// returns _,pos,nil if a variable was found and fully scanned
//
// todo: what if we scan a partial prefix on one block,
//       but then we didn't find it on the second block? We need a way to roll
//       back a half-prefix-scan... we could try return a negative number?
func drawParseVar(dest []byte, src []byte,
	charsource int64) (n int, pos *variablePos, oerr *Error) {

	// we retain i for 2 reasons: 1) we can check of a loop completed and 2)
	// so if we have just scanned in the start of a new variable from src to
	// dest, we can start the normal scanning proccess from where we left off
	// when we discovered the start of the variable.
	var i = 0
	var nonVarBytes int
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
			return i, nil, nil
		}
	}

	// at this point we've just found, or have previously found at least
	// the start of a VariablePrefix that is currently in src at index i.
	// So let's also document how many bytes at the beginning of this buffer
	// it takes to get to the start to the variabel
	nonVarBytes = i

	// so lets find where we left off with dest (when dest[j] == 0 that means
	// we havent written to j yet)
	for ; j < len(dest) && dest[j] != 0; j++ {
	}

	// now appended src[i:] to dest
	for j < len(dest) && i < len(src) {
		dest[j] = src[i]
		j++
		i++
	}

	// if the scanned in bytes is shorter than the prefix, then
	// we need to wait another scan because it's automatically impossible
	// we've recorder the entire thing.
	if j < len(VariablePrefix) {
		return nonVarBytes, nil, nil
	}

	// now we call scanVariable that will parse out the variable's componenets
	// OR it will return an error that will inform us of what we're missing.
	scannedPos, serr := scanVariable(dest, charsource+int64(i))
	if serr != nil {
		// scanVariable has told us we're missing something... so what is it?
		switch serr.ErrStr {
		case errVariableMissingSuffix:
			// so we didn't scan in a full variable into dest...
			// now we ask: are we out of room in dest?
			if j == len(dest) {
				// we are, then the caller can't draw anymore. so send em the
				// error. This will happen if len(dest) < MaxVariableLength or
				// if the variable is simply too long and/or is not terminated

				// but before we return, lets clear out the dest buffer.
				for j = 0; j < len(dest); j++ {
					dest[j] = 0
				}
				return nonVarBytes, nil, serr
			}

			// if we're not at the end of dest then the caller can call this
			// function more times until we indeed fill it.
			return nonVarBytes, nil, nil

		case errVariableMissingPrefix:
			// theres no prefix. which means the buffer is crap if it doesn't
			// even start right. So throw the whole thing away.
			for j = 0; j < len(dest); j++ {
				dest[j] = 0
			}
			return len(src), nil, nil

		}

		// unhandled error returned by scanVariable. Example of this is
		// when the variable uses bad syntax.
		return nonVarBytes, nil, serr
	}

	// at this point, we have successfully scanned in a good variable into
	// scannedpos.
	// so lets clear the buffer and return it.
	for j = 0; j < len(dest); j++ {
		dest[j] = 0
	}
	return nonVarBytes, &scannedPos, nil
}
