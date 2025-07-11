# Makefile for OpenMorph

BINARY=openmorph
VERSION_FILE=.version
VERSION=$(shell cat $(VERSION_FILE) 2>/dev/null || echo "0.0.0")

.PHONY: all build test lint format lint-fix lint-all release clean install help version-show version-bump-patch version-bump-minor version-bump-major version-set version-tag version-release version-major-release version-minor-release version-patch-release version-preview setup-packages validate snapshot

all: build

# Show available commands
help:
	@echo "OpenMorph Makefile Commands:"
	@echo ""
	@echo "Build & Test:"
	@echo "  build                 Build the binary"
	@echo "  test                  Run all tests"
	@echo "  install               Build and install to GOPATH/bin"
	@echo ""
	@echo "Code Quality:"
	@echo "  format                Format code with gofmt and goimports"
	@echo "  lint                  Run linters (reports issues)"
	@echo "  lint-fix              Run linters with auto-fix"
	@echo "  lint-all              Format + lint (complete quality check)"
	@echo ""
	@echo "Version Management:"
	@echo "  version-show          Show current version"
	@echo "  version-bump-patch    Bump patch version"
	@echo "  version-bump-minor    Bump minor version"
	@echo "  version-bump-major    Bump major version"
	@echo "  version-set           Set specific version (interactive)"
	@echo "  version-tag           Create git tag from current version"
	@echo ""
	@echo "Release Commands:"
	@echo "  version-release       Generic release (defaults to patch)"
	@echo "  version-patch-release Patch release (x.y.Z)"
	@echo "  version-minor-release Minor release (x.Y.0)"
	@echo "  version-major-release Major release (X.0.0)"
	@echo "  version-preview       Preview release actions (dry run)"
	@echo ""
	@echo "Release & Distribution:"
	@echo "  release               Create release with goreleaser"
	@echo "  snapshot              Create snapshot release"
	@echo ""
	@echo "Utilities:"
	@echo "  clean                 Clean build artifacts"
	@echo "  setup-packages        Setup package managers"
	@echo "  validate              Validate setup"
	@echo "  help                  Show this help message"

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

# Show current version
version-show:
	@./scripts/version.sh show

# Version bumping (without release)
version-bump-patch:
	@./scripts/version.sh bump patch

version-bump-minor:
	@./scripts/version.sh bump minor

version-bump-major:
	@./scripts/version.sh bump major

# Set specific version
version-set:
	@read -p "Enter version: " version; ./scripts/version.sh set $$version

# Create git tag from current version
version-tag:
	@./scripts/version.sh tag

# Generic release command (defaults to patch)
version-release:
	@./scripts/version.sh release

# Version release commands
version-major-release:
	@./scripts/version.sh major-release

version-minor-release:
	@./scripts/version.sh minor-release

version-patch-release:
	@./scripts/version.sh patch-release

# Version preview (dry run)
version-preview:
	@read -p "Enter release level (major/minor/patch): " level; ./scripts/version.sh preview $$level

# Setup package managers
setup-packages:
	@./scripts/setup-package-managers.sh

# Validate setup
validate:
	@./scripts/validate-setup.sh
