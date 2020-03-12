package doccomp

// todo
// do we introduce the

type ConvertedFile interface {
	// same as io.ReadCloser
	Close() error

	// same as io.ReaderAt
	// must always return error if n < len(p).
	// must return io.EOF when end of file
	ReadAt(p []byte, off int64) (n int, err error)

	Read(p []byte) (n int, err error)
}

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
	ConvertFile(SourceFile) (ConvertedFile, error)

	/*
	 * For verboseness/errors/UI purposes. No functional signifigance
	 */
	GetDescription() string
}
