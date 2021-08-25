default: build

buildv := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
buildh := $(shell git rev-parse HEAD)
linkvars := -X main.buildVersion=$(buildv) -X main.buildHash=$(buildh)

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
	GO111MODULE=off go build -ldflags "$(linkvars) -s -w" -o build/vorlage-http ./http/

install: build/vorlage-http
	@mkdir -p $(DESTDIR)/usr/local/bin/
	cp build/vorlage-http $(DESTDIR)/usr/local/bin/vorlage
	@mkdir -p $(DESTDIR)/etc/vorlage
	cp testing/testing.conf $(DESTDIR)/etc/vorlage/http.conf



.PHONEY: build test default install
