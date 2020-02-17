package doccomp

import (
	"io"
	"os"
	"strconv"
	"strings"
)

//const EndOfLine   = "\n#"
const MacroArgument = " " //todo: just rename this to 'macrospace'
const DefineStr = "#define"
const IncludeStr = "#include"
const EndOfLine = "\n"
const VariablePrefix = "$"
const MacroMaxLength = 2048
const MaxVariableLength = 32

const DocumentReadBlock = len(EndOfLine)*2 + len(
	DefineStr)*len(IncludeStr)*256

type DocumentStream struct {
}

type NormalDefinition struct {
	variable string
	value    string
}

func (d NormalDefinition) GetName() string {
	return d.variable
}

type Document struct {
	file *os.File

	parent             *Document
	curentlyReading    *Document // used for reading
	curentlyReadingDef Definition

	MacroReadBuffer         []byte
	VariableDetectionBuffer []byte // used before every Read(
	// ) to see if there's variables

	includes                []Document
	includePositionsLineNum []uint // used for debugging
	includePositions        []uint64
	includeLengths          []uint

	defines                []Definition
	definePositionsLineNum []uint // used for debugging
	definePositions        []uint64
	defineLengths          []uint

	possibleVariablePositionsLineNum []uint // used for debugging
	possibleVariablePositions        []uint64

	allRecursiveNormalDefines []Definition
}

func LoadRequestedDocument(request Request) (doc Document, oerr *Error) {
	path := request.GetFilePath()
	return loadDocumentFromPath(path, nil)
}

func (doc *Document) FindProcDefinitions() []uint {

}

func (doc *Document) AddDefinition(definitions Definition) {

}

func loadDocumentFromPath(path string, parent *Document) (doc Document, oerr *Error) {
	oerr = &Error{}
	oerr.SetSubject(path)

	var cerr error
	doc.MacroReadBuffer = make([]byte, MacroMaxLength)
	doc.VariableDetectionBuffer = make([]byte, len(VariablePrefix))
	doc.curentlyReading = &doc
	doc.parent = parent
	Debugf("opening file '%s'", path)
	doc.file, cerr = os.Open(path)
	if cerr != nil {
		oerr.ErrStr = "failed to open file stream"
		oerr.SetBecause(NewError(cerr.Error()))
		return doc, oerr
	}

	Debugf("detecting macros in '%s'", path)
	err := doc.detectMacrosPositions()
	if err != nil {
		oerr.ErrStr = "failed to detect macros"
		oerr.SetBecause(err)
		doc.CloseRecursively()
		return doc, oerr
	}

	// step 3
	Debugf("parsing %d includes '%s'", len(doc.includePositions), path)
	err = doc.runIncludes()
	if err != nil {
		oerr.ErrStr = "failed to run includes"
		oerr.SetBecause(err)
		doc.CloseRecursively()
		return doc, oerr
	}

	// step 4
	Debugf("parsing %d defines '%s'", len(doc.definePositions), path)
	err = doc.runDefines()
	if err != nil {
		oerr.ErrStr = "failed to run defines"
		oerr.SetBecause(err)
		doc.CloseRecursively()
		return doc, oerr
	}

	// step 5
	// step 5 must be put into the doccomp.go file. as this functino
	// is recursive and all recursive calls must be complete in order to fill
	// variables.
	/*Debugf("filling normal variables in '%s'", len(doc.definePositions), path)
	oerr = doc.fillNormalVariables()
	if oerr != nil {
		return doc,oerr
	}*/

	_, cerr = doc.file.Seek(0, 0)
	if cerr != nil {
		oerr.ErrStr = "failed to seek back to the beginning of the stream"
		oerr.SetBecause(NewError(cerr.Error()))
		doc.CloseRecursively()
		return doc, oerr
	}

	return doc, nil
}

