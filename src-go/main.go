package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const MacroArgument = ' ' //todo: just rename this to 'macrospace'
const MacroPrefix = "#"
const DefineStr = "#define"
const PrependStr = "#prepend"
const AppendStr = "#append"
const EndOfLine = "\n"
const VariablePrefix = "$("
const VariableSuffix = ")"
const VariableProcessorSeporator = "."
const VariableRegexp = `^(?:[a-z0-9]+\.)?[a-zA-Z0-9]+$`
const MacroMaxLength = 1024
const MaxVariableLength = 32

var variableRegexpProc = regexp.MustCompile(VariableRegexp)

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
	errFailedToReadBytes            = "failed to read bytes from stream"
	errFailedToReadDocument         = "failed to read from document"
	errFailedToReadPrependDocument  = "failed to read from prepended document"
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

// returns nil,nil if that means you should keep scanning
// returns pos,nil if a variable was found
// returns nil,err if an error happened while parsing
func drawParseVar(dest []byte, src []byte,
	charsource int64,
	linenum uint,
	colnum uint) (pos *variablePos, oerr *Error) {

	// we retain i for 2 reasons: 1) we can check of a loop completed and 2)
	// so if we have just scanned in the start of a new variable from src to
	// dest, we can start the normal scanning proccess from where we left off
	// when we discovered the start of the variable.
	var i = 0

	var j = 0

	// if the dest starts with null (0), then that means we haven't started
	// drawing a variable yet. So look at src to see if (and where) we should
	// start.
	if dest[0] != VariablePrefix[0] {
		for ; i < len(src) && src[i] != VariablePrefix[0]; i++ {
		}
		if i == len(src) {
			// we're not recording a variable, nor did we find the start of one
			// in src.
			return nil, nil
		}
	}

	// at this point we've just found, or have previously found at least
	// the start of a variable that is currently loaded in dest.

	// so lets find where we left off with dest (when dest[j] == 0 that means
	// we havent written to that part of it yet)
	for ; j < len(dest) && dest[j] != 0; j++ {
	}

	// now appended src to dest
	for j < len(dest) && i < len(src) {
		dest[j] = src[i]
		j++
		i++
	}

	// if the scanned in bytes is shorter than the prefix, then
	// we need to wait another scan.
	if j < len(VariablePrefix) {
		return nil, nil
	}

	scannedPos, serr := scanVariable(dest, charsource, linenum, colnum)
	if serr != nil {
		switch serr.ErrStr {
		case errVariableMissingSuffix:
			// so we didn't scan in a full variable into dest...
			// now we ask: are we out of room in dest?
			if j == len(dest) {
				// if we are, then the caller can't draw anymore. so send em the
				// error
				for j = 0; j < len(dest); j++ {
					dest[j] = 0
				}
				return nil, serr
			}
			// if we're not at the end of dest then the caller can call this
			// function more times until we indeed fill it.
			return nil, nil
		case errVariableMissingPrefix:
			// theres no prefix. which means the buffer is crap if it doesn't
			// even start right. So throw the whole thing away.
			for j = 0; j < len(dest); j++ {
				dest[j] = 0
			}
			return nil, nil

		}
		return nil, serr
	}
	for j = 0; j < len(dest); j++ {
		dest[j] = 0
	}
	return &scannedPos, nil
}

// helper-function for detectVariables
// looks at the buffer and tries to parse a variable out of it.
// The itself variable must start at the very beginning of the buffer.
func scanVariable(buffer []byte, charsource int64,
	linenum uint, colnum uint) (pos variablePos, oerr *Error) {

	if len(buffer) < len(VariablePrefix)+len(VariableSuffix) {
		// this buffer isn't big enough to even consider the possibility
		// of having a variable.
		return pos, NewError(errBufferTooShort)
	}

	var length, j, dotIndex int
	for length = 0; length < len(VariablePrefix); length++ {
		if buffer[length] != VariablePrefix[length] {
			// no valid prefix, no variable to be found here!
			return pos, NewError(errVariableMissingPrefix)
		}
	}

	for ; length < len(buffer); length++ {
		// keep scanning through until we find the VariableSuffix
		if length+len(VariableSuffix) >= len(buffer) {
			// The VariableSuffix was not found in this buffer.
			oerr = NewError(errVariableMissingSuffix)
			return pos, oerr
		}

		for j = 0; j < len(VariableSuffix); j++ {
			if buffer[length+j] != VariableSuffix[j] {
				break
			}
		}
		if j == len(VariableSuffix) {
			length = length + j
			break
		}
	}

	varName := buffer[len(VariablePrefix) : length-len(VariableSuffix)]

	if !variableRegexpProc.Match(varName) {
		oerr = NewError(errVariableName)
		oerr.SetSubjectf("'%s' at line %d", string(varName), linenum)
		return pos, oerr
	}

	dotIndex = strings.Index(string(buffer[:length]),
		VariableProcessorSeporator)
	if dotIndex == -1 {
		dotIndex = 0
	}

	pos = variablePos{
		fullName:     string(buffer[:length]),
		variableName: string(buffer[len(VariablePrefix) : length-len(VariableSuffix)]),
		processorName: string(buffer[len(VariablePrefix) : len(
			VariablePrefix)+dotIndex]),
		charPos: charsource,
		length:  uint(length),
		linenum: linenum,
		colnum:  colnum,
	}
	return pos, nil
}

type variablePos struct {
	fullName     string
	variableName string // this will be the Processor-Variable Name if
	// processorName is not ""
	processorName string // if "" then it is not a processed variable
	charPos       int64
	length        uint
	linenum       uint // used for debugging
	colnum        uint // used for debugging
}

func (v variablePos) ToString() string {
	return fmt.Sprintf("'%s', line %d, col %d", v.fullName, v.linenum, v.colnum)
}

func main() {
	dest := make([]byte, 28)
	var bufsize = 3
	src := make([]byte, bufsize)
	str := "Hello, this is my $(var) and I like to fuck $(jesus." +
		"GodDamn) and now I present to you $(bullshit"
	for i := 0; i < len(str); i += len(src) {
		for j := 0; j < len(src); j++ {
			src[j] = str[i+j]
		}
		pos, err := drawParseVar(dest, src, 0, 0, 0)
		if err != nil {
			fmt.Printf("'%s' - \"%s\" - (err=%s)\n", string(src), string(dest),
				err.Error())
		} else if pos != nil {
			fmt.Printf("'%s' - \"%s\" - (pos=%s)\n", string(src), string(dest),
				pos.fullName)
		} else {
			fmt.Printf("'%s' - \"%s\" - (pos=nil,err=nil)\n", string(src),
				string(dest))
		}

	}

}
