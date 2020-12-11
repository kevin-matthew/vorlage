default: build



GOFILES  := $(shell find . -name '*.go' -type f)


build: build/vorlage-http

build/vorlage-http: $(GOFILES)
	go build -ldflags "-s -w" -o build/vorlage-http ./http/ 


.PHONEY: build test default
