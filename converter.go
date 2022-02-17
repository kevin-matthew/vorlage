package vorlage

import (
	vorlageproc "./vorlage-interface/golang/vorlageproc"
	"os"
	"regexp"
)

// its a io.Reader that will read from the file but will NOT read the macros.
type File interface {
	// n will sometimes be < len(p) but that does not mean it's the end of the
	// file. Only when 0, io.EOF is returned will it be the end of the file.
	// As per io.Reader definition, Read can an will return a non nil error
	// with n > 0
	Read(p []byte) (n int, err error)

	// returns to the beginning of the file
	Reset() error

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

	// used for drawParser
	variableReadBuffer []byte

	// buffer used to hold what was read from the file when reading from
	// definitions
	tmpBuff []byte

	// will be nil if not currently reading.
	currentlyReadingDef vorlageproc.Definition

	// definitionStack will be used to enter subsequent defintions and define
	// those as it goes along.
	// For example,
	//
	//    #define $(Hello) My name is $(Name)
	//    $(Hello)
	//
	// When the server gets around to defining $(Hello), the first element of
	// the stack will be $(Hello), and the second element will soon become
	// $(Name). And then they will be poped out of the array as their definitions
	// finish.
	definitionStack *[]vorlageproc.Definition
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
func (o osFileHandle) Reset() error {
	_, err := o.File.Seek(o.resetPos, 0)
	return err
}
func (o osFileHandle) Close() error {
	return o.File.Close()
}

type DCInfo struct {
	PathQualifier regexp.Regexp
	Description   string
}
type Converter interface {
	Startup() DCInfo
	/*
	 * Convert must not be dynamic. It must return the same file if given
	 * the same file, as it will be cached and it is not guarenteed to be called
	 * every request.
	 */
	Convert(File) (File, error)
	Shutdown() error
}

type myconvert struct {
}

func (m myconvert) Startup() DCInfo {
	panic("implement me")
}

func (m myconvert) Convert(file File) (File, error) {
	panic("implement me")
}

func (m myconvert) Shutdown() error {
	panic("implement me")
}

var _ File = &nonConvertedFile{}

func (doc *Document) getConverted(sourceFile File) (converedFile File, err *Error) {
	// todo: switch on the source file Name to find a good converted (haml->html)
	file := nonConvertedFile{
		sourceFile:         sourceFile,
		sourceDocument:     doc,
		variableReadBuffer: make([]byte, MaxVariableLength),
		definitionStack:    new([]vorlageproc.Definition),
	}
	return &file, nil
}
