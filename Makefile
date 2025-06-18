# Makefile for OpenMorph

BINARY=openmorph
VERSION_FILE=.version
VERSION=$(shell cat $(VERSION_FILE) 2>/dev/null || echo "0.0.0")

.PHONY: all build test lint format lint-fix lint-all release clean install version-show version-bump-patch version-bump-minor version-bump-major version-set version-tag version-release

all: build

build:
	go build -ldflags "-X github.com/developerkunal/OpenMorph/cmd.version=v$(VERSION)" -o $(BINARY) .

test:
	go test ./... -v

# Format code using gofmt and goimports
format:
	@echo "🎨 Formatting Go code..."
	@gofmt -s -w .
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w -local github.com/developerkunal/OpenMorph .; \
	else \
		echo "⚠️  goimports not found. Install with: go install golang.org/x/tools/cmd/goimports@latest"; \
	fi
	@echo "✅ Code formatting completed"

# Run linters (reports issues)
lint:
	@echo "🔍 Running linters..."
	@golangci-lint run
	@echo "✅ Linting completed"

# Lint and auto-fix issues where possible
lint-fix:
	@echo "🔧 Running linters with auto-fix..."
	@golangci-lint run --fix
	@echo "✅ Linting with auto-fix completed"

# Format code and run linters (complete code quality check)
lint-all: format lint
	@echo "🎯 Complete code quality check completed"

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
