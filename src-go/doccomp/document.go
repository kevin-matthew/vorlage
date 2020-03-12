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

func (d *NormalDefinition) Reset() error {
	d.seeker = 0
	return nil
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
	charPos       int64
	length        uint
	linenum       uint // used for debugging
	colnum        uint // used for debugging
}

func (m macoPos) ToString() string {
	return fmt.Sprintf("line %s", m.linenum)
}

func (v variablePos) ToString() string {
	return fmt.Sprintf("'%s', line %d, col %d", v.fullName, v.linenum, v.colnum)
}

type Document struct {
	file *os.File

	fileInode uint64 // it may be linux-only. but this keeps us grounded,
	// now Document can be made without an actual file backing it.

	converters       []*DocumentConverter
	proccessorLoader ProcessorLoader

	path string

	root   *Document
	parent *Document

	allDefinitions *[]Definition // if root != nil,
	// then this points to the root's allDefinitions
	allIncluded *[]*Document // if root != nil,
	// then this points to the root's allIncluded

	convertedFileDoneReading bool // set to true if the (
	// converted) file and variables this document references has been
	// completely/outputted and all thats left is appended documents.
	//used for reading.
	currentlyReadingDef Definition // points to somewhere in allDefinitions,
	// used in reading. can be nil which means not currenlty reading from one
	cursorPos int64 // used for reading

	MacroReadBuffer         []byte
	VariableDetectionBuffer []byte // used to detect variables when the
	// document is converted and being loaded
	rawContentStart int64 // used for reading

	macros []macoPos

	prepends            []*Document // points to somewhere in allIncluded
	prependReadingIndex int
	prependsPos         []*macoPos // points to somewhere in macros

	appends            []*Document // points to somewhere in allIncluded
	appendReadingIndex int
	appendPos          []*macoPos // points to somewhere in macros

	normalDefines []*NormalDefinition // points to somewhere in allDefines
	normalPos     []*macoPos          // points to somewhere in macros

	variablePos []variablePos // note: these positions are in the CONVERTED file
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

/*
 * Gets the filename to which the document was accessed or included by.
 */
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
	doc.parent = parent
	doc.root = root
	doc.path = path
	doc.proccessorLoader = proccessorLoader
	doc.converters = converters
	doc.convertedFileDoneReading = false

	// see the document struct's instructions about 'allIncluded' and
	// 'allDefinitions'
	if doc.root != nil {
		doc.allDefinitions = doc.root.allDefinitions
		doc.allIncluded = doc.root.allIncluded
	} else {
		doc.root = &doc
		doc.allDefinitions = &[]Definition{}
		doc.allIncluded = &[]*Document{}
	}

	sourceerr := doc.ancestorHasPath(path)
	if sourceerr != nil {
		oerr.ErrStr = "circular inclusion"
		oerr.SetSubject(*sourceerr)
		return doc, oerr
	}

	file, serr := os.Open(path)
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
		inc, err := doc.include(pos.args[1])
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
		inc, err := doc.include(pos.args[1])
		if err != nil {
			oerr.ErrStr = "failed to append document"
			oerr.SetBecause(err)
			_ = doc.Close()
			return doc, oerr
		}
		doc.appends[i] = inc
	}

	// set the cursor past all the #prepends, #appends, and #includes.
	_, cerr = doc.file.Seek(doc.rawContentStart, 0)
	if cerr != nil {
		oerr.ErrStr = errFailedToSeek
		oerr.SetBecause(NewError(cerr.Error()))
		_ = doc.Close()
		return doc, oerr
	}

	// TODO: right here... right before we start looking for and defining
	// variables we need to convert the document to the target format.

	err = doc.detectVariables()
	if err != nil {
		oerr.ErrStr = "failed to detect variables"
		oerr.SetBecause(err)
		_ = doc.Close()
		return doc, oerr
	}
	Debugf("detected '%d' variable uses in '%s'", len(doc.variablePos), path)

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

		// if it has an empty processor name, then its not a processor variable
		// so ignore it
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
		for i = 0; i < len(procVars); i++ {
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

	return doc, nil
}