// helper-function for loadDocumentFromPath
// quickly goes through the document and detects where macros as well as where
// variables could possibly be
func (doc *Document) detectMacrosPositions() (oerr *Error) {
	var n int
	var allbytes uint64
	var linenum uint // used for debugging

	// make a new buffer
	buffer := make([]byte, DocumentReadBlock)

	// loop through the hole file until we hit the end
	for n != len(buffer) {

		// load bytes into the buffer
		n, err := doc.file.Read(buffer)

		// keep track of all bytes we've read so far, we'll need this later.
		allbytes += uint64(n)

		// all errors except for EOF should kill the function
		if err == io.EOF {
			break
		}
		if err != nil {
			oerr := &Error{}
			oerr.ErrStr = errFailedToReadBytes
			oerr.SetBecause(NewError(err.Error()))
			return oerr
		}

		// loop through all bytes in the buffer.
		for i := 0; i < n; i++ {

			// if we cross a newline, increment linenum
			if i+len(EndOfLine) <= n && string(buffer[i:i+len(
				EndOfLine)]) == EndOfLine {
				linenum++
			}

			// try to detect a '#define'
			if i+len(EndOfLine)+len(DefineStr) <= n && string(
				buffer[i:i+len(EndOfLine)+len(DefineStr)]) == EndOfLine+DefineStr {

				Debugf("%s:%d: detected macro '%s'", doc.file.Name(),
					linenum, DefineStr)
				doc.definePositions =
					append(doc.definePositions, allbytes+uint64(i+len(EndOfLine)))
				doc.definePositionsLineNum = append(doc.
					definePositionsLineNum, linenum)
				continue
			}

			// try to detect a '#include'
			if i+len(IncludeStr) <= n && string(buffer[i:i+len(EndOfLine)+len(IncludeStr)]) == EndOfLine+IncludeStr {
				Debugf("%s:%d: detected macro '%s'", doc.file.Name(),
					linenum, IncludeStr)
				doc.includePositions =
					append(doc.includePositions, allbytes+uint64(i+len(EndOfLine)))
				doc.includePositionsLineNum = append(doc.
					includePositionsLineNum, linenum)
				continue
			}

			// simply dectect a '$'
			if i+len(VariablePrefix) <= n && string(
				buffer[i:i+len(VariablePrefix)]) == VariablePrefix {

				Debugf("%s:%d: detected possible variable",
					doc.file.Name(),
					linenum)
				doc.possibleVariablePositions =
					append(doc.possibleVariablePositions, allbytes+uint64(i))
				doc.possibleVariablePositionsLineNum = append(doc.
					possibleVariablePositionsLineNum, linenum)
				continue
			}
		}
	}
	return nil
}

// helper-function for loadDocumentFromPath
func (doc *Document) runIncludes() (oerr *Error) {
	oerr = &Error{}
	doc.includes = make([]Document, len(doc.includePositions))

	// TODO: if we wanted to, we could make this for loop multithreaded.
	for i, inc := range doc.includePositions {
		oerr.SetSubject(doc.file.Name() + ":" + strconv.Itoa(int(doc.
			includePositionsLineNum[i])))

		// extract the include macro
		_, arg, length, err := doc.scanMacroAtPosition(inc)
		if err != nil {
			oerr.ErrStr = "failed to parse"
			oerr.SetBecause(err)
			return oerr
		}
		doc.includeLengths[i] = length

		// the first argument is a filename to include
		filename := strings.TrimSpace(arg)

		// get the file and parse the file (recursively)
		// TODO: detect circular dependencies (what if a file includes itself?)
		// TODO: would it be a good idea to try to load files from the cache
		// first?
		Debugf("%s: including '%s'", oerr.Subject, filename)
		includedDoc, err := loadDocumentFromPath(filename, doc)
		if err != nil {
			oerr.ErrStr = "failed to include document"
			oerr.SetBecause(err)
			return oerr
		}
		doc.includes[i] = includedDoc
	}

	return nil
}

