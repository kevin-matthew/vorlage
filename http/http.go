package main

import (
	vorlage ".."
	"fmt"
	"net"
	"os"
)

func main() {

	// bind to the address we'll be using for http request
	l, err := net.Listen("tcp", "127.0.0.1:8050")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to listen: "+err.Error()+"\n")
		os.Exit(1)
	}

	FileExt = append(FileExt, ".html")
	procs, err := vorlage.LoadCProcessors()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load c processors: "+err.Error()+"\n")
		os.Exit(1)
		return

	}

	err = Serve(l, procs, false, ".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "http server exited: "+err.Error()+"\n")
		os.Exit(1)
		return
	}

}
