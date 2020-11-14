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

const (
	// General
	ActionCritical   = 0x1
	ActionAccessFail = 0xd

	// http only
	ActionHttpRedirect = 0x47790001
	ActionHttpCookie   = 0x47790002
)

type Action struct {
}

type Processor interface {
	// called when loaded into the impl
	Info() ProcessorInfo

	// todo: should I send OnRequest to all processors even those who have no
	//       variables present on the document? Or should I put a level of
	//       abstraction between the webserver and processors (ie multiple webservers?)
	OnRequest(Request) []Action

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
