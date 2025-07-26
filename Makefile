# Makefile for OpenMorph

BINARY=openmorph
VERSION_FILE=.version
VERSION=$(shell cat $(VERSION_FILE) 2>/dev/null || echo "0.0.0")

.PHONY: all build test lint format lint-fix lint-all security security-json release clean install help version-show version-bump-patch version-bump-minor version-bump-major version-set version-tag version-release version-major-release version-minor-release version-patch-release version-preview setup-packages validate snapshot

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
	@echo "Security:"
	@echo "  security              Run security vulnerability scan"
	@echo "  security-json         Run security scan with JSON output"
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
	@echo "Docker Commands:"
	@echo "  docker-build          Build all Docker images (production, distroless, dev)"
	@echo "  docker-build-multiplatform Build multi-platform images (production, distroless, dev) for AMD64 + ARM64 locally"
	@echo "  docker-push-multiplatform Build and push multi-platform images (production, distroless, dev) for AMD64 + ARM64"
	@echo "  docker-test           Test Docker images with health checks"
	@echo "  docker-run            Run OpenMorph in production Docker container"
	@echo "  docker-dev            Start interactive development container"
	@echo "  docker-compose-up     Start with Docker Compose (basic)"
	@echo "  docker-compose-dev    Start development environment with Docker Compose"
	@echo "  docker-compose-ci     Run CI/CD simulation"
	@echo "  docker-clean          Clean Docker images and system"
	@echo "  docker-all            Build and test all Docker images"
	@echo "  docker-tag            Tag images for registry push"
	@echo "  docker-login          Login to GitHub Container Registry"
	@echo "  docker-push           Push tagged images to registry"
	@echo "  docker-push-override  Build and push images (overrides existing version)"
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

# Security vulnerability scanning
security:
	@echo "🔒 Running security vulnerability scan..."
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "⚠️  govulncheck not found. Install with: go install golang.org/x/vuln/cmd/govulncheck@latest"; \
		exit 1; \
	fi
	@echo "✅ Security scan completed"

# Security scan with JSON output for CI/CD
security-json:
	@echo "🔒 Running security vulnerability scan (JSON output)..."
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck -json ./...; \
	else \
		echo "⚠️  govulncheck not found. Install with: go install golang.org/x/vuln/cmd/govulncheck@latest"; \
		exit 1; \
	fi

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

# Docker Commands
docker-build:
	@echo "Building Docker images with version $(VERSION)..."
	docker build --build-arg VERSION=v$(VERSION) -t openmorph:latest .
	docker build --build-arg VERSION=v$(VERSION) -f Dockerfile.distroless -t openmorph:distroless .
	docker build --build-arg VERSION=v$(VERSION) -f Dockerfile.dev -t openmorph:dev .

docker-build-multiplatform:
	@echo "Building multi-platform Docker images with version $(VERSION)..."
	@echo "Creating buildx builder if needed..."
	docker buildx create --name multiplatform --use --driver docker-container 2>/dev/null || docker buildx use multiplatform
	docker buildx inspect --bootstrap
	@echo "Building multi-platform production images..."
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=v$(VERSION) \
		--tag ghcr.io/developerkunal/openmorph:v$(VERSION) \
		--tag ghcr.io/developerkunal/openmorph:latest \
		--tag ghcr.io/developerkunal/openmorph:v$(shell echo $(VERSION) | cut -d. -f1) \
		--tag ghcr.io/developerkunal/openmorph:v$(shell echo $(VERSION) | cut -d. -f1-2) \
		.
	@echo "Building multi-platform distroless images..."
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=v$(VERSION) \
		--file Dockerfile.distroless \
		--tag ghcr.io/developerkunal/openmorph:v$(VERSION)-distroless \
		--tag ghcr.io/developerkunal/openmorph:latest-distroless \
		.
	@echo "Building multi-platform dev images..."
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=v$(VERSION) \
		--file Dockerfile.dev \
		--tag ghcr.io/developerkunal/openmorph:v$(VERSION)-dev \
		--tag ghcr.io/developerkunal/openmorph:latest-dev \
		.
	@echo "✅ All multi-platform images built locally!"

docker-push-multiplatform:
	@echo "Pushing multi-platform Docker images with version $(VERSION)..."
	@echo "Creating buildx builder if needed..."
	docker buildx create --name multiplatform --use --driver docker-container 2>/dev/null || docker buildx use multiplatform
	docker buildx inspect --bootstrap
	@echo "Building and pushing multi-platform production images..."
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=v$(VERSION) \
		--tag ghcr.io/developerkunal/openmorph:v$(VERSION) \
		--tag ghcr.io/developerkunal/openmorph:latest \
		--tag ghcr.io/developerkunal/openmorph:v$(shell echo $(VERSION) | cut -d. -f1) \
		--tag ghcr.io/developerkunal/openmorph:v$(shell echo $(VERSION) | cut -d. -f1-2) \
		--push \
		.
	@echo "Building and pushing multi-platform distroless images..."
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=v$(VERSION) \
		--file Dockerfile.distroless \
		--tag ghcr.io/developerkunal/openmorph:v$(VERSION)-distroless \
		--tag ghcr.io/developerkunal/openmorph:latest-distroless \
		--push \
		.
	@echo "Building and pushing multi-platform dev images..."
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=v$(VERSION) \
		--file Dockerfile.dev \
		--tag ghcr.io/developerkunal/openmorph:v$(VERSION)-dev \
		--tag ghcr.io/developerkunal/openmorph:latest-dev \
		--push \
		.
	@echo "✅ All multi-platform images built and pushed!"

