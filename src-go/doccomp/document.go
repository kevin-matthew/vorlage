package doccomp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

//const EndOfLine   = "\n#"
const MacroArgument = ' ' //todo: just rename this to 'macrospace'
const MacroPrefix = "#"
const DefineStr = "#define"
const PrependStr = "#prepend"
const AppendStr = "#append"
const EndOfLine = "\n"
const VariablePrefix = "$("
const VariableSuffix = ")"
const VariableProcessorSeporator = "."
const VariableRegexp = `^(?:[a-z0-9]+\.)?[a-zA-Z0-9]+$`
const MacroMaxLength = 1024
const MaxVariableLength = 32

var variableRegexpProc = regexp.MustCompile(VariableRegexp)

const DocumentReadBlock = len(MacroPrefix)*3 + len(
	DefineStr)*len(PrependStr)*len(AppendStr)*256

type NormalDefinition struct {
	variable string
	value    string
	seeker   int
}

func CreateNormalDefinition(variable string, value string) (NormalDefinition,
	*Error) {
	ret := NormalDefinition{
		variable: variable,
		value:    value,
	}
	if !bytesAreString([]byte(VariablePrefix), variable, 0) {
		err := NewError("#define variable does not start with '$'")
		err.SetSubject(variable)
		return ret, err
	}

	if len(variable) == len(VariablePrefix) {
		return ret, NewError("variable is blank")
	}
	if value == "" {
		return ret, NewError("value is blank")
	}
	return ret, nil
}

func (d NormalDefinition) GetFullName() string {
	return d.variable
}

func (d *NormalDefinition) Read(p []byte) (int, error) {
	if d.seeker == len(d.value) {
		return 0, io.EOF
	}
	d.seeker = copy(p, d.value[d.seeker:])
	return d.seeker, nil
}

type macoPos struct {
	args    []string
	charPos uint64
	length  uint
	linenum uint
}

type variablePos struct {
	fullName     string
	variableName string // this will be the Processor-Variable Name if
	// processorName is not ""
	processorName string // if "" then it is not a processed variable
	charPos uint64
	length  uint
	linenum uint // used for debugging
	colnum  uint // used for debugging
}

func (m macoPos) ToString() string {
	return fmt.Sprintf("line %s", m.linenum)
}

func (v variablePos) ToString() string {
	return fmt.Sprintf("'%s', line %d, col %d", v.fullName, v.linenum, v.colnum)
}

type Document struct {
	file       *os.File
	fileInode  uint64
	converters  []*DocumentConverter
	proccessorLoader ProcessorLoader

	path string

	root               *Document
	parent             *Document

	allDefinitions   *[]Definition // if root != nil,
	 // then this points to the root's allDefinitions
	allIncluded     *[]*Document // if root != nil,
	// then this points to the root's allIncluded

	curentlyReading    *Document // used for reading
	curentlyReadingDef Definition

	MacroReadBuffer         []byte
	VariableDetectionBuffer []byte // used before every Read(
	// ) to see if there's variables

	macros   []macoPos


	prepends    []*Document // points to somewhere in allIncluded
	prependsPos []*macoPos  // points to somewhere in macros

	appends   []*Document // points to somewhere in allIncluded
	appendPos []*macoPos  // points to somewhere in macros

	normalDefines []*NormalDefinition // points to somewhere in allDefines
	normalPos     []*macoPos // points to somewhere in macros

	variablePos []variablePos
}

/*
 * Opens a document and recursively opens all the documents referenced by
 * #prepends. For every document that is opened,
 * the converters are first consulted (via converters[i].ShouldConvert) in
 * the order they are in the array. The first converter to return true will
 * be used. If no converters return true, the document is not converted and will
 * be read as normal (via io.OpenFile).
 */
func LoadDocument(path string, converters []*DocumentConverter,
	proccessorLoader ProcessorLoader) (doc Document,
	oerr *Error) {
	return loadDocumentFromPath(path, converters, proccessorLoader, nil, nil)
}

func (doc Document) GetFileName() string {
	return doc.path
}

