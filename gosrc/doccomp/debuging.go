package doccomp

import "fmt"

func Debugf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	_, _ = fmt.Printf("[ %s%.6s \033[0m] %s\n",
		"\033[1;36m",
		"DEBUG",
		message)
}
