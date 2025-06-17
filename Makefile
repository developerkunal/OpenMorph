# Makefile for OpenMorph

BINARY=openmorph
VERSION_FILE=.version
VERSION=$(shell cat $(VERSION_FILE) 2>/dev/null || echo "0.0.0")

.PHONY: all build test lint release clean install version-show version-bump-patch version-bump-minor version-bump-major version-set version-tag version-release

all: build

build:
	go build -ldflags "-X github.com/developerkunal/OpenMorph/cmd.version=v$(VERSION)" -o $(BINARY) .

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
	go build -ldflags "-X github.com/developerkunal/OpenMorph/cmd.version=v$(VERSION)" -o $(BINARY) .
	mv $(BINARY) "$(GOPATH)/bin/$(BINARY)"

# Version management targets
version-show:
	@./scripts/version.sh show

version-bump-patch:
	@./scripts/version.sh bump patch

version-bump-minor:
	@./scripts/version.sh bump minor

version-bump-major:
	@./scripts/version.sh bump major

version-set:
	@read -p "Enter version: " version; ./scripts/version.sh set $$version

version-tag:
	@./scripts/version.sh tag

version-release:
	@./scripts/version.sh release

# Setup package managers
setup-packages:
	@./scripts/setup-package-managers.sh

# Validate setup
validate:
	@./scripts/validate-setup.sh
