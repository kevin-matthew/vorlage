package vorlage

import (
	vorlageproc "ellem.so/vorlageproc"
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

var variableRegexpProc = regexp.MustCompile(VariableRegexp)

type NormalDefinition struct {
	variable string
	value    string
	seeker   int
}

func (d *NormalDefinition) Close() error {
	d.seeker = 0
	return nil
}

var _ vorlageproc.Definition = &NormalDefinition{}

func createNormalDefinition(variable string, value string) (NormalDefinition,
	*Error) {
	ret := NormalDefinition{
		variable: variable,
		value:    value,
	}

	if strings.Contains(variable, ".") {
		err := NewError("cannot #define a processor variable")
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

func (d *NormalDefinition) GetFullName() string {
	return d.variable
}

func (d *NormalDefinition) Read(p []byte) (int, error) {
	if d.seeker == len(d.value) {
		return 0, io.EOF
	}
	n := copy(p, d.value[d.seeker:])
	if d.seeker+n >= len(d.value) {
		d.seeker = len(d.value)
		return n, io.EOF
	}
	d.seeker += n
	return n, nil
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
	ConvertedFile File

	fileInode uint64 // it may be linux-only. but this keeps us grounded,
	// now Document can be made without an actual file backing it.

	path string

	// will point to the root's. will not be nil after the document
	// is loaded. only used when the document is being read.
	// This map will have the same keys as streamArguments. The purpose of this
	// map is to keep track of what streamArguments have already been handed out
	// to processor Variables. In accordance to the manual, if a streamed input
	// is attempted to be used twice, an error will occour (errDoubleInputStream
	// will be thrown)
	// The index is the input Name, the value is which processor variable
	// had used it, if the value is "" then that means it hasn't been used yet.
	streamInputsUsed map[string]string

	root   *Document
	parent *Document

	// if root != nil,
	// then this points to the root's allDefinitions
	allDefinitions *[]NormalDefinition

	allIncluded *[]*Document // if root != nil,
	// then this points to the root's allIncluded

	documentEOF bool // while reading, will be set to true if the document (
	// plus prepends and appends) is at End of file
	convertedFileDoneReading bool // set to true if the (
	// converted) file and variables this document references has been
	// completely/outputted and all thats left is appended documents.
	//used for reading.

	// used in reading. can be nil which means not currenlty reading from one

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

	// compRequest is the original requested passed into loadDocument.
	// you must to make sure to assign compRequest.ProcessorInfo before you use
	// this in any of the processor interface functions.
	compRequest compileRequest

	// compiler is the original compiler passed into loadDocument.
	compiler *Compiler

	preProcessed bool
}

/*
 * Opens a document and recursively opens all the documents referenced by
 * #prepends. For every document that is opened,
 * the converters are first consulted (via converters[i].ShouldConvert) in
 * the order they are in the array. The first converter to return true will
 * be used. If no converters return true, the document is not converted and will
 * be read as normal (via io.OpenFile).
 */
func (compiler *Compiler) loadDocument(compReq compileRequest) (doc Document,
	oerr *Error) {
	d, err := loadDocumentFromPath(compReq.filepath, compiler, compReq, nil, nil)
	if err != nil {
		return d, err
	}

	return d, nil
}

/*
 * Gets the filename to which the document was accessed or included by.
 */
func (doc Document) GetFileName() string {
	return doc.path
}

func loadDocumentFromPath(path string,
	compiler *Compiler,
	request compileRequest,
	parent *Document,
	root *Document) (doc Document, oerr *Error) {

	oerr = &Error{}
	oerr.SetSubject(path)

	doc.MacroReadBuffer = make([]byte, MacroMaxLength)
	doc.VariableDetectionBuffer = make([]byte, len(VariablePrefix))
	doc.parent = parent
	doc.root = root
	doc.path = path
	doc.convertedFileDoneReading = false
	doc.compiler = compiler
	// zero-out the variable detection buffer
	for i := range doc.VariableDetectionBuffer {
		doc.VariableDetectionBuffer[i] = 0
	}

	// see the document struct's instructions about 'allIncluded' and
	// 'allDefinitions'
	if doc.root != nil {
		doc.allDefinitions = doc.root.allDefinitions
		doc.allIncluded = doc.root.allIncluded
		doc.compRequest = doc.root.compRequest
		doc.streamInputsUsed = doc.root.streamInputsUsed
	} else {
		doc.root = &doc
		doc.allDefinitions = &[]NormalDefinition{}
		doc.allIncluded = &[]*Document{}
		doc.compRequest = request
		doc.streamInputsUsed = make(map[string]string, len(request.allStreams))
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
		oerr.SetBecause(NewError(serr.Error()))
		_ = doc.Close()
		return doc, oerr
	}
	doc.fileInode = stat.Ino

	// now that the file is open (and converting), lets detect all macros in it
	Logger.Debugf("detecting macros in '%s'", path)
	err := doc.detectMacrosPositions()
	if err != nil {
		oerr.ErrStr = "failed to detect macros"
		oerr.SetBecause(err)
		_ = doc.Close()
		return doc, oerr
	}

	Logger.Debugf("interpreting macros in '%s'", path)
	err = doc.processMacros()
	if err != nil {
		oerr.ErrStr = "failed to interpret macros"
		oerr.SetBecause(err)
		_ = doc.Close()
		return doc, oerr
	}

	// run #prepends
	Logger.Debugf("prepending %d documents to '%s'", len(doc.prependsPos), path)
	doc.prepends = make([]*Document, len(doc.prependsPos))
	for i := 0; i < len(doc.prependsPos); i++ {
		pos := doc.prependsPos[i]
		inc, err := doc.include(strings.Join(pos.args[1:], " "))
		if err != nil {
			oerr.ErrStr = "failed to prepend document"
			oerr.SetBecause(err)
			_ = doc.Close()
			return doc, oerr
		}
		doc.prepends[i] = inc
	}

	// run #appends
	Logger.Debugf("appending %d documents to '%s'", len(doc.appendPos), path)
	doc.appends = make([]*Document, len(doc.appendPos))
	for i := 0; i < len(doc.appendPos); i++ {
		pos := doc.appendPos[i]
		inc, err := doc.include(strings.Join(pos.args[1:], " "))
		if err != nil {
			oerr.ErrStr = "failed to append document"
			oerr.SetBecause(err)
			_ = doc.Close()
			return doc, oerr
		}
		doc.appends[i] = inc
	}

	// normal definitions (#define)
	Logger.Debugf("parsing %d normal define(s) '%s'", len(doc.normalPos), path)
	for _, d := range doc.normalPos {
		def, err := createNormalDefinition(d.args[1], strings.Join(d.args[2:], " "))
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

	// set the cursor past all the #prepends, #appends, and #includes.
	_, serr = doc.rawFile.Seek(doc.rawContentStart, 0)
	if serr != nil {
		oerr.ErrStr = errFailedToSeek
		oerr.SetBecause(NewError(serr.Error()))
		_ = doc.Close()
	}

	// variables we need to convert the document to the target format.
	Logger.Debugf("opening a converter to '%s'", path)
	doc.ConvertedFile, err = doc.getConverted(osFileToFile(doc.rawFile, doc.rawContentStart))
	if err != nil {
		oerr.ErrStr = errConvert
		oerr.SetBecause(err)
		_ = doc.Close()
		return doc, oerr
	}

	return doc, nil
}

func bytesAreString(buff []byte, str string, offset int) bool {
	return offset+len(str) <= len(buff) &&
		string(buff[offset:offset+len(str)]) == str
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
		pos.length = 0
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
		oerr.ErrStr = "macro prefix detected but no macro present"
		oerr.SetSubjectf(pos.ToString())
		return pos, oerr
	}

	// todo: what if macro is to long
	//append(pos.args, )
	tmp := strings.Split(string(buffer[:pos.length-uint(len(EndOfLine))]),
		string(MacroArgument))
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
		lastBuffer = err == io.EOF && n == 0
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

		if pos.length == 0 {
			doc.rawContentStart = at
			Logger.Debugf("finished detecting macros in '%s'", doc.path)
			return nil
		}

		Logger.Debugf("detected macro '%s' in %s", pos.args[0], doc.path)
		doc.macros = append(doc.macros, pos)

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

	// make sure we dont re-include anything
	for _, d := range *doc.allIncluded {
		if d.fileInode == stat.Ino {
			Logger.Debugf("avoiding a re-opening of document '%s' (inode match)",
				path)
			return d, nil
		}
	}

	adoc, err := loadDocumentFromPath(relPath,
		doc.compiler,
		doc.compRequest,
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

//TODO: VIOLATION: This document is exceeding 500 lines.
func (doc Document) findDefinitionByName(FullName string) *NormalDefinition {
	for i := 0; i < len(*doc.allDefinitions); i++ {
		d := (*doc.allDefinitions)[i]
		if d.GetFullName() == FullName {
			return &d
		}
	}
	return nil
}

// if n < len(p) it's probably because you are about to read a macro,
// simply read again and you'll read the expanded macro. In other words,
// any time there's a macro in the file, read is forced to start there and be
// truncated on the call before.
//
// that being said, len(p) >= MacroMaxLength.
//
// Calling Read on a document on a thread that is different from the original
// thread the document was created on (via Compiler.Compile) is undefined behaviour.
func (doc *Document) Read(dest []byte) (int,
	error) {
	// the caller is requesting we read from this document even though we've
	// previously returned an EOF... so lets reset
	// todo: the caller should be doing this explicitly... why did I put this here?
	// may just have to remove.
	if doc.documentEOF {
		Logger.Debugf("rewinding EOF'd document '%s' for reading", doc.path)
		cerr := doc.Rewind()
		if cerr != nil {
			oerr := NewError(errFailedToReadPrependDocument)
			oerr.SetSubject(doc.prepends[doc.prependReadingIndex].path)
			oerr.SetBecause(NewError(cerr.Error()))
			return 0, oerr
		}
		doc.documentEOF = false
		doc.convertedFileDoneReading = false
	}

	// If we have prepends that we haven't read, keep reading those.
	if doc.prependReadingIndex < len(doc.prepends) {
		n, cerr := doc.prepends[doc.prependReadingIndex].Read(dest)
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

	// At this point, we're not reading a prepended file.
	// Now the question is, are we done reading the content of the actual docmnet?...
	if !doc.convertedFileDoneReading {
		// ...we're not. so lets continue reading the content from this document
		Logger.Debugf("reading (converted) document to buffer %s", doc.path)
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
			Logger.Debugf("document '%s' reading return EOF, "+
				"will no longer read it", doc.path)
			doc.convertedFileDoneReading = true
		}
		return n, nil
	}

	// well okay looks like the document itself has been fully read.
	// lets read from appended files now...
	if doc.appendReadingIndex < len(doc.appends) {
		Logger.Debugf("reading from appended file %s", doc.path)

		n, cerr := doc.appends[doc.appendReadingIndex].Read(dest)
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
	doc.documentEOF = true
	return 0, io.EOF
}

// Calling Rewind on a document on a thread that is different from the original
// thread the document was created on (via Compiler.Compile) is undefined behaviour.
func (doc *Document) Rewind() error {
	Logger.Debugf("rewinding document %s", doc.path)
	cerr := doc.ConvertedFile.Rewind()
	if cerr != nil {
		oerr := NewError(errRewind)
		oerr.SetSubject(doc.path)
		oerr.SetBecause(NewError(cerr.Error()))
		return oerr
	}
	for i := 0; i < doc.prependReadingIndex; i++ {
		cerr := doc.prepends[i].Rewind()
		if cerr != nil {
			oerr := NewError(errRewind)
			oerr.SetSubject(doc.path)
			oerr.SetBecause(NewError(cerr.Error()))
			return oerr
		}
	}

	for i := 0; i < doc.appendReadingIndex; i++ {
		cerr := doc.appends[i].Rewind()
		if cerr != nil {
			oerr := NewError(errRewind)
			oerr.SetSubject(doc.path)
			oerr.SetBecause(NewError(cerr.Error()))
			return oerr
		}
	}
	doc.appendReadingIndex = 0
	doc.prependReadingIndex = 0
	doc.documentEOF = false
	doc.convertedFileDoneReading = false
	return nil
}

// Closes the document.
// Calling Close on a document on a thread that is different from the original
// thread the document was created on (via Compiler.Compile) is undefined behaviour.
func (doc *Document) Close() error {

	// close self
	Logger.Debugf("closing '%s'",
		doc.path)
	if doc.rawFile != nil {
		_ = doc.rawFile.Close()
	}
	if doc.ConvertedFile != nil {
		_ = doc.ConvertedFile.Close()
	}

	// close child docs
	for _, d := range doc.prepends {
		// in the case that a prepend failed to load,
		// it will be nil.
		if d == nil {
			continue
		}
		_ = d.Close()
	}
	for _, d := range doc.appends {
		if d == nil {
			continue
		}
		_ = d.Close()
	}

	// does this mark the finish of the compRequest?
	if doc.root == nil {
		// we just closed the root document. Which means this compRequest
		// has been finished. So call the onFinish to the vorlageproc.
		for i := range doc.compiler.processors {
			rinfo := doc.compRequest.processorRInfos[i]
			doc.compiler.processors[i].OnFinish(rinfo, *rinfo.Cookie)
		}
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
