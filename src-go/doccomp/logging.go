package doccomp

import (
	"fmt"
	"io"
)

// if set to non-nil, Verbose output will be written to io.Writer.
// io errors will be ignored. Verbose output will basically be very
// vocal in what the library is doing. It's good for debugging, but,
// when handling multithreaded request it may become overwelming.
var VerboseOutput io.Writer

func verbose(message string) {
	verbosef("%s", message)
}

func verbosef(format string, args ...interface{}) {
	if VerboseOutput != nil {
		_, _ = VerboseOutput.Write([]byte(fmt.Sprintf(format, args...)))
	}
}