docker-test:
	@echo "Testing Docker images..."
	docker run --rm -v $(PWD)/scripts:/scripts openmorph:dev /scripts/healthcheck.sh || true
	docker run --rm openmorph:latest --version
	docker run --rm openmorph:distroless --version
	@echo "Creating test files..."
	@mkdir -p test-output
	@echo '{"openapi": "3.0.0", "x-test": "value"}' > test-input.json
	docker run --rm -v $(PWD):/workspace openmorph:latest --input /workspace --dry-run
	@rm -f test-input.json
	@rm -rf test-output

docker-run:
	@echo "Running OpenMorph in Docker..."
	docker run --rm -v $(PWD):/workspace openmorph:latest --help

docker-dev:
	@echo "Starting development container..."
	docker run --rm -it -v $(PWD):/workspace openmorph:dev

docker-compose-up:
	@echo "Starting with Docker Compose..."
	docker-compose up openmorph

docker-compose-dev:
	@echo "Starting development environment with Docker Compose..."
	docker-compose up openmorph-dev

docker-compose-ci:
	@echo "Running CI/CD simulation with Docker Compose..."
	docker-compose --profile ci up openmorph-ci

docker-clean:
	@echo "Cleaning Docker artifacts..."
	docker rmi openmorph:latest openmorph:distroless openmorph:dev 2>/dev/null || true
	docker system prune -f

docker-all: docker-build docker-test
	@echo "Docker build and test complete"

# Docker registry commands
docker-tag:
	@echo "Tagging Docker images for registry..."
	docker tag openmorph:latest ghcr.io/developerkunal/openmorph:v$(VERSION)
	docker tag openmorph:latest ghcr.io/developerkunal/openmorph:latest
	docker tag openmorph:latest ghcr.io/developerkunal/openmorph:v$(shell echo $(VERSION) | cut -d. -f1)
	docker tag openmorph:latest ghcr.io/developerkunal/openmorph:v$(shell echo $(VERSION) | cut -d. -f1-2)
	docker tag openmorph:distroless ghcr.io/developerkunal/openmorph:v$(VERSION)-distroless
	docker tag openmorph:dev ghcr.io/developerkunal/openmorph:v$(VERSION)-dev
	@echo "✅ Images tagged for ghcr.io/developerkunal/openmorph"

docker-login:
	@echo "Logging into GitHub Container Registry..."
	@echo "Make sure you have a GitHub token with write:packages permission"
	@echo "Run: echo \$$GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin"
	@echo "Or use: docker login ghcr.io"
	docker login ghcr.io

docker-push: docker-tag
	@echo "Pushing Docker images to GitHub Container Registry..."
	docker push ghcr.io/developerkunal/openmorph:v$(VERSION)
	docker push ghcr.io/developerkunal/openmorph:latest
	docker push ghcr.io/developerkunal/openmorph:v$(shell echo $(VERSION) | cut -d. -f1)
	docker push ghcr.io/developerkunal/openmorph:v$(shell echo $(VERSION) | cut -d. -f1-2)
	docker push ghcr.io/developerkunal/openmorph:v$(VERSION)-distroless
	docker push ghcr.io/developerkunal/openmorph:v$(VERSION)-dev
	@echo "✅ All images pushed successfully!"

docker-push-override: docker-build docker-tag
	@echo "🚨 OVERRIDING existing v$(VERSION) images with security fixes..."
	@echo "This will replace the existing images in the registry"
	@read -p "Are you sure you want to override v$(VERSION)? (y/N): " confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		echo "Pushing images..."; \
		docker push ghcr.io/developerkunal/openmorph:v$(VERSION); \
		docker push ghcr.io/developerkunal/openmorph:latest; \
		docker push ghcr.io/developerkunal/openmorph:v$(shell echo $(VERSION) | cut -d. -f1); \
		docker push ghcr.io/developerkunal/openmorph:v$(shell echo $(VERSION) | cut -d. -f1-2); \
		docker push ghcr.io/developerkunal/openmorph:v$(VERSION)-distroless; \
		docker push ghcr.io/developerkunal/openmorph:v$(VERSION)-dev; \
		echo "✅ v$(VERSION) images overridden with security fixes!"; \
	else \
		echo "❌ Push cancelled"; \
	fi
