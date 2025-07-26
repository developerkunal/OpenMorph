#!/bin/bash
# Health check script for OpenMorph Docker container

set -e

# Check if openmorph binary exists and is executable
if [ ! -x "/usr/local/bin/openmorph" ]; then
    echo "ERROR: openmorph binary not found or not executable"
    exit 1
fi

# Check if openmorph can show version
if ! /usr/local/bin/openmorph --version >/dev/null 2>&1; then
    echo "ERROR: openmorph cannot show version"
    exit 1
fi

# Check if openmorph can show help
if ! /usr/local/bin/openmorph --help >/dev/null 2>&1; then
    echo "ERROR: openmorph cannot show help"
    exit 1
fi

# Create a test file and verify openmorph can process it
TEST_DIR="/tmp/healthcheck"
mkdir -p "$TEST_DIR"

# Create a simple test JSON file
cat > "$TEST_DIR/test.json" << 'EOF'
{
  "openapi": "3.0.0",
  "info": {
    "title": "Test API",
    "version": "1.0.0"
  },
  "x-test-extension": "value"
}
EOF

# Test dry-run functionality
if ! /usr/local/bin/openmorph --input "$TEST_DIR" --dry-run >/dev/null 2>&1; then
    echo "ERROR: openmorph cannot process test file"
    rm -rf "$TEST_DIR"
    exit 1
fi

# Cleanup
rm -rf "$TEST_DIR"

echo "OK: OpenMorph container is healthy"
exit 0
