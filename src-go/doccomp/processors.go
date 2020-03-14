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

type ProcessorLoader interface {
	// GetProcessor should be ready to be called multiple times with the same
	// argument. So it's best to cache the Processors.
	GetProcessor(name string) (Processor, *Error)
}

//TODO: I don't think 'arbitrarycode' is a good name
type Processor interface {
	GetName() string // not present in the processor itself
	GetDescription() string
	GetVariableNames() []string // returns a list of Processor-Variable
	// Names. Note this may be called multiple times so it's best to make the
	//list as static as possible.
	DefineVariable(procVariableName string) (Definition,
		*Error) // will be called only after
	// the 'variable' string was found in GetVariableNames.
}

var _ Definition = &ProcessorDefinition{}

type ProcessorDefinition struct {
	fullname string
	parent   Processor
}

func (p ProcessorDefinition) Reset() error {
	panic("implement me")
}

func (p *ProcessorDefinition) Read(dest []byte) (int, error) {
	panic("implement me")
}

func (p ProcessorDefinition) GetFullName() string {
	return p.fullname
}

func GetProcessorVariables() ([]Definition, error) {
	return nil, nil
	//return [](Definition(ProcessorDefinition{})),nil
}

func init() {
	// do dlopen()'s
}

// todo: use package 'C' as well as dlopen to dymiaclly load all archive.
