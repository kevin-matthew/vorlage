package doccomp

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//const EndOfLine   = "\n#"
const MacroArgument = " " //todo: just rename this to 'macrospace'
const DefineStr = "#define"
const IncludeStr = "#include"
const EndOfLine = "\n"
const VariablePrefix = "$"
const MacroMaxLength = 1024
const MaxVariableLength = 32

const DocumentReadBlock = len(EndOfLine)*2 + len(
	DefineStr)*len(IncludeStr)*256

type NormalDefinition struct {
	variable string
	value    string
	seeker   int
}

func (d NormalDefinition) GetName() string {
	return d.variable
}

func (d *NormalDefinition) Read(p []byte) (int, error) {
	if d.seeker == len(d.value) {
		return 0, io.EOF
	}
	d.seeker = copy(p, d.value[d.seeker:])
	return d.seeker, nil
}

type Document struct {
	file *os.File

	path string

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

	normalDefines          []NormalDefinition
	definePositionsLineNum []uint // used for debugging
	definePositions        []uint64
	defineLengths          []uint

	possibleVariablePositionsLineNum []uint // used for debugging
	possibleVariablePositions        []uint64

	allRecursiveNormalDefines []Definition
}

func LoadDocument(path string) (doc Document, oerr *Error) {
	return loadDocumentFromPath(path, nil)
}

func (doc *Document) AddDefinition(definition Definition) {
	doc.allRecursiveNormalDefines = append(doc.allRecursiveNormalDefines, definition)
}

func (doc Document) GetFileName() string {
	return doc.path
}

func loadDocumentFromPath(path string, parent *Document) (doc Document, oerr *Error) {
	oerr = &Error{}
	oerr.SetSubject(path)

	var cerr error
	doc.MacroReadBuffer = make([]byte, MacroMaxLength)
	doc.VariableDetectionBuffer = make([]byte, len(VariablePrefix))
	doc.curentlyReading = &doc
	doc.parent = parent
	doc.path = path

	sourceerr := doc.ancestorHasPath(path)
	if sourceerr != nil {
		oerr.ErrStr = "circular inclusion"
		oerr.SetSubject(*sourceerr)
		return doc, oerr
	}

	cwd, _ := os.Getwd()
	Debugf("opening file '%s' from %s", path, cwd)
	doc.file, cerr = os.OpenFile(path, os.O_RDONLY, 0)
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
		_ = doc.Close()
		return doc, oerr
	}

	// step 3
	Debugf("parsing %d includes '%s'", len(doc.includePositions), path)
	err = doc.runIncludes()
	if err != nil {
		oerr.ErrStr = "failed parsing #include in file"
		oerr.SetBecause(err)
		_ = doc.Close()
		return doc, oerr
	}

	// step 4
	Debugf("parsing %d normalDefines '%s'", len(doc.definePositions), path)
	err = doc.runDefines()
	if err != nil {
		oerr.ErrStr = "failed parsing #defines in file"
		oerr.SetBecause(err)
		_ = doc.Close()
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
		_ = doc.Close()
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
	var linenum uint = 1 // used for debugging
	var onNewLine = true

	// make a new buffer
	buffer := make([]byte, DocumentReadBlock)

	// loop through the hole file until we hit the end
	for n != len(buffer) {

		// load bytes into the buffer
		n, err := doc.file.Read(buffer)

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

			// before we even care about being a new line or not,
			// lets detect if we see a variable.
			if i+len(VariablePrefix) <= n && string(
				buffer[i:i+len(VariablePrefix)]) == VariablePrefix {

				Debugf("%s:%d: detected possible variable",
					doc.path,
					linenum)
				doc.possibleVariablePositions =
					append(doc.possibleVariablePositions, allbytes+uint64(i))
				doc.possibleVariablePositionsLineNum = append(doc.
					possibleVariablePositionsLineNum, linenum)
			}

			// if we're on a fresh line (either right after a '\n' or
			// it's at the very start of the file
			if onNewLine {

				// try to detect a '#define'
				if i+len(DefineStr) <= n && string(
					buffer[i:i+len(DefineStr)]) == DefineStr {

					Debugf("%s:%d: detected macro '%s'", doc.path,
						linenum, DefineStr)
					doc.definePositions =
						append(doc.definePositions, allbytes+uint64(i))
					doc.definePositionsLineNum = append(doc.
						definePositionsLineNum, linenum)
				}

				// try to detect a '#include'
				if i+len(IncludeStr) <= n && string(
					buffer[i:i+len(IncludeStr)]) == IncludeStr {

					Debugf("%s:%d: detected macro '%s'", doc.path,
						linenum, IncludeStr)
					doc.includePositions =
						append(doc.includePositions, allbytes+uint64(i))
					doc.includePositionsLineNum = append(doc.
						includePositionsLineNum, linenum)
				}
			}

			// if we cross a newline, increment linenum
			if i+len(EndOfLine) <= n && string(buffer[i:i+len(
				EndOfLine)]) == EndOfLine {
				onNewLine = true
				linenum++
			} else {
				onNewLine = false
			}
		}

		// keep track of total number of bytes we've read so far
		allbytes += uint64(n)
	}
	return nil
}

