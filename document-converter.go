package vorlage

import (
	vorlageproc "./vorlage-interface/golang/vorlageproc"
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
	currentlyReadingDef vorlageproc.Definition
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
	 * is non-nil, the document's procload is stopped completely.
	 * note that the SourceFile:Close MUST be called before this function
	 * returns.
	 */
	ConvertFile(reader io.Reader) (io.ReadCloser, error)

	/*
	 * For verboseness/errors/UI purposes. No functional signifigance
	 */
	GetDescription() string
}

var _ File = &nonConvertedFile{}

func (doc *Document) getConverted(sourceFile File) (converedFile File, err *Error) {
	// todo: switch on the source file Name to find a good converted (haml->html)
	file := nonConvertedFile{
		sourceFile:         sourceFile,
		sourceDocument:     doc,
		variableReadBuffer: make([]byte, MaxVariableLength),
	}
	return &file, nil
}
