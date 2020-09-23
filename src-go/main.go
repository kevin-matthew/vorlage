package main

import (
	"io"
	"net"
	"./doccomp"
	"./doccomp/http"
	"strings"
)

type myProc struct {

}

type streamReader struct {
	r *strings.Reader
}

func (s streamReader) Reset() error {
	_,err := s.r.Seek(0,0)
	return err
}

func (s streamReader) Read(p []byte) (int, error) {
	return s.r.Read(p)
}

var myVars = []doccomp.ProcessorVariable{
		doccomp.ProcessorVariable{"Time", "tells the time", nil, nil},
	}

func (m myProc) GetDescription() string {
	return "my first processor"
}

func (m myProc) GetVariables() []doccomp.ProcessorVariable {
	return myVars
}

func (m myProc) DefineVariable(name string, input map[string]string, streams map[string]io.Reader) (def doccomp.Definition, err error) {
	switch name {
	case "Time":
		return streamReader{strings.NewReader("asdf")}, nil
	}
	return def,err
}

func main() {
	/*filepath := os.Args[1]
	reader,err := doccomp.Process(filepath)
	if err != nil {
		os.Stderr.WriteString("failed to read: " + err.Error() + "\n")
		os.Exit(1)
	}
	defer reader.Close()
	http.Serve()

	buffer := make([]byte, 1024)
	_,err = io.CopyBuffer(os.Stdout, reader, buffer)
	if err != nil {
		os.Stderr.WriteString( "failed to write: " + err.Error() + "\n")
		os.Exit(1)
	}*/

	doccomp.Processors["myproc"] = myProc{}

	l,_ := net.Listen("tcp", "localhost:8080")
	err := http.Serve(l, ".")
	if err != nil {
		println("return err: " + err.Error())
	}
	return
}
