package vorlage

import "regexp"

var goLibraryFilenameSig = regexp.MustCompile("^golib([^.]+).so")
var GoPluginLoadPath = "go.src"

