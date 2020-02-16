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

const DocumentReadBlock = len(EndOfLine)*2 + len(
	DefineStr)*len(IncludeStr)*256

type DocumentStream struct {
}

type NormalDefinition struct {
	variable string
	value    string
}

type Document struct {
	file *os.File

	includes []Document
	defines  []NormalDefinition

	MacroReadBuffer []byte

	includePositionsLineNum []uint // used for debugging
	includePositions        []uint64

	definePositionsLineNum []uint // used for debugging
	definePositions        []uint64

	possibleVariablePositionsLineNum []uint // used for debugging
	possibleVariablePositions        []uint64
}

func LoadRequestedDocument(request Request) (doc Document, oerr *Error) {
	path := request.GetFilePath()
	return LoadDocumentFromPath(path)
}

func LoadDocumentFromPath(path string) (doc Document, oerr *Error) {
	oerr = &Error{}
	oerr.SetSubject(path)

	var cerr error
	doc.MacroReadBuffer = make([]byte, MacroMaxLength)

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
		return doc, oerr
	}

	// step 3
	Debugf("parsing %d includes '%s'", len(doc.includePositions), path)
	err = doc.runIncludes()
	if err != nil {
		oerr.ErrStr = "failed to run includes"
		oerr.SetBecause(err)
		return doc, oerr
	}

	// step 4
	Debugf("parsing %d defines '%s'", len(doc.definePositions), path)
	err = doc.runDefines()
	if err != nil {
		oerr.ErrStr = "failed to run defines"
		oerr.SetBecause(err)
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

	return doc, nil
}

// helper-function for LoadDocumentFromPath
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
			oerr.ErrStr = "failed to read bytes from stream"
			oerr.SetBecause(NewError(err.Error()))
			return oerr
		}

		// loop through all bytes in the buffer.
		for i := 0; i < n; i++ {

			// if we cross a newline, increment linenum
			if i+len(EndOfLine) < n && string(buffer[i:i+len(
				EndOfLine)]) == EndOfLine {
				linenum++
			}

			// try to detect a '#define'
			if i+len(EndOfLine)+len(DefineStr) < n && string(
				buffer[i+len(EndOfLine):i+len(DefineStr)]) == DefineStr {

				Debugf("%s:%d: detected macro '%s'", doc.file.Name(),
					linenum, DefineStr)
				doc.definePositions =
					append(doc.definePositions, allbytes+uint64(i+len(EndOfLine)))
				doc.definePositionsLineNum = append(doc.
					definePositionsLineNum, linenum)
				continue
			}

			// try to detect a '#include'
			if i+len(IncludeStr) < n && string(buffer[i+len(EndOfLine):i+len(
				IncludeStr)]) == IncludeStr {

				Debugf("%s:%d: detected macro '%s'", doc.file.Name(),
					linenum, IncludeStr)
				doc.includePositions =
					append(doc.includePositions, allbytes+uint64(i+len(EndOfLine)))
				doc.includePositionsLineNum = append(doc.
					includePositionsLineNum, linenum)
				continue
			}

			// simply dectect a '$'
			if i+len(VariablePrefix) < n && string(
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

// helper-function for LoadDocumentFromPath
func (doc *Document) runIncludes() (oerr *Error) {
	oerr = &Error{}

	// TODO: if we wanted to, we could make this for loop multithreaded.
	for i, inc := range doc.includePositions {
		oerr.SetSubject(doc.file.Name() + ":" + strconv.Itoa(int(doc.
			includePositionsLineNum[i])))

		// extract the include macro
		_, arg, err := doc.scanMacroAtPosition(inc)
		if err != nil {
			oerr.ErrStr = "failed to parse"
			oerr.SetBecause(err)
			return oerr
		}

		// the first argument is a filename to include
		filename := strings.TrimSpace(arg)

		// get the file and parse the file (recursively)
		// TODO: detect circular dependencies (what if a file includes itself?)
		// TODO: would it be a good idea to try to load files from the cache
		// first?
		Debugf("%s: including '%s'", oerr.Subject, filename)
		includedDoc, err := LoadDocumentFromPath(filename)
		if err != nil {
			oerr.ErrStr = "failed to include document"
			oerr.SetBecause(err)
			return oerr
		}
		doc.includes = append(doc.includes, includedDoc)
	}

	return nil
}

// helper-function for LoadDocumentFromPath
func (doc *Document) runDefines() (oerr *Error) {
	oerr = &Error{}

	// TODO: if we wanted to, we could make this for loop multithreaded.
	for i, inc := range doc.definePositions {
		oerr.SetSubject(doc.file.Name() + ":" + strconv.Itoa(int(doc.
			definePositionsLineNum[i])))

		// extract the include macro
		_, arg, err := doc.scanMacroAtPosition(inc)
		if err != nil {
			oerr.ErrStr = "failed to parse"
			oerr.SetBecause(err)
			return oerr
		}

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

		// extract the 'myvar' and the 'hello' while ignoreing extra whitespace
		var variableName, value string
		for j := len(VariablePrefix) + 1; j < len(trimmedArg); j++ {
			if j+len(MacroArgument) < len(trimmedArg) && trimmedArg[j:j+len(
				MacroArgument)] == MacroArgument {

				variableName = strings.TrimSpace(trimmedArg[len(
					VariablePrefix)+1 : j])
				value = strings.TrimSpace(trimmedArg[j:])
			}
		}

		Debugf("%s: adding normal definition of '%s' = '%s'", oerr.Subject,
			variableName, value)
		doc.defines = append(doc.defines, NormalDefinition{
			variable: variableName,
			value:    value,
		})
	}
	return nil
}

// helper-function for runIncludes and runDefines
// makes use of doc.MacroReadBuffer
func (doc *Document) scanMacroAtPosition(position uint64) (macro string,
	argument string, oerr *Error) {
	_, err := doc.file.Seek(int64(position), 0)
	if err != nil {
		oerr := NewError("cannot seek file")
		oerr.SetSubject(doc.file.Name())
		oerr.SetBecause(NewError(err.Error()))
		return "", "", oerr
	}
	n, err := doc.file.Read(doc.MacroReadBuffer)
	if err != nil {
		oerr := NewError("cannot read file")
		oerr.SetSubject(doc.file.Name())
		oerr.SetBecause(NewError(err.Error()))
		return "", "", oerr
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
		return "", "", oerr
	}

	// now we just seperate the macro from the argument. don't trim, be very
	// litterall
	macro = string(doc.MacroReadBuffer[0:argumentPos])
	argument = string(doc.MacroReadBuffer[argumentPos:endOfLine])

	Debugf("parsed '%s' macro in '%s' with argument '%s'", macro,
		doc.file.Name(), argument)

	return macro, argument, nil
}

func (d *Document) addDefinition(definitions Definition) {

}

func (d *Document) remainingDefinitions() []Definition {
	return nil
}

func (d *Document) complete() (stream DocumentStream, err *Error) {
	remaining := d.remainingDefinitions()
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
