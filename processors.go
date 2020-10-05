package doccomp


type Rid uint64

type ProcessorInfo struct {
	Description string

	// returns a list of Processor-Variable
	// Names. Note this may be called multiple times so it's best to make the
	//list as static as possible.
	Variables []ProcessorVariable
}

type Processor interface {
	Info() ProcessorInfo

	// Called when the document compiler backend gets a new request, the request
	// is given a unique Rid
	Process(Rid) Definer
}


type Definer interface {
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
