package doccomp

//TODO: I don't think 'arbitrarycode' is a good name
type Processor interface {
	GetName() string // not present in the processor itself
	GetDescription() string
	GetVariableNames() []string // returns a list of Processor-Variable Names

	ReadVariable(variable string, p []byte) (bytesRead int, err error)
}

// todo: use package 'C' as well as dlopen to dymiaclly load all archive.