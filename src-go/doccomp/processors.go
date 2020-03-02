package doccomp


type RequestData struct {
	// cookies?
	// request data?
	// session?
	// should we even care about what they expect to cache their variables on?
	// what if we just allow them to pass a function pointer that when we
	//call answers if we should recalculate this?...
	//no thats dumb they might as well just manage it themselves.

	//
}


//TODO: I don't think 'arbitrarycode' is a good name
type Processor interface {
	GetName() string // not present in the processor itself
	GetDescription() string
	GetVariableNames() []string // returns a list of Processor-Variable Names

	ReadVariable(variable string, p []byte) (bytesRead int, err error)
}

// todo: use package 'C' as well as dlopen to dymiaclly load all archive.