func bytesAreString(buff []byte, str string, offset int) bool {
	return offset+len(str) <= len(buff) &&
		string(buff[offset:offset+len(str)]) == str
}

// TODO: this function does not look at the converted file,
// it looks at the raw file.... we need to have it look at the converted file.
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
		n, err := doc.file.ReadAt(doc.VariableDetectionBuffer, at)
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
				int64(at),
				linenum,
				colnum)
			if serr != nil {
				// failed to parse. send it up.
				return serr
			}
			if pos != nil {
				// success, we've found a variable.
				doc.variablePos = append(doc.variablePos, *pos)
				Debugf("found variable '%s'", pos.fullName,
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
		var scannedBytes = 0
		colnum++ // increment column num because we
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
func scanVariable(buffer []byte, charsource int64,
	linenum uint, colnum uint) (pos *variablePos, oerr *Error) {

	if len(buffer) < len(VariablePrefix)+len(VariableSuffix) {
		// this buffer isn't big enough to even consider the possibility
		// of having a variable.
		return nil, nil
	}

	var length, j, dotIndex int
	for length = 0; length < len(VariablePrefix); length++ {
		if buffer[length] != VariablePrefix[length] {
			// no valid prefix, no variable to be found here!
			return nil, nil
		}
	}

	for ; length < len(buffer); length++ {
		// keep scanning through until we find the VariableSuffix

		if length+len(VariableSuffix) >= len(buffer) {
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
			length = length + j
			break
		}
	}

	if !variableRegexpProc.Match(buffer[:length]) {
		oerr = NewError(errVariableName)
		oerr.SetSubjectf("'%s' at line %d", string(buffer[:length]), linenum)
		return nil, oerr
	}

	dotIndex = strings.Index(string(buffer[:length]),
		VariableProcessorSeporator)
	if dotIndex == -1 {
		dotIndex = 0
	}

	pos = &variablePos{
		fullName:     string(buffer[:length]),
		variableName: string(buffer[len(VariablePrefix) : length-len(VariableSuffix)]),
		processorName: string(buffer[len(VariablePrefix) : len(
			VariablePrefix)+dotIndex]),
		charPos: charsource,
		length:  uint(length),
		linenum: linenum,
		colnum:  colnum,
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
			doc.rawContentStart = at
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
	for _, d := range *doc.allIncluded {
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
	for _, d := range *doc.allDefinitions {
		if d.GetFullName() == definition.GetFullName() {
			oerr := NewError(errAlreadyDefined)
			oerr.SetSubjectf(d.GetFullName())
			return oerr
		}
	}

	*doc.allDefinitions = append(*doc.allDefinitions, definition)
	return nil
}

func (doc Document) findDefinitionByName(FullName string) Definition {
	for _, d := range *doc.allDefinitions {
		if d.GetFullName() == FullName {
			return d
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
	return doc.ReadIgnore(dest, true)
}

// todo: remove the 'ignore' bools... we need to have some way to communicate
// processor vars to the processor. Maybe they should be passed in via Read?
// hmmmm.... or something like that. or perhaps 'add proccessor' instead of
// 'add definition'.... AH YES. Let's do that.
func (doc *Document) ReadIgnore(dest []byte, defineVariables bool) (int, error) {

	// If we have prepends that we haven't read, keep reading those.
	if doc.prependReadingIndex < len(doc.prepends) {
		Debugf("reading from prepended file %s", doc.path)
		n, cerr := doc.prepends[doc.prependReadingIndex].ReadIgnore(dest, defineVariables)
		if cerr != nil && cerr != io.EOF {
			oerr := NewError(errFailedToReadPrependDocument)
			oerr.SetSubject(doc.prepends[doc.prependReadingIndex].path)
			oerr.SetBecause(NewError(cerr.Error()))
			return n, oerr
		}
		if cerr == io.EOF {
			doc.prependReadingIndex++
			return n, nil
		}
		return n, nil
	}

	// if we're currenlty reading a variable, lets continue doing that
	if doc.currentlyReadingDef != nil {
		Debugf("reading variable '%s' into buffer",
			doc.currentlyReadingDef.GetFullName())
		n, cerr := doc.currentlyReadingDef.Read(dest)
		if cerr != nil && cerr != io.EOF {
			oerr := NewError(errFailedToReadVariable)
			oerr.SetSubject(doc.currentlyReadingDef.GetFullName())
			oerr.SetBecause(NewError(cerr.Error()))
			return n, oerr
		}
		if cerr == io.EOF {
			// this variable is done being read. let's move on the next call.
			Debugf("done from reading variable '%s'",
				doc.currentlyReadingDef.GetFullName())
			doc.currentlyReadingDef = nil
		}
		return n, nil
	}

	// At this point, we're not reading a prepended file, we're not reading
	// a variable. Now the question is,
	// are we done reading the content of this doucmnet?...
	if !doc.convertedFileDoneReading {
		// ...we're not. so lets continue reading the content from this document
		// TODO: this needs to read from the converted file
		Debugf("reading document to buffer")
		n, cerr := doc.file.ReadAt(dest, doc.cursorPos)
		if cerr != nil && cerr != io.EOF {
			oerr := NewError(errFailedToReadDocument)
			oerr.SetSubject(doc.path)
			oerr.SetBecause(NewError(cerr.Error()))
			return n, oerr
		}
		if cerr == io.EOF {
			// ...we are done reading this document,
			// so lets not read it anymore in subsequent read()'s
			Debugf("document reading return EOF, will no longer read it")
			doc.convertedFileDoneReading = true
		}

		if defineVariables {
			// are there any variables in what we just read?
			for _, v := range doc.variablePos {
				for i := 0; i < n; i++ {
					if int64(i)+doc.cursorPos == v.charPos {
						// there is a variable in this buffer. let's mark it to be
						// read on the next read()
						def := doc.findDefinitionByName(v.fullName)
						if def == nil {
							oerr := NewError(errNotDefined)
							oerr.SetSubject(v.ToString())
							return n, oerr
						}
						doc.currentlyReadingDef = def
						cerr = def.Reset()
						if cerr != nil {
							oerr := NewError(errResetVariable)
							oerr.SetSubject(v.ToString())
							return n, oerr
						}
						Debugf("found variable '%s' in read buffer, "+
							"will now read from that", def.GetFullName())

						// now advance the cursor forward to where the variable is
						// plus its length so we dont read the raw variable again.
						doc.cursorPos = doc.cursorPos + int64(i) + int64(v.length)

						// only return all bytes that have been proccessed up to
						// the variable
						return i, nil
					}
				}
			}
		}

		// no variables in this buffer, so just return the generic read results
		doc.cursorPos = doc.cursorPos + int64(n)
		return n, nil
	}

	// well okay looks like the document itself has been fully read.
	// lets read from appended files now...
	if doc.appendReadingIndex < len(doc.appends) {
		Debugf("reading from appended file %s", doc.path)
		n, cerr := doc.appends[doc.appendReadingIndex].ReadIgnore(dest, defineVariables)
		if cerr != nil && cerr != io.EOF {
			oerr := NewError(errFailedToReadAppendedDocument)
			oerr.SetSubject(doc.appends[doc.appendReadingIndex].path)
			oerr.SetBecause(NewError(cerr.Error()))
			return n, oerr
		}
		if cerr == io.EOF {
			doc.appendReadingIndex++
			return n, nil
		}
		return n, nil
	}

	// well look at that. If we've made it this far,
	// then we've read all prepended files,
	// the document itself + variables, and all appended files.
	// In other words, we've got nothing left to do.
	return 0, io.EOF
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
	for _, d := range doc.appends {
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
