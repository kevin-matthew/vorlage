/*
 * Note: for documentation on these structs, please see c.src/processors.h
 */

package vorlage

import (
	"io"
)

type Rid uint64

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
	Close() error
}
type InputPrototype struct {
	Name        string
	Description string
}
type ProcessorInfo struct {
	// Name will be set by the loader, do not set this in the processor.
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
	RequestInfo  *RequestInfo
	ProcVarIndex int
	Input        []string
	StreamInput  []StreamInput
}

// Input will be associtive to InputPrototype
type Input struct {
	string
}

// RequestInfo is sent to processors
type RequestInfo struct {
	ProcessorInfo *ProcessorInfo

	Filepath string

	Input       []string
	StreamInput []StreamInput

	// Rid will be set by Compiler.Compile (will be globally unique)
	// treat it as read-only.
	Rid Rid

	cookie *interface{}
}

// See doc > Loading Process
type Processor interface {
	Startup() ProcessorInfo
	OnRequest(RequestInfo, *interface{}) []Action

	// Called multiple times (after PreProcess and before PostProcess).
	// Rid will be the same used in preprocess and post process.
	// variable pointer will be equal to what was provided from Info().Variables.
	DefineVariable(DefineInfo, interface{}) Definition
	OnFinish(RequestInfo, interface{})
	Shutdown() error
}