// helper-function for loadDocumentFromPath
func (doc *Document) runIncludes() (oerr *Error) {
	oerr = &Error{}
	doc.includes = make([]Document, len(doc.includePositions))
	doc.includeLengths = make([]uint, len(doc.includePositions))

	// TODO: if we wanted to, we could make this for loop multithreaded.
	for i, inc := range doc.includePositions {
		oerr.SetSubject(doc.path + ":" + strconv.Itoa(int(doc.
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
		fullFileName := filepath.Dir(doc.path) + string(filepath.
			Separator) + filename

		// get the file and parse the file (recursively)
		// TODO: detect circular dependencies (what if a file includes itself?)
		// TODO: would it be a good idea to try to load files from the cache
		// first?
		Debugf("%s: including '%s'", oerr.Subject, fullFileName)

		includedDoc, err := loadDocumentFromPath(fullFileName, doc)
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
	doc.normalDefines = make([]NormalDefinition, len(doc.definePositions))
	doc.defineLengths = make([]uint, len(doc.definePositions))

	// TODO: if we wanted to, we could make this for loop multithreaded.
	for i, inc := range doc.definePositions {
		oerr.SetSubject(doc.path + ":" + strconv.Itoa(int(doc.
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

		// make sure we see the 'm' in '$myvar'... otherwise we're missing
		// the variable name.
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
				break
			}
		}

		Debugf("%s: adding normal definition of '%s' = '%s'", oerr.Subject,
			variableName, value)
		doc.normalDefines[i] = NormalDefinition{
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
	return doc.ReadIgnore(dest, false)
}

// used for cacheing
func (doc *Document) ReadIgnore(dest []byte, ignoreMissingDefinition bool) (
	int,
	error) {

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
			Debugf("Read() completed the variable. going back to file '%s'",
				doc.curentlyReading.path)
			// ... lets set it to not currenlt being read anymore
			doc.curentlyReadingDef = nil
			// and continue on with a normal read...
		} else {
			return n, nil
		}
	}

	// do a normal read
	return doc.curentlyReading.read(dest,
		doc, ignoreMissingDefinition) // TODO: I'm really not sure this woorks...
}

func (doc *Document) read(dest []byte, root *Document,
	ignoreMissing bool) (int, error) {
	// before we do any reading, lets get the current position.
	pos, cerr := doc.file.Seek(0, 1)
	if cerr != nil {
		oerr := NewError(errFailedToSeek)
		oerr.SetSubject(doc.path)
		oerr.SetBecause(NewError(cerr.Error()))
		return 0, oerr
	}

	n, cerr := doc.file.Read(dest)
	if cerr == io.EOF {
		Debugf("Read() completed file %s",
			doc.path)

		// so we've reached the end of the file.
		// HOWEVER: the file we reached the end of could be any file... and
		// the parents could all still have work to do. so lets pass it back
		// up to the parent.
		root.curentlyReading = doc.parent

		// if we don't have a parent, then we're done-done.
		if doc.parent == nil {
			return 0, io.EOF
		}
		return root.Read(dest) // TODO: this is really weird...
	}
	if cerr != nil {
		oerr := NewError(errFailedToReadBytes)
		oerr.SetSubject(doc.path)
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
					oerr.SetSubject(doc.path + ":#inlcude")
					oerr.SetBecause(NewError(cerr.Error()))
					return n, oerr
				}
				goto ret
			}
		}

		// we hit a #define... just skip over it.
		for i, incpos := range doc.definePositions {
			if cutOff == int64(incpos) {

				// skip over it. We already got everything we need out of it.
				Debugf("Read() omitting #define at %s:%d",
					doc.path,
					doc.definePositionsLineNum[i])
				_, cerr := doc.file.Seek(cutOff+int64(doc.defineLengths[i]), 0)
				if cerr != nil {
					oerr := NewError(errFailedToReadBytes)
					oerr.SetSubject(doc.path)
					oerr.SetBecause(NewError(cerr.Error()))
					return n, oerr
				}
				goto ret
			}
		}

		// we hit a possible $variable position
		// and the next time they read,
		//they'll be reading the definitions Read() function
		for i, incpos := range doc.possibleVariablePositions {
			if cutOff == int64(incpos) {
				Debugf("Read() came across a possible variable at %s:%d",
					doc.path,
					doc.possibleVariablePositionsLineNum[i])

				// extract the possible variable name (possibleVariableName)
				variableNameBuffer := make([]byte, MaxVariableLength)
				vn, cerr := doc.file.ReadAt(variableNameBuffer, cutOff)
				if cerr != nil && cerr != io.EOF {
					oerr := NewError(errFailedToReadBytes)
					oerr.SetSubject(doc.path + ":" + strconv.Itoa(int(doc.
						possibleVariablePositionsLineNum[i])) + "@" + strconv.Itoa(int(cutOff)))
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
						doc.path,
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
						oerr := NewError(errFailedToSeek)
						oerr.SetSubject(doc.path + ":$var")
						oerr.SetBecause(NewError(cerr.Error()))
						return n, oerr
					}
					break
				} // for _,d := range root.getRecursiveDefinitions()
				if !foundMatch {
					if ignoreMissing {
						Debugf("ignore missing definition at %s:%d",
							doc.path,
							doc.possibleVariablePositionsLineNum[i])
					} else {
						// we didn't find a match, throw an error.
						oerr := NewError(errNotDefined)
						oerr.SetSubject(doc.path + ":" + strconv.Itoa(
							int(doc.possibleVariablePositionsLineNum[i])))
						return n, oerr
					}
				}
				goto ret
			} // if cutOff == int64(incpos)
		} // for i,incpos := range doc.possibleVariablePositions
	} // for cutOff = pos; cutOff < endpos; cutOff++

	// sense we are going to force the Read function to be called again,
	// let's cut their buffer short so they don't read any info that
	// was deliberatley skipped using Seek.
ret:
	n = int(cutOff - pos)
	return n, nil
}

func (doc *Document) close() error {
	if doc.file == nil {
		return nil
	}
	Debugf("closing '%s'",
		doc.path)
	return doc.file.Close()
}

// recursively closes
func (doc *Document) Close() error {
	_ = doc.close()
	for _, d := range doc.includes {
		_ = d.Close()
	}
	return nil
}

// generate all definitions that are detected in all documents below this one.
// it stores the answer and subseqent calls will always return the initial
// result.
func (doc *Document) getRecursiveDefinitions() []Definition {
	if doc.allRecursiveNormalDefines != nil {
		return doc.allRecursiveNormalDefines
	}
	doc.allRecursiveNormalDefines = make([]Definition, len(doc.normalDefines))
	for i := 0; i < len(doc.normalDefines); i++ {
		doc.allRecursiveNormalDefines[i] = &doc.normalDefines[i]
	}
	for _, d := range doc.includes {
		childDefines := d.getRecursiveDefinitions()
		for _, c := range childDefines {
			doc.allRecursiveNormalDefines = append(doc.
				allRecursiveNormalDefines, c)
		}
	}
	return doc.allRecursiveNormalDefines
}

// helper-function for loadDocumentFromPath
// returns non-nill if ancestor has path. What then returns is a 'stack'
// of what is included by what.
func (doc *Document) ancestorHasPath(filepath string) *string {
	// todo: what if one of the inlcudes is a symlink? It can be tricked
	// into a circular dependency

	if doc.parent != nil {
		if doc.parent.path == filepath {
			stack := doc.path + " -> " + filepath
			return &stack
		}
		perr := doc.parent.ancestorHasPath(filepath)
		if perr != nil {
			//oerr := NewError(perr.Error() + ": " + "which includes")
			//oerr.SetSubject(doc.parent.path)
			//oerr.SetBecause(perr)
			stack := doc.path + " -> " + *perr
			return &stack
		}
	}
	return nil

	/*Debugf("does %s == %s?", doc.path, filepath)
	if doc.path == filepath {
		Debugf("yes it does!")
		oerr := NewError("including")
		oerr.SetSubject(doc.path)
		return oerr
	}
	for _,inc := range doc.includes {
		ooerr := inc.ancestorHasPath(filepath)
		if ooerr != nil {
			oerr := NewError("including")
			oerr.SetSubject(doc.path)
			oerr.SetBecause(ooerr)
			return oerr
		}
	}
	return  nil*/

}

// helper-function for runIncludes and runDefines
// makes use of doc.MacroReadBuffer
func (doc *Document) scanMacroAtPosition(position uint64) (macro string,
	argument string, length uint, oerr *Error) {
	_, err := doc.file.Seek(int64(position), 0)
	if err != nil {
		oerr := NewError(errFailedToSeek)
		oerr.SetSubject("@ char" + strconv.Itoa(int(position)))
		oerr.SetBecause(NewError(err.Error()))
		return "", "", 0, oerr
	}
	n, err := doc.file.Read(doc.MacroReadBuffer)
	if err != nil {
		oerr := NewError(errFailedToReadBytes)
		oerr.SetSubject("@ char" + strconv.Itoa(int(position)))
		oerr.SetBecause(NewError(err.Error()))
		return "", "", 0, oerr
	}
	var endOfLine int = 0
	var argumentPos int = 0

	// we see where the end of the line is.
	// and to be efficent we'll also grab where the argument start is
	// aswell.
	for endOfLine = 0; endOfLine < n; endOfLine++ {

		// grab the start of the argument
		// ie: '#define   $myvar asdf  ' we grab '  $myvar asdf  '
		if argumentPos == 0 && endOfLine+len(MacroArgument) < n &&
			string(doc.MacroReadBuffer[endOfLine:endOfLine+len(
				MacroArgument)]) == MacroArgument {
			argumentPos = endOfLine
		}

		// grab the end of the line

		// first see if we can get to '\n'...
		if endOfLine+len(EndOfLine) <= n && string(doc.
			MacroReadBuffer[endOfLine:endOfLine+len(EndOfLine)]) == EndOfLine {
			// cut out the end of the line
			endOfLine += len(EndOfLine)
			break
		}
	}
	if endOfLine == n {
		// ...or we just see if we're at the end of the file
		if n < len(doc.MacroReadBuffer) {
			// do nothing, 'n' is the proper end of the line.
			// (logic left like this for readability)
		} else {
			// however, if we didn't detect a new line, then that means
			// that this macro is too long.
			oerr := NewError("no end-of-line detected (macro too long?)")
			return "", "", 0, oerr
		}
	}

	// now we just seperate the macro from the argument. don't trim, be very
	// litterall
	macro = string(doc.MacroReadBuffer[0:argumentPos])
	argument = string(doc.MacroReadBuffer[argumentPos:endOfLine])

	Debugf("parsed '%s' macro in '%s' with argument '%s'", macro,
		doc.path, argument)

	return macro, argument, uint(endOfLine), nil
}
