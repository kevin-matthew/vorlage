package main

import vorlageproc "../vorlageproc"

func main() {

}

func Startup() vorlageproc.ProcessorInfo {
	return vorlageproc.ProcessorInfo{
		Name:             "",
		Description:      "",
		InputProto:       nil,
		StreamInputProto: nil,
		Variables:        nil,
	}
}

func OnRequest(vorlageproc.RequestInfo, *interface{}) []vorlageproc.Action {
	return nil
}

func DefineVariable(vorlageproc.DefineInfo, interface{}) vorlageproc.Definition {
	return nil
}

func OnFinish(vorlageproc.RequestInfo, interface{}) {

}

func Shutdown() error {

}

