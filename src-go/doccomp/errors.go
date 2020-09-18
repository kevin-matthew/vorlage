package doccomp

import (
	"fmt"
	"strconv"
)

type Error struct {
	ErrStr  string
	Subject string //optional, can be ""
	Because *Error //optional
}

func NewError(ErrStr string) *Error {
	return &Error{ErrStr: ErrStr}
}

func (e Error) Error() string {
	ret := e.ErrStr
	if e.Subject != "" {
		ret += " (" + e.Subject + ")"
	}
	if e.Because != nil {
		ret += ": " + e.Because.Error()
	}
	return ret
}

// only set subject to a variable that was passed through the arguments of
//the scope.
func (e *Error) SetSubject(subject string) {
	e.Subject = subject
}

func (e *Error) SetSubjectf(format string, args ...interface{}) {
	e.Subject = fmt.Sprintf(format, args...)
}

func (e *Error) SetSubjectInt(subject int) {
	str := strconv.Itoa(subject)
	e.Subject = str
}

func (e *Error) SetBecause(because *Error) {
	e.Because = because
}

// highlights the last error (the root error) on the stack.
func (e Error) ErrorHighlight() string {
	ret := e.ErrStr
	if e.Subject != "" {
		ret += " (" + e.Subject + ")"
	}
	if e.Because != nil {
		ret += ": " + e.Because.ErrorHighlight()
	} else {
		return "\033[1;31m" + ret + "\033[0m"
	}
	return ret
}

var errNotImplemented = &Error{ErrStr: "not implemented"}

const (
	errNoProcessor                  = "no proccessor found"
	errFailedToReadBytes            = "failed to read bytes from stream"
	errFailedToReadDocument         = "failed to read from document"
	errFailedToReadPrependDocument  = "failed to read from prepended document"
	errRewind                       = "cannot rewind"
	errFailedToReadAppendedDocument = "failed to read from appended document"
	errFailedToReadVariable         = "failed to read variable"
	errFailedToSeek                 = "failed to seek through file"
	errConvert                      = "could not convert file"
	errNotDefined                   = "a variable was left undefined"
	errResetVariable                = "failed to reset variable"
	errAlreadyDefined               = "variable has already been defined"
	errVariableTooLong              = "variable too long"
	errVariableMissingSuffix        = "'$(' detected but no ')'"
	errVariableMissingPrefix        = "'$(' not detected"
	errBufferTooShort               = "the buffer is too short"
	errVariableDraw                 = "cannot draw variable"
	errVariableName                 = "variable has an invalid name"
)
