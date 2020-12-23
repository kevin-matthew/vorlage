import (
	vorlageproc "./vorlageproc"
)

// you must define these functions. And build your project using
//   go build -buildmode=plugin

func VorlageStartup() vorlageproc.ProcessorInfo
func VorlageOnRequest(r vorlageproc.RequestInfo, i *interface{}) []vorlageproc.Action
func VorlageDefineVariable(info vorlageproc.DefineInfo, i interface{}) vorlageproc.Definition
func VorlageOnFinish(vorlageproc.RequestInfo, interface{})
func VorlageShutdown() error


