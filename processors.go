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

/*
 * note: I was going too have this as an interface so that way I can load
 * proocessors multiple ways. But I'll transfer these methods to namespace-wide.
type ProcessorLoader interface {
	// GetProcessor should be ready to be called multiple times with the same
	// argument. So it's best to cache the Processors.
	GetProcessor(Name string) (*Processor, error)

	// AddProcessor adds a p
	AddProcessor(Name string, processor Processor)
}*/

type Processor interface {

	// Description of the proccessor
	GetDescription() string

	// returns a list of Processor-Variable
	// Names. Note this may be called multiple times so it's best to make the
	//list as static as possible.
	GetVariables() []ProcessorVariable

	// defines a given variable only if that variable was a match to what
	// was provided by GetVariables, thus this method will never be called with
	// unfimiliar arguments to the processor.
	// All errors returned by this method will simply be logged. def WILL ALWAYS
	// be used to define the processor variable.
	DefineVariable(name string,
		input Input,
		streams StreamInput) (def Definition, err error)
}

type ProcessorVariable struct {
	Name        string
	Description string
	InputNames  []string

	// streamed inputs are mutually exclusive from InputNames.
	// StreamedInputNames will be passed into Processor.DefineVariable as an
	// io.Reader under the streams argument.
	StreamedInputNames []string
}

// This is the source of all processors. Add to this list if you
// want to add your own processor. They're mapped via their Name.
var Processors map[string]Processor = make(map[string]Processor)
