package vorlage

import (
	"regexp"
	"./vorlageproc"
)

var goLibraryFilenameSig = regexp.MustCompile("^lib([^.]+).go.so")

func LoadGoProcessors() ([]vorlageproc.Processor, error) {

	return nil, nil
}