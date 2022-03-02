default: build

buildv := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
buildh := $(shell git rev-parse HEAD)
linkvars := -X main.buildVersion=$(buildv) -X main.buildHash=$(buildh)

GOFILES  := $(shell find . -name '*.go' -type f)
cwd := $(shell pwd)

rebuild:
	rm -rf ./build
	$(MAKE) build

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
	GO111MODULE=off go build -gcflags=-trimpath=$(cwd) -asmflags=-trimpath=$(cwd) -ldflags "$(linkvars) -s -w" -o build/vorlage-http ./http/main

install: build/vorlage-http conf/vorlage.service
	@mkdir -pm 755 $(DESTDIR)/
	umask 0022
	@mkdir -pm 755 $(DESTDIR)/usr/bin/
	cp build/vorlage-http $(DESTDIR)/usr/bin/vorlage
	@mkdir -pm 755 $(DESTDIR)/etc/vorlage
	$(DESTDIR)/usr/bin/vorlage --default-conf > $(DESTDIR)/etc/vorlage/http-systemd.conf
	cp conf/http.conf $(DESTDIR)/etc/vorlage/http.conf
	@mkdir -pm 755 $(DESTDIR)/usr/lib/systemd/system
	cp conf/vorlage.service $(DESTDIR)/usr/lib/systemd/system
	@mkdir -pm 755 $(DESTDIR)/var/log
	@mkdir -pm 755 $(DESTDIR)/usr/lib/vorlage/go
	touch $(DESTDIR)/var/log/vorlage-info.log
	touch $(DESTDIR)/var/log/vorlage-error.log

build/vorlage.tar.gz: $(GOFILES)
	@mkdir -p build/deb
	DESTDIR=build/deb $(MAKE) install
	tar --owner=root --group=root -czf build/vorlage.tar.gz -C build/deb .

package: build/vorlage.tar.gz

.PHONEY: build test default install package