// helper-function for loadDocumentFromPath
func (doc *Document) runDefines() (oerr *Error) {
	oerr = &Error{}
	doc.defines = make([]NormalDefinition, len(doc.definePositions))

	// TODO: if we wanted to, we could make this for loop multithreaded.
	for i, inc := range doc.definePositions {
		oerr.SetSubject(doc.file.Name() + ":" + strconv.Itoa(int(doc.
			definePositionsLineNum[i])))

		// extract the include macro
		_, arg, length, err := doc.scanMacroAtPosition(inc)
		if err != nil {
			oerr.ErrStr = "failed to parse"
			oerr.SetBecause(err)
			return oerr
		}
		doc.defineLengths[i] = length

		trimmedArg := strings.TrimSpace(arg)

		// the following comments will be talking in the context of the
		// following example: #define $myvar hello

		// make sure we see the '$' in #define $myvar hello
		if len(trimmedArg) < len(VariablePrefix) || trimmedArg[0:len(
			VariablePrefix)] != VariablePrefix {
			oerr.ErrStr = "variable to define is missing the prefix '" +
				"" + VariablePrefix + "'"
			return oerr
		}

		// make sure we see the 'myvar' in '$myvar'
		if len(trimmedArg) < len(VariablePrefix)+1 {
			oerr.ErrStr = "variable name is missing"
			return oerr
		}

		// extract the '$myvar' (variableName) and the 'hello'
		// (value) from '$myvar hello' (trimmedArg)
		var variableName, value string
		for j := len(VariablePrefix) + 1; j < len(trimmedArg); j++ {
			if j+len(MacroArgument) <= len(trimmedArg) && trimmedArg[j:j+len(
				MacroArgument)] == MacroArgument {

				variableName = strings.TrimSpace(trimmedArg[:j])
				value = strings.TrimSpace(trimmedArg[j:])
			}
		}

		Debugf("%s: adding normal definition of '%s' = '%s'", oerr.Subject,
			variableName, value)
		doc.defines[i] = NormalDefinition{
			variable: variableName,
			value:    value,
		}
	}
	return nil
}

// if n < len(p) it's probably because you were about to read a macro,
// simply read again and you'll read the expanded macro. In other words,
// any time there's a macro in the file, read is forced to start there.
//
// if you hit a variable than, then n will be < len(p).
//your next read will read the contents of the variable.
//
// that being said, len(p) >= MacroMaxLength.
func (doc *Document) Read(dest []byte) (int, error) {

	// you may ask... what the hell is going on:
	// - why is there read() AND Read()?
	// - what does doc.currentlyReading mean?
	//
	// The code is laid out like this because of the fact that there's
	// '#include's. Once an '#include' is read, doc.currentlyReading swtiches
	// to the document that was included by that '#include'. Furthermore,
	// that included document can also have documents IT includes, thus,
	// currentlyReading can be pointing to a document that doesn't even exist
	// in doc.Includes (ie, it could point to doc.Includes[3].Includes[1])
	if doc.curentlyReadingDef != nil {
		n, err := doc.curentlyReadingDef.Read(dest)
		if err != nil && err != io.EOF {
			return n, err
		}

		// if the variable is all done being read...
		if err == io.EOF {
			// ... lets set it to not currenlt being read anymore
			doc.curentlyReadingDef = nil
			// and continue on with a normal read...
		}
	}

	// do a normal read
	return doc.curentlyReading.read(dest,
		doc) // TODO: I'm really not sure this woorks...
}

