# Makefile for OpenMorph

BINARY=openmorph

.PHONY: all build test lint release clean install

all: build

build:
	go build -o $(BINARY) .

test:
	go test ./... -v

lint:
	golangci-lint run

release:
	goreleaser release --clean

snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -f $(BINARY)
	rm -rf dist/

install:
	go build -o $(BINARY) .
	mv $(BINARY) "$(GOPATH)/bin/$(BINARY)"
