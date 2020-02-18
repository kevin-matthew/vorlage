package doccomp

// todo
// do we introduce the

type TargetFile interface {
	// same as io.ReadCloser
	Read(p []byte) (int, error)
	Close() error

	// same as io.ReaderAt
	ReadAt(p []byte, off int64) (n int, err error)

	// same as io.Seaker
	Seek(offset int64, whence int) (int64, error)
}

// its a io.Reader that will read from the file but will NOT read the macros.
type SourceFile interface {
	// n will sometimes be < len(p) but that does not mean it's the end of the
	// file. Only when 0, io.EOF is returned will it be the end of the file.
	Read(p []byte) (int, error)
}

type DocumentConverter interface {
	/*
	 * ShouldConvert is called to see if this particular document converter
	 * should handler the conversion of the file. If true is returned,
	 * ConverFile will be called. If false is returned,
	 * the next available document converter will be asked the same question.
	 * On error, the document's loading is stopped completely.
	 */
	ShouldConvert(path string) (bool, error)

	/*
	 * Convert the file and return the TargetFile. If Error
	 * is non-nil, the document's loading is stopped completely.
	 */
	ConvertFile(SourceFile) (TargetFile, error)

	/*
	 * For verboseness/errors/UI purposes. No functional signifigance
	 */
	GetDescription() string
}
