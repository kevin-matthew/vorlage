default: build



GOFILES  := $(shell find . -name '*.go' -type f)


build: build/vorlage-http

test: build/procs/libctest.so build/procs/golibgotest.so build/vorlage-http
	build/vorlage-http testing/testing.conf

build/procs/golibgotest.so: testing/proctest.go $(wildcard vorlageproc/*.go)
	@mkdir -p build/procs
	go build -buildmode=plugin -o $@ $<

build/procs/libctest.so: testing/proctest.c c.src/processors.h c.src/processor-interface.h
	@mkdir -p build/procs
	gcc -o $@ -Wall -shared -fpic $<

build/vorlage-http: $(GOFILES)
	go build -ldflags "-s -w" -o build/vorlage-http ./http/




.PHONEY: build test default
