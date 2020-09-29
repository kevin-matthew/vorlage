package main

import (
	"./doccomp"
	"./doccomp/http"
	"./doccomp/simple"
	"net"
	"time"
)

func GetTime(args map[string]string) string {
	return time.Now().Format(time.Kitchen)
}

func main() {

	var vars = map[string]simple.CallbackDefinition{
		"Time":{"",GetTime,nil},
	}

	doccomp.Processors["myproc"] = simple.NewProcessor("My Processor", vars)

	l,_ := net.Listen("tcp", "localhost:8080")
	err := http.Serve(l, ".")
	if err != nil {
		println("return err: " + err.Error())
	}
	return
}