func (doc *Document) read(dest []byte, root *Document) (int, error) {
	// before we do any reading, lets get the current position.
	pos, cerr := doc.file.Seek(0, 1)
	if cerr != nil {
		oerr := NewError(errFailedToSeek)
		oerr.SetSubject(doc.file.Name())
		oerr.SetBecause(NewError(cerr.Error()))
		return 0, oerr
	}

	n, cerr := doc.file.Read(dest)
	if cerr == io.EOF {
		// so we've reached the end of the file.
		// HOWEVER: the file we reached the end of could be any file... and
		// the parents could all still have work to do. so lets pass it back
		// up to the parent.
		root.curentlyReading = doc.parent
		return root.Read(dest) // TODO: this is really weird...
	}
	if cerr != nil {
		oerr := NewError(errFailedToReadBytes)
		oerr.SetSubject(doc.file.Name())
		oerr.SetBecause(NewError(cerr.Error()))
		return n, oerr
	}
	endpos := pos + int64(n)

	// set cutOff to the next detected #define, #include, or possible variable
	var cutOff int64
	for cutOff = pos; cutOff < endpos; cutOff++ {
		// we hit an #include
		for i, incpos := range doc.includePositions {
			if cutOff == int64(incpos) {
				// next time they call read, it will be reading the file
				// that was included by the '#include'
				Debugf("directing Read() to access '%s'",
					doc.includes[i].file.Name())
				root.curentlyReading = &doc.includes[i]

				// before we let the child doc start reading, lets skip the
				// #include macro so when the child sets currently reading
				// back to doc, we don't read the same #include twice.
				_, cerr := doc.file.Seek(cutOff+int64(doc.includeLengths[i]), 0)
				if cerr != nil {
					oerr := NewError(errFailedToReadBytes)
					oerr.SetSubject(doc.file.Name())
					oerr.SetBecause(NewError(cerr.Error()))
					return n, oerr
				}
				break
			}
		}

		// we hit a #define... just skip over it.
		for i, incpos := range doc.definePositions {
			if cutOff == int64(incpos) {

				// skip over it. We already got everything we need out of it.
				Debugf("Read() omitting #define at %s:%d",
					doc.file.Name(),
					doc.definePositionsLineNum[i])
				_, cerr := doc.file.Seek(cutOff+int64(doc.defineLengths[i]), 0)
				if cerr != nil {
					oerr := NewError(errFailedToReadBytes)
					oerr.SetSubject(doc.file.Name())
					oerr.SetBecause(NewError(cerr.Error()))
					return n, oerr
				}
				break
			}
		}

		// we hit a possible $variable position
		// and the next time they read,
		//they'll be reading the definitions Read() function
		for i, incpos := range doc.possibleVariablePositions {
			if cutOff == int64(incpos) {
				Debugf("Read() came across a possible variable at %s:%d",
					doc.file.Name(),
					doc.possibleVariablePositionsLineNum[i])

				// extract the possible variable name (possibleVariableName)
				variableNameBuffer := make([]byte, MaxVariableLength)
				vn, cerr := doc.file.ReadAt(variableNameBuffer, cutOff)
				if cerr != nil {
					oerr := NewError(errFailedToReadBytes)
					oerr.SetSubject(doc.file.Name())
					oerr.SetBecause(NewError(cerr.Error()))
					return n, oerr
				}
				possibleVariableName := string(variableNameBuffer[:vn])

				var foundMatch = false
				for _, d := range root.getRecursiveDefinitions() {
					// first off, lets make it easy, trim down the possible variable
					// name to be the same length as an actual one
					actualVariableName := d.GetName()
					tuncPossibleVariableName := possibleVariableName
					if len(possibleVariableName) > len(actualVariableName) {
						tuncPossibleVariableName = possibleVariableName[:len(
							actualVariableName)]
					}

					if tuncPossibleVariableName != actualVariableName {
						// didn't find it yet.
						continue
					}

					foundMatch = true
					Debugf("found definition for variable '%s' at  %s:%d",
						actualVariableName,
						doc.file.Name(),
						doc.possibleVariablePositionsLineNum[i])
					Debugf("switching Read() to look at variable '%s'",
						actualVariableName)

					// this will be used back in the original Read() function
					// to then read from the definition.
					root.curentlyReadingDef = d

					// before we let the child doc start reading, lets skip the
					// #include macro so when the child sets currently reading
					// back to doc, we don't read the same #include twice.
					_, cerr := doc.file.Seek(cutOff+int64(len(actualVariableName)), 0)
					if cerr != nil {
						oerr := NewError(errFailedToReadBytes)
						oerr.SetSubject(doc.file.Name())
						oerr.SetBecause(NewError(cerr.Error()))
						return n, oerr
					}
					break
				} // for _,d := range root.getRecursiveDefinitions()
				if !foundMatch {
					// we didn't find a match, throw an error.
					oerr := NewError(errNotDefined)
					oerr.SetSubject(doc.file.Name() + ":" + strconv.Itoa(
						int(doc.possibleVariablePositionsLineNum[i])))
					return n, oerr
				}
				break
			} // if cutOff == int64(incpos)
		} // for i,incpos := range doc.possibleVariablePositions
	} // for cutOff = pos; cutOff < endpos; cutOff++

	// sense we are going to force the Read function to be called again,
	// let's cut their buffer short so they don't read any info that
	// was deliberatley skipped using Seek.
	n = int(cutOff - pos)
	return n, nil
}

