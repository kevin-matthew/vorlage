package doccomp

import (
	"fmt"
	"io"
)

// this file provides logging information if its enabled.

// if set to non-nil, Verbose output will be written to io.Writer.
// io errors will be ignored.
var VerboseOutput io.Writer

func verbose(message string) {
	verbosef("%s", message)
}

func verbosef(format string, args ...interface{}) {
	if VerboseOutput != nil {
		_, _ = VerboseOutput.Write([]byte(fmt.Sprintf(format, args...)))
	}
}
