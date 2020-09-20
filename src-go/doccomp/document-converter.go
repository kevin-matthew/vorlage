package doccomp

import (
	"io"
	"os"
	"strings"
)

import "../lmlog"

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

// nonConvertedFile means the file to which was originally supplied by
// the user will be the one to which will be outputted.
type nonConvertedFile struct {
	bytesRead int64

	// remember to use the sourceFile fore doing file ops. Ie. calling
	// sourceDocument.Read in nonConvertedFile.Read will cause a recursive crash.
	sourceDocument *Document

	// the file to read, close, rewind.
	sourceFile File
	hasEOFd    bool

	// used for drawParser
	variableReadBuffer []byte

	// buffer used to hold what was read from the file when reading from
	// definitions
	tmpBuff []byte

	// will be nil if not currently reading.
	currentlyReadingDef Definition
}

type osFileHandle struct {
	*os.File
	resetPos int64
}

func osFileToFile(file *os.File, resetPos int64) File {
	return osFileHandle{file, resetPos}
}

func (o osFileHandle) Read(p []byte) (int, error) {
	return o.File.Read(p)
}
func (o osFileHandle) Rewind() error {
	_, err := o.File.Seek(o.resetPos, 0)
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
	 * Convert the file and return the nonConvertedFile. If Error
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
// returns >0,nil,nil if a variable has been found but not completely done scanned, send the next block of src over. Be sure to add n to charsource next call.
// returns _,pos,nil if a variable was found and fully scanned
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
				// if we are, then the caller can't draw anymore. so send em the
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
			return nonVarBytes, nil, nil

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

func (c *nonConvertedFile) Read(dest []byte) (n int, err error) {

	if c.hasEOFd {
		return 0, io.EOF
	}

	// are we currently exposing a defnition?
	if c.currentlyReadingDef != nil {
		// we are... so lets read it.
		n, err = c.currentlyReadingDef.Read(dest)
		if err != nil {
			if err != io.EOF {
				return n, err
			}
			// we're done reading the current definition.
			c.currentlyReadingDef = nil

			dest = dest[n:]

			//return n, nil
		} else {
			// we're not done reading the definition yet.
			return n, nil
		}
	}

	// before we do another read from the file, we need to make sure that
	// tmpBuff has been emptied.
	if len(c.tmpBuff) != 0 {
		lmlog.DebugF("tmp buffer has stuff in it \"%s\"", string(c.tmpBuff))
		bytesCopied := copy(dest, c.tmpBuff)
		c.tmpBuff = c.tmpBuff[bytesCopied:]
		dest = dest[bytesCopied:]
		n += bytesCopied
		if len(c.tmpBuff) != 0 {
			// if we still have stuff in tmp buffer, we need another read.
			return n, nil
		}
	}

	// so if we're done reading the definition, move along with another read
	// from the file

	var sourceBytesRead int
	sourceBytesRead, err = c.sourceFile.Read(dest)

	// we WILL NOT append this bytes to n. Because if we do there's a chance
	// we've read in a variable. We don't want that to show up for the caller.
	//n+=sourceBytesRead
	if err != nil && err != io.EOF {
		return n, err
	}
	// set hasEOFd so future calls will return EOF.
	c.hasEOFd = err == io.EOF

	c.bytesRead += int64(sourceBytesRead)

	nonVarByteCount, pos, cerr := drawParseVar(c.variableReadBuffer, dest[:sourceBytesRead], c.bytesRead)
	//lmlog.DebugF("drawparseVar: %d, %#v, %#v -- %s", nonVarByteCount, pos,cerr, string(c.variableReadBuffer));
	if cerr != nil {
		return n + nonVarByteCount, *cerr
	}
	if nonVarByteCount == sourceBytesRead {
		lmlog.DebugF("detected no variables in bytes")
		return n + nonVarByteCount, err
	}
	if pos != nil {
		// we have stumbled apon a variable.
		def, derr := c.sourceDocument.define(*pos)
		if derr != nil {
			return n + nonVarByteCount, derr
		}

		// lets start reading it on the next read.
		c.currentlyReadingDef = def

		// but first we ask:
		// did the buffer (dest) pick up anything after the variable?
		// (ie dest[:n] = "123$(varible)abc")
		//                    ^         ^  ^
		//                   (a)       (b)(c)
		//
		//  (a) = position of nonVarByteCount
		//  (b) = position of nonVarByteCount + len(pos.fullName)
		//  (c) = position of sourceBytesRead (length of string)
		// if so, we need to save the extra (ie "abc") to tmpBuff because
		// dest will be used to read-in the variable and will in turn all
		// content that was read after the variable from the file.
		if sourceBytesRead > nonVarByteCount+len(pos.fullName) {

			// calculate the remaining buffer length
			remainingBuffLen := sourceBytesRead - (nonVarByteCount + len(pos.fullName))
			// make the tmp buff the size of everything after the variable.
			c.tmpBuff = make([]byte, remainingBuffLen)
			// copy everything after that variable into that buffer.
			copy(c.tmpBuff, dest[nonVarByteCount+len(pos.fullName):sourceBytesRead])

			// lets use up the rest of the buffer we were given in this call
			// to try to fill the next one. You could comment these three
			// lines out and the only thing it would really affect is the
			// fact that the caller needs to call Read a few more times.
			//lmlog.AlertF("%s", string(dest[nonVarByteCount:]))
			bytesOfDefinition, err := c.Read(dest[nonVarByteCount:])
			n += bytesOfDefinition + nonVarByteCount
			return n, err
		}
		return n + nonVarByteCount, err
	}

	// at this point we know that a variable was not found, but not all bytes were
	// ignored.
	return n + nonVarByteCount, err
}

// todo: I don't think this method should belong to Document...
// ARCHITECTUAL ERROR.
func (doc *Document) define(pos variablePos) (Definition, error) {
	var foundDef Definition

	// we have found a variable in the document.
	// lets go find it's definition

	// first we ask if its a processor variable or a normal variable?
	if len(pos.processorName) != 0 {
		// its a processed variable.
		// lets find the right processor...
		p, ok := Processors[pos.processorName]
		if !ok {
			// processor not found
			oerr := NewError(errNoProcessor)
			oerr.SetSubject(pos.processorName)
			return nil, oerr
		}

		// at this point we've found the processor now we need to get
		// its variables to find the right one.
		vars := p.GetVariables()
		var i int
		for i = range vars {
			if vars[i].name == pos.variableName {
				break
			}
		}
		if i == len(vars) {
			// we didn't find it in the processor.
			oerr := NewError(errNotDefined)
			oerr.SetSubject(pos.fullName)
			return nil, oerr
		}
		// at this point: we've found the processor, we've foudn the variable
		// but what about the variable's inputs... do we have everything we
		// we need?
		for _, n := range vars[i].inputNames {
			// Now we ask ourselves (doc) if we've been given all the right
			// inputs
			_, foundStatic := doc.args.staticInputs[n]
			_, foundStream := doc.args.streamInputs[n]
			if !foundStatic && !foundStream {
				// op. we wern't given all the right inputs.
				oerr := NewError(errInputNotProvided)
				oerr.SetSubjectf("\"%s\" not provided for %s", n, pos.fullName)
				return nil, oerr
			}
			if foundStatic && foundStream {
				// for some reason we have both a static and stream input with
				// the same name that are being requested. That's an error.
				oerr := NewError(errInputInStreamAndStatic)
				oerr.SetSubjectf("\"%s\" in %s", n, pos.fullName)
				return nil, oerr
			}
			if foundStream {
				// so we found the stream this input wants... but was it
				// used by a previous procvar?
				if pv, ok := doc.args.streamInputsUsed[n]; ok {
					// it was. That's an error.
					oerr := NewError(errDoubleInputStream)
					oerr.SetSubjectf("\"%s\" requested by %s but was used by %s already", n, pv, pos.fullName)
					return nil, oerr
				}
				// it was not. So lets keep track this this streamed input
				// was just consumed by this procvar.
				doc.args.streamInputsUsed[n] = pos.fullName
			}
			if foundStatic {
				// no further action needs to take place if we found the
				// static variable.
			}
		}

		// lets recap, it's a processor variable. We found the processor.
		// we found the variable. we found all of it's inputs.
		// lets define it.
		var logerr error
		foundDef, logerr = p.DefineVariable(pos.variableName,
			doc.args.staticInputs,
			doc.args.streamInputs)

		// as per the documentation, if there's an error with the definition,
		// it is ignored. All proc vars MUST be defined as long as they're loaded.
		if logerr != nil {
			Debugf("error defining %s: %s", pos.variableName, logerr.Error())
		}
	} else {
		// its a normal variable.
		// look through all the doucment's normal definitions.
		for i, d := range *(doc.allDefinitions) {
			if d.GetFullName() == pos.fullName {
				foundDef = &((*(doc.allDefinitions))[i])
				break
			}
		}
	}

	// did we find a definition from the logic above?
	if foundDef != nil {
		// found it!
		// lets start reading this normal definition.
		// but first we must reset it as per the Definition specification.
		err := foundDef.Reset()
		if err != nil {
			oerr := NewError(errResetVariable)
			oerr.SetSubject(pos.fullName)
			oerr.SetBecause(NewError(err.Error()))
			return nil, oerr
		}
		// okay that's out of the way. The next call will begin reading
		// the definition's contents.
		return foundDef, nil
	}

	// we did not find the definition
	oerr := NewError(errNotDefined)
	oerr.SetSubject(pos.fullName)
	return foundDef, oerr
}

func (c *nonConvertedFile) Rewind() error {
	//clear the variable buffer
	for i := 0; i < len(c.variableReadBuffer); i++ {
		c.variableReadBuffer[i] = 0
	}

	err := c.sourceFile.Rewind()
	if err != nil {
		return err
	}
	c.hasEOFd = false

	c.bytesRead = 0
	return nil
}

func (c *nonConvertedFile) Close() error {
	if c.currentlyReadingDef != nil {
		_ = c.currentlyReadingDef.Reset()
	}

	err := c.sourceFile.Close()
	if err != nil {
		return err
	}

	return nil
}

var _ File = &nonConvertedFile{}

func (doc *Document) getConverted(sourceFile File) (converedFile File, err *Error) {
	// todo: switch on the source file name to find a good converted (haml->html)
	file := nonConvertedFile{
		sourceFile:         sourceFile,
		sourceDocument:     doc,
		variableReadBuffer: make([]byte, MacroMaxLength),
	}
	return &file, nil
}
