package vorlage

import (
	"io"
	"regexp"
)

type Rid uint64

var validProcessorName = regexp.MustCompile(`^[a-z0-9]+$`)

type ProcessorInfo struct {
	// todo: I should probably make this private so I can make sure it loads in
	// via the filename.
	Name string

	Description string

	// returns a list ProcessorVariable pointers (that all point to valid
	// memory).
	Variables []*ProcessorVariable
}

type Processor interface {
	// called when loaded into the impl
	Info() ProcessorInfo

	// Called multiple times (after PreProcess and before PostProcess).
	// rid will be the same used in preprocess and post process.
	// variable pointer will be equal to what was provided from Info().Variables.
	DefineVariable(rid Rid, variable *ProcessorVariable) Definition
}

type ProcessorVariable struct {
	Name        string
	Description string

	// before Definer.DefineVariable is called, this map will be populated.
	// When passing into NewCompiler, the map keys need to be present, but
	// the values will be ignored.
	Input map[string]string

	// When passing into NewCompiler, the map keys need to be present, but
	// the values will be ignored.
	// before Definer.DefineVariable is called, this map will be populated
	// streamed inputs are mutually exclusive from Input.
	// StreamedInput will be passed into Processor.DefineVariable as an
	// io.Reader under the streams argument.
	StreamedInput map[string]io.Reader
}
