package procload

import (
	"regexp"
)

var goLibraryFilenameSig = regexp.MustCompile("^lib([^.]+).go.so")

func LoadGoProcessors() ([]Processor, error) {

	return nil, nil
}