# Multi-stage Dockerfile for OpenMorph
# Production-ready container for CI/CD pipelines

# Build stage
FROM golang:1.24.4-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code (only what's needed for build)
COPY main.go ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Accept version and target architecture as build arguments
ARG VERSION=dev
ARG TARGETARCH

# Build the binary with optimizations for container usage
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -ldflags="-w -s -extldflags '-static' -X 'github.com/developerkunal/OpenMorph/cmd.version=${VERSION}'" \
    -a -installsuffix cgo \
    -o openmorph \
    ./main.go

# Final stage - minimal production image with Node.js for swagger-cli
FROM node:22-alpine

# Install swagger-cli globally
RUN npm install -g @apidevtools/swagger-cli

# Create non-root user for security
RUN addgroup -g 1001 -S openmorph && \
    adduser -u 1001 -S openmorph -G openmorph

# Copy certificates and timezone data from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the binary from builder
COPY --from=builder /build/openmorph /usr/local/bin/openmorph

# Make binary executable
RUN chmod +x /usr/local/bin/openmorph

# Switch to non-root user
USER openmorph

# Set working directory
WORKDIR /workspace

# Set binary as entrypoint
ENTRYPOINT ["/usr/local/bin/openmorph"]

# Default to help command
CMD ["--help"]

# Add labels for better container management
LABEL maintainer="developerkunal" \
      version="latest" \
      description="OpenMorph - Transform OpenAPI vendor extension keys" \
      org.opencontainers.image.title="OpenMorph" \
      org.opencontainers.image.description="Production-grade CLI tool for transforming OpenAPI vendor extension keys" \
      org.opencontainers.image.vendor="developerkunal" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.source="https://github.com/developerkunal/OpenMorph"

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/openmorph", "--version"] || exit 1