func loadDocumentFromPath(path string,
		converters []*DocumentConverter,
		proccessorLoader ProcessorLoader,
		parent *Document,
		root *Document) (doc Document, oerr *Error) {

	oerr = &Error{}
	oerr.SetSubject(path)

	var cerr error
	doc.MacroReadBuffer = make([]byte, MacroMaxLength)
	doc.VariableDetectionBuffer = make([]byte, len(VariablePrefix))
	doc.curentlyReading = &doc
	doc.parent = parent
	doc.root   = root
	doc.path = path
	doc.proccessorLoader = proccessorLoader
	doc.converters = converters

	// see the document struct's instructions about 'allIncluded' and
	// 'allDefinitions'
	if doc.root != nil {
		doc.allDefinitions = doc.root.allDefinitions
		doc.allIncluded    = doc.root.allIncluded
	} else {
		doc.root = &doc
		doc.allDefinitions = &[]Definition{}
		doc.allIncluded    = &[]*Document{}
	}

	sourceerr := doc.ancestorHasPath(path)
	if sourceerr != nil {
		oerr.ErrStr = "circular inclusion"
		oerr.SetSubject(*sourceerr)
		return doc, oerr
	}

	file,serr := os.Open(path)
	if serr != nil {
		oerr.ErrStr = "failed to open file"
		oerr.SetBecause(NewError(serr.Error()))
		_ = doc.Close()
		return doc, oerr
	}
	doc.file = file
	var stat syscall.Stat_t
	serr = syscall.Stat(path, &stat)
	if serr != nil {
		oerr.ErrStr = "failed to get inode for file"
		oerr.SetBecause(NewError(cerr.Error()))
		_ = doc.Close()
		return doc, oerr
	}
	doc.fileInode = stat.Ino

	// now that the file is open (and converting), lets detect all macros in it
	Debugf("detecting macros in '%s'", path)
	err := doc.detectMacrosPositions()
	if err != nil {
		oerr.ErrStr = "failed to detect macros"
		oerr.SetBecause(err)
		_ = doc.Close()
		return doc, oerr
	}


	err = doc.detectVariables()
	if err != nil {
		oerr.ErrStr = "failed to detect variables"
		oerr.SetBecause(err)
		_ = doc.Close()
		return doc, oerr
	}
	Debugf("detected '%d' variable uses in '%s'", len(doc.variablePos), path)

	Debugf("interpreting macros in '%s'", path)
	err = doc.processMacros()
	if err != nil {
		oerr.ErrStr = "failed to interpret macros"
		oerr.SetBecause(err)
		_ = doc.Close()
		return doc, oerr
	}

	// run #prepends
	Debugf("prepending %d documents to '%s'", len(doc.prependsPos), path)
	doc.prepends = make([]*Document, len(doc.prependsPos))
	for i := 0; i < len(doc.prependsPos); i++ {
		pos := doc.prependsPos[i]
		inc,err := doc.include(pos.args[1])
		if err != nil {
			oerr.ErrStr = "failed to prepend document"
			oerr.SetBecause(err)
			_ = doc.Close()
			return doc, oerr
		}
		doc.prepends[i] = inc
	}

	// run #appends
	Debugf("appending %d documents to '%s'", len(doc.appendPos), path)
	doc.appends = make([]*Document, len(doc.appendPos))
	for i := 0; i < len(doc.appendPos); i++ {
		pos := doc.appendPos[i]
		inc,err := doc.include(pos.args[1])
		if err != nil {
			oerr.ErrStr = "failed to append document"
			oerr.SetBecause(err)
			_ = doc.Close()
			return doc, oerr
		}
		doc.appends[i] = inc
	}

	// normal definitions (#define)
	Debugf("parsing %d normalDefines '%s'", len(doc.normalPos), path)
	for _, d := range doc.normalPos {
		def, err := CreateNormalDefinition(d.args[1], d.args[2])
		if err != nil {
			oerr.ErrStr = "cannot parse definition"
			oerr.SetSubjectf("%s %s", path, d.ToString())
			oerr.SetBecause(err)
			_ = doc.Close()
			return doc, oerr
		}

		err = doc.addDefinition(&def)
		if err != nil {
			oerr.ErrStr = "failed to add normal definition"
			oerr.SetSubjectf("%s %s", path, d.ToString())
			oerr.SetBecause(err)
			_ = doc.Close()
			return doc, oerr
		}
	}

	// processed definitions
	for _, p := range doc.variablePos {

		// if it has a non-empty processor name, then its a processor variable
		// otherwise ignore it
		if p.processorName == "" {
			continue
		}

		pros, err := doc.proccessorLoader.GetProcessor(p.processorName)
		if err != nil {
			oerr.ErrStr = "failed to get processor for variable"
			oerr.SetSubjectf("%s %s", path, p.ToString())
			oerr.SetBecause(err)
			_ = doc.Close()
			return doc, oerr
		}

		// check to make sure the processor will actually define it.
		var i int
		var procVar string
		var procVars = pros.GetVariableNames()
		for i = 0; i < len(procVars); i ++ {
			procVar = procVars[i]
			if procVar == p.variableName {
				break
			}
		}
		if i == len(procVars) {
			oerr.ErrStr = "processor does not define variable"
			oerr.SetSubjectf("%s %s", path, p.ToString())
			_ = doc.Close()
			return doc, oerr
		}

		// okay, now demand it defines it
		def, err := pros.DefineVariable(p.variableName)
		if err != nil {
			oerr.ErrStr = "processor failed to define variable"
			oerr.SetSubjectf("%s %s", path, p.ToString())
			oerr.SetBecause(err)
			_ = doc.Close()
			return doc, oerr
		}

		err = doc.addDefinition(def)
		if err != nil {
			oerr.ErrStr = "failed to add normal definition"
			oerr.SetSubjectf("%s %s", path, p.ToString())
			oerr.SetBecause(err)
			_ = doc.Close()
			return doc, oerr
		}
	}


	_, cerr = doc.file.Seek(0, 0)
	if cerr != nil {
		oerr.ErrStr = errFailedToSeek
		oerr.SetBecause(NewError(cerr.Error()))
		_ = doc.Close()
		return doc, oerr
	}
	return doc, nil
}

