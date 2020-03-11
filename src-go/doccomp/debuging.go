package doccomp

import (
	"fmt"
	"os"
)

func Debugf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	_, _ = fmt.Printf("[ %s%.6s \033[0m] %s\n",
		"\033[1;36m",
		"DEBUG",
		message)
}

// anytime an interface... such as a converter, processor, or caching does not
// follow the rules, use this to output an error of what rule they broke.
func InterfaceErrorf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(os.Stderr, "[ %s%.6s \033[0m] %s\n",
		"\033[1;31m",
		"FUCK",
		message)
}
