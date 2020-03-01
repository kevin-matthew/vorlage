package main

import (
	"fmt"
	"strconv"
	"strings"
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
	errFailedToReadBytes = "failed to read bytes from stream"
	errFailedToSeek      = "failed to seek through file"
	errNotDefined        = "a variable was left undefined"
)

type macoPos struct {
	args  []string
	charPos uint64
	length  uint
	linenum uint
}

//const EndOfLine   = "\n#"
const MacroArgument = ' '  //todo: just rename this to 'macrospace'
const MacroPrefix = "#"
const DefineStr  = "define"
const PrependStr = "prepend"
const AppendStr  = "append"
const EndOfLine = "\n"
const VariablePrefix = "$"
const MacroMaxLength = 1024
const MaxVariableLength = 32

func bytesAreString(buff []byte, str string, offset int) bool {
	return offset+len(str) <= len(buff) &&
		string(buff[offset:offset+len(str)]) == str
}

// helper-function for detectMacrosPositions
// simply looks at the buffer and scans a macro out of it. It returns the
// length of the line, and any possible error. If 0 length is returned,
// no more macros are left to scan.
func scanMaco(buffer []byte, charsource int64,
	linenum uint) (pos macoPos, oerr *Error) {

	// first off, do we even have a valid macro?
	if !bytesAreString(buffer, MacroPrefix, 0) {
		// no this isn't a macro... so we're done looking for macros.
		return pos,nil
	}



	// get length
	pos.linenum = linenum
	pos.charPos = uint64(charsource)
	pos.length = uint(len(MacroPrefix)) // we skip scanning the macro prefix
	for ; pos.length < uint(len(buffer)); pos.length++ {
		// grab the end of the line

		// first see if we can get to '\n'...
		if bytesAreString(buffer, EndOfLine, int(pos.length)) {
			// cut out the end of the line
			pos.length += uint(len(EndOfLine))
			break
		}
	}
	if pos.length <= uint(len(MacroPrefix)) {
		oerr = &Error{}
		oerr.ErrStr = "macro prefix detected but nothing defined"
		oerr.SetSubjectf("line %d", pos.linenum)
		return pos, oerr
	}

	// todo: what if macro is to long
	//append(pos.args, )
	tmp := strings.Split(string(buffer[:pos.length]), string(MacroArgument))
	pos.args = []string{}
	for _,t := range tmp {
		if t != "" {
			pos.args = append(pos.args, t)
		}
	}
	return pos,nil
}

func main() {
	p,e := scanMaco([]byte("#a"), 0,0)
	fmt.Printf("%s/%v",strings.Join(p.args, "-"),e)
}