func bytesAreString(buff []byte, str string, offset int) bool {
	return offset+len(str) <= len(buff) &&
		string(buff[offset:offset+len(str)]) == str
}

func (doc *Document) detectVariables() *Error {
	var linenum uint = 1 // used for debugging
	var colnum uint      // used to generate colnum (for debuggin)

	_, cerr := doc.file.Seek(0, 0)
	if cerr != nil {
		oerr := NewError(errFailedToSeek)
		oerr.SetBecause(NewError(cerr.Error()))
		_ = doc.Close()
		return oerr
	}

	var at int64
	var lastBuffer = false

	// loop through the hole file until we hit the end
	for !lastBuffer {
		// load bytes into the buffer
		n, err := doc.file.ReadAt(doc.VariableDetectionBuffer,at)
		lastBuffer = err == io.EOF
		if err != nil && err != io.EOF {
			oerr := &Error{}
			oerr.ErrStr = errFailedToReadBytes
			oerr.SetBecause(NewError(err.Error()))
			return oerr
		}


		// if this buffer starts with a '$' we can then try to interpret a
		// variable
		if doc.VariableDetectionBuffer[0] == VariablePrefix[0] {
			pos, serr := scanVariable(doc.VariableDetectionBuffer,
				uint64(at),
				linenum,
				colnum)
			if serr != nil {
				// failed to parse. send it up.
				return serr
			}
			if pos != nil {
				// success, we've found a variable.
				doc.variablePos = append(doc.variablePos, *pos)
				Debugf("found variable '%s' at '%s'",
					string(doc.VariableDetectionBuffer[pos.charPos:pos.
					charPos+uint64(pos.length)]),
					pos.ToString())
			} else {
				// this buffer did contain a valid variable. Oh well, let
				// just move on. (else statement left for this comment's
				// readability)
			}
		}

		// find the next availabe '$' (aside from the '0' index which we just
		// checked above) and force the next/itoration to start
		// where that '$' was found or just move the entire buffer if nothing
		// was found. This also increments linenum if if finds and newlines.
		var scannedBytes = 0; colnum++ // increment column num because we
		// skip it in the for-loop.
		for scannedBytes = 1; scannedBytes < n; scannedBytes++ {
			if doc.VariableDetectionBuffer[scannedBytes] == '\n' {
				colnum = 1
				linenum++
			}
			if doc.VariableDetectionBuffer[scannedBytes] == VariablePrefix[0] {
				break
			}
			colnum++
		}
		at += int64(scannedBytes)
	}
	return nil
}

