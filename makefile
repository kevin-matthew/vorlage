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

install: build/vorlage-http conf/vorlage.service
	@mkdir -p $(DESTDIR)/usr/bin/
	cp build/vorlage-http $(DESTDIR)/usr/bin/vorlage
	@mkdir -p $(DESTDIR)/etc/vorlage
	$(DESTDIR)/usr/bin/vorlage --default-conf > $(DESTDIR)/etc/vorlage/http-systemd.conf
	cp conf/http.conf $(DESTDIR)/etc/vorlage/http.conf
	@mkdir -p $(DESTDIR)/lib/systemd/system
	cp conf/vorlage.service $(DESTDIR)/lib/systemd/system
	@mkdir -p $(DESTDIR)/var/log
	@mkdir -p $(DESTDIR)/usr/lib/vorlage/go
	touch $(DESTDIR)/var/log/vorlage-info.log
	touch $(DESTDIR)/var/log/vorlage-error.log

build/vorlage.tar.gz:
	@mkdir -p build/deb
	DESTDIR=build/deb $(MAKE) install
	tar -czf build/vorlage.tar.gz -C build/deb .

package: build/vorlage.tar.gz

.PHONEY: build test default install package
