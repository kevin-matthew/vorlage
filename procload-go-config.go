package vorlage

import "regexp"

var goLibraryFilenameSig = regexp.MustCompile(`^lib([^.]+)\.go\.so`)
var GoPluginLoadPath = "/lib/usr/vorlage/go"