// helper-function for detectVariables
// looks at the buffer and trys to parse a variable out of it. The variable must
// start at the very beginning of the buffer.
// If VariablePrefix and VariableSuffix was not found, this will NOT result
// in an error.
func scanVariable(buffer []byte, charsource uint64,
	linenum uint, colnum uint) (pos *variablePos, oerr *Error) {

	if len(buffer) < len(VariablePrefix) + len(VariableSuffix) {
		// this buffer isn't big enough to even consider the possibility
		// of having a variable.
		return nil, nil
	}

	var length,j,dotIndex int
	for length = 0; length < len(VariablePrefix); length++ {
		if buffer[length] != VariablePrefix[length] {
			// no valid prefix, no variable to be found here!
			return nil, nil
		}
	}


	for ; length < len(buffer); length++ {
		// keep scanning through until we find the VariableSuffix

		if length + len(VariableSuffix) >= len(buffer) {
			// The VariableSuffix was not found in this buffer.
			oerr = NewError(errVariableTooLong)
			oerr.SetSubjectf("line %d", linenum)
			return nil, oerr
		}


		for j = 0; j < len(VariableSuffix); j++ {
			if buffer[length+j] != VariableSuffix[j] {
				break
			}
		}
		if j < len(VariableSuffix) {
			length = length +j
			break
		}
	}

	if !variableRegexpProc.Match(buffer[:length]) {
		oerr = NewError(errVariableName)
		oerr.SetSubjectf("'%s' at line %d", string(buffer[:length]), linenum)
		return nil,oerr
	}

	dotIndex = strings.Index(string(buffer[:length]),
		VariableProcessorSeporator)
	if dotIndex == -1 {
		dotIndex = 0
	}
	
	pos = &variablePos{
		fullName:      string(buffer[:length]),
		variableName:  string(buffer[len(VariablePrefix):length-len(VariableSuffix)]),
		processorName: string(buffer[len(VariablePrefix):len(
			VariablePrefix)+dotIndex]),
		charPos:       charsource,
		length:        uint(length),
		linenum:       linenum,
		colnum:        colnum,
	}
	return pos, nil
}

// helper-function for detectMacrosPositions
// simply looks at the buffer and scans a macro out of it. It returns the
// length of the line, and any possible error. If 0 length is returned,
// no more macros are left to scan.
// todo: capture sequencies ie: #include "this argument is in double quotes.txt"
func scanMaco(buffer []byte, charsource int64,
	linenum uint) (pos macoPos, oerr *Error) {

	// first off, do we even have a valid macro?
	if !bytesAreString(buffer, MacroPrefix, 0) {
		// no this isn't a macro... so we're done looking for macros.
		return pos, nil
	}

	// get length
	pos.linenum = linenum
	pos.charPos = uint64(charsource)
	pos.length = uint(len(MacroPrefix)) // we skip scanning the macro prefix
	for ; pos.length < uint(len(buffer)); pos.length++ {
		// grab the end of the line

		// first see if we can get to '\n'...
		if bytesAreString(buffer, EndOfLine, int(pos.length)) {
			// cut out the end of the line
			pos.length += uint(len(EndOfLine))
			break
		}
	}
	if pos.length <= uint(len(MacroPrefix)) {
		oerr = &Error{}
		oerr.ErrStr = "macro prefix detected but nothing defined"
		oerr.SetSubjectf(pos.ToString())
		return pos, oerr
	}

	// todo: what if macro is to long
	//append(pos.args, )
	tmp := strings.Split(string(buffer[:pos.length]), string(MacroArgument))
	pos.args = []string{}
	for _, t := range tmp {
		if t != "" {
			pos.args = append(pos.args, t)
		}
	}
	return pos, nil
}

// helper-function for loadDocumentFromPath
// quickly goes through the document and detects where macros as well as where
// variables could possibly be
func (doc *Document) detectMacrosPositions() (oerr *Error) {
	var linenum uint // used for debugging
	var at int64
	var lastBuffer bool

	// loop through the hole file until we hit the end
	for !lastBuffer {
		linenum++
		// load bytes into the buffer
		n, err := doc.file.ReadAt(doc.MacroReadBuffer, at)

		// all errors except for EOF should kill the function
		lastBuffer = err == io.EOF
		if err != nil && err != io.EOF {
			oerr := &Error{}
			oerr.ErrStr = errFailedToReadBytes
			oerr.SetBecause(NewError(err.Error()))
			return oerr
		}

		pos, oerr := scanMaco(doc.MacroReadBuffer[:n], at, linenum)
		if oerr != nil {
			return oerr
		}

		Debugf("detected macro '%s' in %s", pos.args[0], doc.path)
		doc.macros = append(doc.macros, pos)

		if pos.length == 0 {
			Debugf("finished detecting macros in '%s'", doc.path)
			return nil
		}
		at += int64(pos.length)
	}
	return nil
}

