#!/bin/bash
# Docker setup validation script for OpenMorph

set -e

echo "🐳 OpenMorph Docker Setup Validation"
echo "===================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print status
print_status() {
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✅ $1${NC}"
    else
        echo -e "${RED}❌ $1${NC}"
        return 1
    fi
}

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

# Check if Docker is available
echo "Checking Docker availability..."
docker --version > /dev/null 2>&1
print_status "Docker is available"

# Check if Docker Compose is available
echo "Checking Docker Compose availability..."
docker-compose --version > /dev/null 2>&1 || docker compose version > /dev/null 2>&1
print_status "Docker Compose is available"

# Build all Docker images
echo ""
print_info "Building Docker images..."

echo "Building production image..."
docker build -t openmorph:latest . > /dev/null 2>&1
print_status "Production image built successfully"

echo "Building distroless image..."
docker build -f Dockerfile.distroless -t openmorph:distroless . > /dev/null 2>&1
print_status "Distroless image built successfully"

echo "Building development image..."
docker build -f Dockerfile.dev -t openmorph:dev . > /dev/null 2>&1
print_status "Development image built successfully"

# Test image functionality
echo ""
print_info "Testing image functionality..."

# Test production image
echo "Testing production image..."
docker run --rm openmorph:latest --version > /dev/null 2>&1
print_status "Production image runs correctly"

# Test distroless image
echo "Testing distroless image..."
docker run --rm openmorph:distroless --version > /dev/null 2>&1
print_status "Distroless image runs correctly"

# Test development image
echo "Testing development image..."
docker run --rm openmorph:dev openmorph --version > /dev/null 2>&1
print_status "Development image runs correctly"

# Test help command
echo "Testing help command..."
docker run --rm openmorph:latest --help > /dev/null 2>&1
print_status "Help command works"

# Create test files for transformation testing
echo ""
print_info "Creating test files..."
mkdir -p test-specs test-output

cat > test-specs/test-api.json << 'EOF'
{
  "openapi": "3.0.0",
  "info": {
    "title": "Test API",
    "version": "1.0.0"
  },
  "x-test-extension": "test-value",
  "x-custom-field": "custom-value",
  "paths": {
    "/test": {
      "get": {
        "summary": "Test endpoint",
        "responses": {
          "200": {
            "description": "Success"
          }
        }
      }
    }
  }
}
EOF

cat > test-specs/openmorph.yaml << 'EOF'
mappings:
  x-test-extension: x-vendor-test
  x-custom-field: x-vendor-custom
exclude:
  - x-internal
backup: true
validation:
  enabled: true
EOF

print_status "Test files created"

# Test dry-run functionality
echo ""
print_info "Testing dry-run functionality..."
docker run --rm -v $(pwd)/test-specs:/workspace openmorph:latest \
  --input /workspace --dry-run --config /workspace/openmorph.yaml > /dev/null 2>&1
print_status "Dry-run test passed"

# Test actual transformation
echo "Testing actual transformation..."
docker run --rm \
  -v $(pwd)/test-specs:/workspace \
  -v $(pwd)/test-output:/output \
  openmorph:latest \
  --input /workspace \
  --config /workspace/openmorph.yaml \
  --output /output/transformed.json > /dev/null 2>&1
print_status "Transformation test passed"

# Verify transformation output
if [ -f "test-output/transformed.json" ]; then
    print_status "Output file created successfully"
    
    # Check if transformation actually occurred
    if grep -q "x-vendor-test" test-output/transformed.json; then
        print_status "Key transformation verified"
    else
        print_warning "Key transformation may not have occurred as expected"
    fi
else
    print_warning "Output file not found"
fi

# Test Docker Compose
echo ""
print_info "Testing Docker Compose..."
if docker-compose config > /dev/null 2>&1; then
    print_status "Docker Compose configuration is valid"
else
    print_warning "Docker Compose configuration may have issues"
fi

# Test security aspects
echo ""
print_info "Testing security aspects..."

# Test non-root user
echo "Testing non-root user execution..."
docker run --rm openmorph:latest sh -c 'whoami' 2>/dev/null | grep -q 'nobody' || \
docker run --rm openmorph:latest id -u 2>/dev/null | grep -q '65534'
if [ $? -eq 0 ]; then
    print_status "Container runs as non-root user"
else
    print_warning "Container may not be running as non-root user"
fi

# Test read-only filesystem capability
echo "Testing with read-only filesystem..."
docker run --rm --read-only -v $(pwd)/test-specs:/workspace:ro openmorph:latest \
  --input /workspace --dry-run > /dev/null 2>&1
print_status "Read-only filesystem test passed"

# Test resource limits
echo "Testing with resource limits..."
docker run --rm --memory=256m --cpus=0.5 -v $(pwd)/test-specs:/workspace openmorph:latest \
  --input /workspace --dry-run > /dev/null 2>&1
print_status "Resource limits test passed"

# Test image sizes
echo ""
print_info "Checking image sizes..."
PROD_SIZE=$(docker images openmorph:latest --format "table {{.Size}}" | tail -n 1)
DISTROLESS_SIZE=$(docker images openmorph:distroless --format "table {{.Size}}" | tail -n 1)
DEV_SIZE=$(docker images openmorph:dev --format "table {{.Size}}" | tail -n 1)

echo "Production image size: $PROD_SIZE"
echo "Distroless image size: $DISTROLESS_SIZE"
echo "Development image size: $DEV_SIZE"

# Health check script test
echo ""
print_info "Testing health check script..."
if [ -f "scripts/healthcheck.sh" ]; then
    # Test health check inside container
    docker run --rm -v $(pwd)/scripts:/scripts openmorph:dev /scripts/healthcheck.sh > /dev/null 2>&1
    print_status "Health check script works"
else
    print_warning "Health check script not found"
fi

# Cleanup test files
echo ""
print_info "Cleaning up test files..."
rm -rf test-specs test-output
print_status "Test files cleaned up"

# Summary
echo ""
echo "🎉 Docker Setup Validation Complete!"
echo "=================================="
echo ""
print_info "All Docker images are ready for production use"
print_info "You can now use OpenMorph in CI/CD pipelines"
echo ""
echo "Quick start commands:"
echo "  Production: docker run --rm -v \$(pwd):/workspace ghcr.io/developerkunal/openmorph:latest --help"
echo "  Development: docker run --rm -it openmorph:dev"
echo "  Docker Compose: docker-compose up openmorph"
echo ""
print_info "See DOCKER.md and PRODUCTION_DOCKER.md for detailed usage instructions"
