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

func (m macoPos) ToString() string {
	return fmt.Sprintf("line %d", m.linenum)
}

type Document struct {
	rawFile       *os.File
	ConvertedFile ConvertedFile

	fileInode uint64 // it may be linux-only. but this keeps us grounded,
	// now Document can be made without an actual file backing it.

	converters []DocumentConverter

	path string

	root   *Document
	parent *Document

	allDefinitions *[]NormalDefinition // if root != nil,
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

	normalPos []*macoPos // points to somewhere in macros

	//variablePos []variablePos // note: these positions are in the CONVERTED
	// file
}

/*
 * Opens a document and recursively opens all the documents referenced by
 * #prepends. For every document that is opened,
 * the converters are first consulted (via converters[i].ShouldConvert) in
 * the order they are in the array. The first converter to return true will
 * be used. If no converters return true, the document is not converted and will
 * be read as normal (via io.OpenFile).
 */
func LoadDocument(path string, converters []DocumentConverter,
	proccessorLoader ProcessorLoader) (doc Document,
	oerr *Error) {
	return loadDocumentFromPath(path, converters, nil, nil)
}

/*
 * Gets the filename to which the document was accessed or included by.
 */
func (doc Document) GetFileName() string {
	return doc.path
}

func loadDocumentFromPath(path string,
	converters []DocumentConverter,
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
	doc.converters = converters
	doc.convertedFileDoneReading = false

	// see the document struct's instructions about 'allIncluded' and
	// 'allDefinitions'
	if doc.root != nil {
		doc.allDefinitions = doc.root.allDefinitions
		doc.allIncluded = doc.root.allIncluded
	} else {
		doc.root = &doc
		doc.allDefinitions = &[]NormalDefinition{}
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
	doc.rawFile = file
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
	doc.cursorPos = doc.rawContentStart

	// TODO: right here... right before we start looking for and defining
	// variables we need to convert the document to the target format.
	doc.ConvertedFile = doc.rawFile
	for _, c := range doc.converters {
		if c.ShouldConvert(doc.path) {
			Debugf("using converter '%s' for '%s'", c.GetDescription(), path)
			converted, cerr := c.ConvertFile(doc.rawFile)
			if cerr != nil {
				oerr.ErrStr = errConvert
				oerr.SetBecause(NewError(cerr.Error()))
				_ = doc.Close()
				return doc, oerr
			}
			doc.ConvertedFile = converted
			break
		}
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

		err = doc.addDefinition(def)
		if err != nil {
			oerr.ErrStr = "failed to add normal definition"
			oerr.SetSubjectf("%s '%s'", path, d.ToString())
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

/*
 * Get's a list of all paths that are included in this document recursively.
 * Good to monitor changes.
 */
func (doc Document) GetDependants() []string {
	ret := make([]string, len(*doc.allIncluded))
	for i, d := range *doc.allIncluded {
		ret[i] = d.path
	}
	return ret
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
		n, err := doc.rawFile.ReadAt(doc.MacroReadBuffer, at)

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
func (doc *Document) addDefinition(definition NormalDefinition) *Error {
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

func (doc Document) findDefinitionByName(FullName string) *NormalDefinition {
	for i := 0; i < len(*doc.allDefinitions); i++ {
		d := (*doc.allDefinitions)[i]
		if d.GetFullName() == FullName {
			return &d
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

// Does /not/ define processor variables
func (doc *Document) ReadIgnore(dest []byte, defineProcVars bool) (int,
	error) {
	// If we have prepends that we haven't read, keep reading those.
	if doc.prependReadingIndex < len(doc.prepends) {
		Debugf("reading from prepended file %s", doc.path)
		n, cerr := doc.prepends[doc.prependReadingIndex].ReadIgnore(dest, defineProcVars)
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
		Debugf("reading (converted) document to buffer")
		n, cerr := doc.ConvertedFile.Read(dest)
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
		return n, nil
	}

	// well okay looks like the document itself has been fully read.
	// lets read from appended files now...
	if doc.appendReadingIndex < len(doc.appends) {
		Debugf("reading from appended file %s", doc.path)
		n, cerr := doc.appends[doc.appendReadingIndex].ReadIgnore(dest, defineProcVars)
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
	Debugf("closing '%s'",
		doc.path)
	if doc.rawFile != nil {
		_ = doc.rawFile.Close()
	}

	if doc.ConvertedFile != nil {
		_ = doc.ConvertedFile.Close()
	}
	return nil
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
