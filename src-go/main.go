package main

import (
	"net"
	"./doccomp/http"
)

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
	l,_ := net.Listen("tcp", "localhost:8080")
	err := http.Serve(l, ".")
	if err != nil {
		println("return err: " + err.Error())
	}
	return
}
