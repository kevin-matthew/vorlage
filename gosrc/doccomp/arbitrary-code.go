package doccomp

//TODO: I don't think 'arbitrarycode' is a good name
type PageProcessor interface {
	Preprocess(Request) *Error
	Process(Request) ([]Definition,*Error)
	Postprocess(Request) *Error
}
