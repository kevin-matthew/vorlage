package vorlage

import (
	"io"
	"regexp"
)

type Rid uint64

var validProcessorName = regexp.MustCompile(`^[a-z0-9]+$`)

/*
 * This is a definition, they can be made either by using '#define' in a file or
 * if the page processor
 */
type Definition interface {
	// reset the reader to the beginning,
	// this is called before the every instance of the variable by the loader
	// Thus repetitions of large definitions should be advised against,
	// or at least have a sophisticated caching system.
	Reset() error

	// must return EOF when complete (no more bytes left to read)
	Read(p []byte) (int, error)

	//
	Close() error

	// needed for content-length to be sent.
	// if nil is returned, doccomp will not calculate nor send content-length.
	// however this is not prefered and should be only used for applications
	// that truelly cannot know what their content length will be.
	//Length() *uint64
}

// simply a list of variables
type InputPrototype struct {
	name        string
	description string
}
type ProcessorInfo struct {
	// todo: I should probably make this private so I can make sure it loads in
	// via the filename.
	Name string

	Description string

	InputProto       []InputPrototype
	StreamInputProto []InputPrototype

	// returns a list ProcessorVariable pointers (that all point to valid
	// memory).
	Variables []ProcessorVariable
}
type ProcessorVariable struct {
	Name             string
	Description      string
	InputProto       []InputPrototype
	StreamInputProto []InputPrototype
}

const (
	// General
	ActionCritical   = 0x1
	ActionAccessFail = 0xd
	ActionSee        = 0xb

	// http only
	ActionHTTPHeader = 0x47790002
)

type Action struct {

	// see above enum
	Action int
	Data   []byte
}

type StreamInput interface {
	io.Reader
	io.Closer
}

type ExitInfo struct {

}

type DefineInfo struct {
	*RequestInfo
	ProcVarIndex  int
	Input       []string
	StreamInput []StreamInput
	Context       interface{}
}

// Input will be associtive to InputPrototype
type Input struct {
	string
}

// RequestInfo is sent to processors
type RequestInfo struct {
	*ProcessorInfo

	Filepath string

	// muse be same lengths as ProcessorInfo.InputProto and
	// ProcessorInfo.StreamInputProto otherwise everything goes to shit.
	Input []string
	// list of file descriptors
	StreamInput []StreamInput

	// Rid will be set by Compiler.Compile (will be globally unique)
	// treat it as read-only.
	Rid
}

type Processor interface {
	// called when loaded into the impl
	Startup() ProcessorInfo

	OnRequest(RequestInfo, *interface{}) []Action

	// Called multiple times (after PreProcess and before PostProcess).
	// rid will be the same used in preprocess and post process.
	// variable pointer will be equal to what was provided from Info().Variables.
	DefineVariable(DefineInfo, interface{}) Definition

	OnFinish(RequestInfo, interface{})

	Shutdown() error
}
