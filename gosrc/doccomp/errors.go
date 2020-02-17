package doccomp

import (
	"fmt"
	"os"
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

func (e *Error) Error() string {
	ret := e.ErrStr
	if e.Subject != "" {
		ret += " (" + e.Subject + ")"
	}
	if e.Because != nil {
		ret += e.Because.Error()
	}
	return ret
}

func (e *Error) SetSubject(subject string) {
	e.Subject = subject
}

func (e *Error) SetSubjectInt(subject int) {
	str := strconv.Itoa(subject)
	e.Subject = str
}

func (e *Error) SetBecause(because *Error) {
	e.Because = because
}

func (e *Error) Output() {
	_, _ = fmt.Fprintf(os.Stderr, "%s\n", e.Error())
}

var errNotImplemented = &Error{ErrStr: "not implemented"}

const (
	errFailedToReadBytes = "failed to read bytes from stream"
	errFailedToSeek      = "failed to seek through file"
	errNotDefined        = "a variable was left undefined"
)