func (doc *Document) processMacros() (oerr *Error) {
	doc.normalPos = []*macoPos{}
	doc.prependsPos = []*macoPos{}
	doc.appendPos = []*macoPos{}
	for i := 0; i < len(doc.macros); i++ {
		m := &(doc.macros[i])
		switch m.args[0] {
		case DefineStr:
			if len(m.args) < 3 {
				oerr := NewError("#define missing arguments")
				oerr.SetSubject(m.ToString())
				return oerr
			}
			doc.normalPos = append(doc.normalPos, m)
			break
		case PrependStr:
			if len(m.args) < 2 {
				oerr := NewError("#prepend missing arguments")
				oerr.SetSubject(m.ToString())
				return oerr
			}
			doc.prependsPos = append(doc.prependsPos, m)
			break
		case AppendStr:
			if len(m.args) < 2 {
				oerr := NewError("#append missing arguments")
				oerr.SetSubject(m.ToString())
				return oerr
			}
			doc.appendPos = append(doc.appendPos, m)
			break
		}
	}
	return nil
}

// prevents duplicate opens
func (doc *Document) include(path string) (incdoc *Document, oerr *Error) {
	relPath := filepath.Dir(doc.path) + string(filepath.Separator) + path

	var stat syscall.Stat_t
	cerr := syscall.Stat(relPath, &stat)
	if cerr != nil {
		oerr := NewError("failed to stat document")
		oerr.SetSubject(relPath)
		oerr.SetBecause(NewError(cerr.Error()))
		return nil, oerr
	}

	// make sure we done re-include anything
	for _,d := range *doc.allIncluded {
		if d.fileInode == stat.Ino {
			Debugf("avoiding a re-opening of document '%s' (inode match)",
				path)
			return d, nil
		}
	}


	adoc, err := loadDocumentFromPath(relPath,
		doc.converters,
		doc.proccessorLoader,
		doc,
		doc.root)

	if err != nil {
		oerr := NewError("failed to include document")
		oerr.SetSubject(path)
		oerr.SetBecause(err)
		return nil, oerr
	}

	*doc.allIncluded = append(*doc.allIncluded, &adoc)
	return &adoc, nil

}

// prevent duplicate definitions
func (doc *Document) addDefinition(definition Definition) *Error {
	for _,d := range *doc.allDefinitions {
		if d.GetFullName() == definition.GetFullName() {
			oerr := NewError(errAlreadyDefined)
			oerr.SetSubjectf(d.GetFullName())
			return oerr
		}
	}

	*doc.allDefinitions = append(*doc.allDefinitions, definition)
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
func (doc *Document) ReadIgnore(dest []byte,
	ignoreMissingDefinition bool) (
	int,
	error) {

	// you may ask... what the hell is going on:
	// - why is there read() AND Read()?
	// - what does doc.currentlyReading mean?
	//
	// The code is laid out like this because of the fact that there's
	// '#include's. Once an '#include' is read, doc.currentlyReading swtiches
	// to the document that was allIncluded by that '#include'. Furthermore,
	// that allIncluded document can also have documents IT prepends, thus,
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
		doc, false, ignoreMissingDefinition) // TODO: I'm really not sure this
	// woorks...
}

// todo: remove the 'ignore' bools... we need to have some way to communicate
// processor vars to the processor. Maybe they should be passed in via Read?
// hmmmm.... or something like that. or perhaps 'add proccessor' instead of
// 'add definition'.... AH YES. Let's do that.
func (doc *Document) read(dest []byte, root *Document,
	ignoreIncludes bool,
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
		// up to the doc.
		root.curentlyReading = doc.parent

		// if we don't have a doc, then we're done-done.
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
		for i, incpos := range doc.prependPositions {
			if cutOff == int64(incpos) {

				// if we're ignoring macros then don't bother opening up
				// the include
				if !ignoreIncludes {
					// next time they call read, it will be reading the file
					// that was allIncluded by the '#include'
					Debugf("directing Read() to access '%s'",
						doc.prepends[i].path)
					root.curentlyReading = &doc.prepends[i]
				}

				// before we let the child doc start reading, lets skip the
				// #include macro so when the child sets currently reading
				// back to doc, we don't read the same #include twice.
				_, cerr := doc.file.Seek(cutOff+int64(doc.prependLengths[i]), 0)
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
	for _, d := range doc.prepends {
		_ = d.Close()
	}
	return nil
}

// helper-function for loadDocumentFromPath
// returns non-nill if ancestor has path. What then returns is a 'stack'
// of what is allIncluded by what.
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
			stack := doc.path + " -> " + *perr
			return &stack
		}
	}
	return nil
}
