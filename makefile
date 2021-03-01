default: build



GOFILES  := $(shell find . -name '*.go' -type f)


build: build/vorlage-http

test: build/procs/libctest.so build/procs/libgotest.go.so build/vorlage-http
	build/vorlage-http testing/testing.conf

build/procs/libgotest.go.so: testing/proctest.go $(wildcard vorlageproc/*.go)
	@mkdir -p build/procs
	go build -buildmode=plugin -o $@ $<

build/procs/libctest.so: testing/proctest.c vorlage-interface/shared-library/processors.h vorlage-interface/shared-library/processor-interface.h
	@mkdir -p build/procs
	gcc -o $@ -Wall -shared -fpic $<

build/vorlage-http: $(GOFILES)
	go build -ldflags "-s -w" -o build/vorlage-http ./http/

install: build/vorlage-http
	cp build/vorlage-http /usr/local/bin/vorlage
	mkdir /etc/vorlage
	cp testing/testing.conf /etc/vorlage/http.conf



.PHONEY: build test default install
