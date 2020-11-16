package vorlage

import (
	"regexp"
)

type Rid uint64

var validProcessorName = regexp.MustCompile(`^[a-z0-9]+$`)

type ProcessorInfo struct {
	// todo: I should probably make this private so I can make sure it loads in
	// via the filename.
	Name string

	Description string

	InputProto []InputPrototype
	StreamInputProto []InputPrototype


	// returns a list ProcessorVariable pointers (that all point to valid
	// memory).
	Variables []ProcessorVariable
}
type ProcessorVariable struct {
	Name        string
	Description string
	InputProto []InputPrototype
	StreamInputProto []InputPrototype
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

type ExitInfo struct {
}
type DefineInfo struct {
	*RequestInfo
	*ProcessorVariable
	Input
	StreamInput
}

type Processor interface {
	// called when loaded into the impl
	Startup() ProcessorInfo

	OnRequest(RequestInfo) []Action

	// Called multiple times (after PreProcess and before PostProcess).
	// rid will be the same used in preprocess and post process.
	// variable pointer will be equal to what was provided from Info().Variables.
	DefineVariable(DefineInfo) Definition

	Shutdown() ExitInfo
}


