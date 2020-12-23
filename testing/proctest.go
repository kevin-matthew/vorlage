package main

import (
	vorlageproc "../vorlage-interface/go/vorlageproc"
	"math/rand"
	"strconv"
)

func main() {

}

func VorlageStartup() vorlageproc.ProcessorInfo {
	rand.Seed(69)
	return vorlageproc.ProcessorInfo{
		Name:             "testgoproc",
		Description:      "this processor was written in go.",
		InputProto:       nil,
		StreamInputProto: nil,
		Variables:        []vorlageproc.ProcessorVariable{{
			Name:             "RandomNumber",
			Description:      "A random integer.",
			InputProto:       nil,
			StreamInputProto: nil,
		}},
	}

}

func VorlageOnRequest(r vorlageproc.RequestInfo, i *interface{}) []vorlageproc.Action {
	randomInt := rand.Int()
	*i = randomInt
	act := vorlageproc.Action{
		Action: vorlageproc.ActionHTTPHeader,
		Data:   []byte("X-golangtest: true"),
	}
	act2 := vorlageproc.Action{
		Action: vorlageproc.ActionHTTPHeader,
		Data:   []byte("X-random: " + strconv.Itoa(randomInt)),
	}
	return []vorlageproc.Action{act, act2}
}

func VorlageDefineVariable(info vorlageproc.DefineInfo, i interface{}) vorlageproc.Definition {
	switch(info.RequestInfo.ProcessorInfo.Variables[info.ProcVarIndex].Name) {
	case "RandomNumber":
		return &vorlageproc.StringBuffer{String: strconv.Itoa(i.(int))}
	}
	return nil
}

func VorlageOnFinish(vorlageproc.RequestInfo, interface{}) {

}

func VorlageShutdown() error {
	return nil
}

