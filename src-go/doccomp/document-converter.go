package doccomp

import (
	"io"
	"os"
)

// its a io.Reader that will read from the file but will NOT read the macros.
type File interface {
	// n will sometimes be < len(p) but that does not mean it's the end of the
	// file. Only when 0, io.EOF is returned will it be the end of the file.
	Read(p []byte) (int, error)

	// returns to the beginning of the file
	Rewind() error

	// must be called when conversion is done.
	Close() error
}


type osFileHandle struct {
	*os.File
	resetPos int64
}

func osFileToFile(file *os.File, resetPos int64) File {
	return osFileHandle{file,resetPos}
}

func (o osFileHandle) Read(p []byte) (int, error) {
	return o.File.Read(p)
}
func (o osFileHandle) Rewind() error {
	_,err := o.File.Seek(o.resetPos, 0)
	return err
}
func (o osFileHandle) Close() error {
	return o.File.Close()
}

type DocumentConverter interface {
	/*
	 * ShouldConvert is called to see if this particular document converter
	 * should handler the conversion of the file. If true is returned,
	 * ConverFile will be called. If false is returned,
	 * the next available document converter will be asked the same question.
	 */
	ShouldConvert(path string) bool

	/*
	 * Convert the file and return the ConvertedFile. If Error
	 * is non-nil, the document's loading is stopped completely.
	 * note that the SourceFile:Close MUST be called before this function
	 * returns.
	 */
	ConvertFile(reader io.Reader) (io.ReadCloser, error)

	/*
	 * For verboseness/errors/UI purposes. No functional signifigance
	 */
	GetDescription() string
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

// reads a variable into dest from src. Returns how many bytes were ignored
// at the start of src. len(dest) must be >= MaxVariableLength.
// make sure you use the same dest buffer for subsequent reads.
//
// returns _,nil,err if an error happened while parsing
// returns len(src),nil,nil if no variable has been found yet
// returns 0,nil,nil if the VariablePrefix hasn't been fully read
// returns >0,nil,nil if a variable has been found but not completely done scanned, send the next block of src over.
// returns _,pos,nil if a variable was found and fully scanned
func drawParseVar(dest []byte, src []byte,
	charsource int64) (n int, pos *variablePos, oerr *Error) {

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
			return i, nil, nil
		}
	}

	// at this point we've just found, or have previously found at least
	// the start of a VariablePrefix that is currently in src at index i

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
	// we need to wait another scan.
	if j < len(VariablePrefix) {
		return 0,nil, nil
	}

	// now we call scanVariable that will parse out the variable's componenets
	// OR it will return an error that will inform us of what we're missing.
	scannedPos, serr := scanVariable(dest, charsource)
	if serr != nil {
		// scanVariable has told us we're missing something... so what is it?
		switch serr.ErrStr {
		case errVariableMissingSuffix:
			// so we didn't scan in a full variable into dest...
			// now we ask: are we out of room in dest?
			if j == len(dest) {
				// if we are, then the caller can't draw anymore. so send em the
				// error. This will happen if len(dest) < MaxVariableLength or
				// if the variable is simply too long and/or is not terminated

				// but before we return, lets clear out the dest buffer.
				for j = 0; j < len(dest); j++ {
					dest[j] = 0
				}
				return i, nil, serr
			}

			// if we're not at the end of dest then the caller can call this
			// function more times until we indeed fill it.
			return i, nil, nil

		case errVariableMissingPrefix:
			// theres no prefix. which means the buffer is crap if it doesn't
			// even start right. So throw the whole thing away.
			for j = 0; j < len(dest); j++ {
				dest[j] = 0
			}
			return i, nil, nil

		}

		// unhandled error returned by scanVariable. Example of this is
		// when the variable uses bad syntax.
		return i, nil, serr
	}

	// at this point, we have successfully scanned in a good variable into
	// scannedpos.
	// so lets clear the buffer and return it.
	for j = 0; j < len(dest); j++ {
		dest[j] = 0
	}
	return i, &scannedPos, nil
}

func getConverted(sourceFile File) (converedFile File, *Error) {

	return rawcontents, nil
}
