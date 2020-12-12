default: build



GOFILES  := $(shell find . -name '*.go' -type f)


build: build/vorlage-http

test: build/procs/libtest-proc.so build/vorlage-http
	build/vorlage-http testing/testing.conf

build/procs/libtest-proc.so: testing/test-proc.c c.src/processors.h c.src/processor-interface.h
	@mkdir -p build/procs
	gcc -o build/procs/libtest-proc.so -Wall -shared -fpic testing/test-proc.c

build/vorlage-http: $(GOFILES)
	go build -ldflags "-s -w" -o build/vorlage-http ./http/




.PHONEY: build test default