func (doc *Document) Close() error {
	if doc.file == nil {
		return nil
	}
	Debugf("closing '%s'",
		doc.file.Name())
	return doc.file.Close()
}

func (doc *Document) CloseRecursively() {
	_ = doc.Close()
	for _, d := range doc.includes {
		d.CloseRecursively()
	}
}

// TODO: hopefully this works?
func (doc *Document) getRecursiveDefinitions() []Definition {
	if doc.allRecursiveNormalDefines != nil {
		return doc.allRecursiveNormalDefines
	}
	doc.allRecursiveNormalDefines = doc.defines
	for _, d := range doc.includes {
		childDefines := d.getRecursiveDefinitions()
		for _, c := range childDefines {
			doc.allRecursiveNormalDefines = append(doc.
				allRecursiveNormalDefines, c)
		}
	}
	return doc.allRecursiveNormalDefines
}

// helper-function for runIncludes and runDefines
// makes use of doc.MacroReadBuffer
func (doc *Document) scanMacroAtPosition(position uint64) (macro string,
	argument string, length uint, oerr *Error) {
	_, err := doc.file.Seek(int64(position), 0)
	if err != nil {
		oerr := NewError(errFailedToSeek)
		oerr.SetSubject(doc.file.Name())
		oerr.SetBecause(NewError(err.Error()))
		return "", "", 0, oerr
	}
	n, err := doc.file.Read(doc.MacroReadBuffer)
	if err != nil {
		oerr := NewError(errFailedToReadBytes)
		oerr.SetSubject(doc.file.Name())
		oerr.SetBecause(NewError(err.Error()))
		return "", "", 0, oerr
	}
	var endOfLine int = 0
	var argumentPos int = 0

	// we see where the end of the line is.
	// and to be efficent we'll also grab where the argument start is
	// aswell.
	for endOfLine = 0; endOfLine < n; endOfLine++ {

		if argumentPos == 0 && endOfLine+len(MacroArgument) < n &&
			string(doc.MacroReadBuffer[endOfLine:endOfLine+len(
				MacroArgument)]) == MacroArgument {
			argumentPos = endOfLine
		}

		if endOfLine+len(EndOfLine) < n &&
			string(doc.MacroReadBuffer[endOfLine:endOfLine+len(
				EndOfLine)]) == EndOfLine {
			break
		}
	}
	if endOfLine == n {
		oerr := NewError("no end-of-line detected")
		return "", "", 0, oerr
	}

	// now we just seperate the macro from the argument. don't trim, be very
	// litterall
	macro = string(doc.MacroReadBuffer[0:argumentPos])
	argument = string(doc.MacroReadBuffer[argumentPos:endOfLine])

	Debugf("parsed '%s' macro in '%s' with argument '%s'", macro,
		doc.file.Name(), argument)

	return macro, argument, uint(endOfLine), nil
}

func (doc *Document) remainingDefinitions() []Definition {
	return nil
}

func (doc *Document) complete() (stream DocumentStream, err *Error) {
	remaining := doc.remainingDefinitions()
	if len(remaining) != 0 {
		err := NewError("variables were left undefined")
		// build a nice little string of remaining definitinos
		names := make([]string, len(remaining))
		for i, d := range remaining {
			names[i] = d.GetName()
		}
		subject := strings.Join(names, ", ")
		err.SetSubject(subject)
		return stream, err
	}
	return stream, errNotImplemented
}
