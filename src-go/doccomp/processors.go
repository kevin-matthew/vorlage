package doccomp

import "io"

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

type Processor interface {
	// not present in the processor itself.. but in the filename
	GetName() string

	// description of the proccessor
	GetDescription() string

	// returns a list of Processor-Variable
	// Names. Note this may be called multiple times so it's best to make the
	//list as static as possible.
	GetVariables() []ProcessorVariable

	// defines a given variable only if that variable was a match to what
	// was provided by GetVariables.
	// All errors returned by this method will simply be logged. def WILL ALWAYS
	// be used to define the processor variable.
	DefineVariable(name string,
		input map[string]string,
		streams map[string]io.Reader) (def Definition, err error)
}

type ProcessorVariable struct {
	name        string
	description string
	inputNames  []string

	// streamed inputs are mutually exclusive from inputNames.
	// streamedInputNames will be passed into Processor.DefineVariable as an
	// io.Reader under the streams argument.
	streamedInputNames []string
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
