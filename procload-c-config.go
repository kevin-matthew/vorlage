package vorlage

import "regexp"

var goLibraryFilenameSig = regexp.MustCompile("^lib([^.]+).go.so")
var CLoadPath = "c.src"
