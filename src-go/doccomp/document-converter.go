package doccomp

import (
	"io"
)

// its a io.Reader that will read from the file but will NOT read the macros.
type SourceFile interface {
	// n will sometimes be < len(p) but that does not mean it's the end of the
	// file. Only when 0, io.EOF is returned will it be the end of the file.
	Read(p []byte) (int, error)

	// must be called when conversion is done.
	Close() error
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
	 * Convert the file and return the ConvertedFile. If Error
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

func getConverted(rawcontents io.ReadCloser) (io.ReadCloser, *Error) {
	return rawcontents, nil
}
