#!/bin/bash

# Validation script for OpenMorph release setup
# This script validates that all components are properly configured

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🔍 OpenMorph Release Setup Validation${NC}"
echo "======================================"
echo ""

# Function to check if command exists
check_command() {
    if command -v "$1" &> /dev/null; then
        echo -e "  ✅ $1 is available"
        return 0
    else
        echo -e "  ❌ $1 is not available"
        return 1
    fi
}

# Function to check file exists
check_file() {
    if [ -f "$1" ]; then
        echo -e "  ✅ $1 exists"
        return 0
    else
        echo -e "  ❌ $1 does not exist"
        return 1
    fi
}

# Function to validate version file
validate_version_file() {
    if [ -f "$PROJECT_DIR/.version" ]; then
        VERSION=$(cat "$PROJECT_DIR/.version" | tr -d '\n')
        if [[ $VERSION =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            echo -e "  ✅ .version file contains valid semver: $VERSION"
            return 0
        else
            echo -e "  ❌ .version file contains invalid version: $VERSION"
            return 1
        fi
    else
        echo -e "  ❌ .version file does not exist"
        return 1
    fi
}

echo -e "${YELLOW}1. Checking Prerequisites${NC}"
PREREQ_OK=true
check_command "go" || PREREQ_OK=false
check_command "git" || PREREQ_OK=false
check_command "make" || PREREQ_OK=false

echo ""
echo -e "${YELLOW}2. Checking Project Files${NC}"
FILES_OK=true
cd "$PROJECT_DIR"
check_file ".goreleaser.yml" || FILES_OK=false
check_file "Makefile" || FILES_OK=false
check_file "scripts/version.sh" || FILES_OK=false
check_file "scripts/setup-package-managers.sh" || FILES_OK=false
check_file ".github/workflows/release.yml" || FILES_OK=false
validate_version_file || FILES_OK=false

echo ""
echo -e "${YELLOW}3. Testing Version Management${NC}"
VERSION_OK=true
if [ -x "scripts/version.sh" ]; then
    CURRENT_VERSION=$(./scripts/version.sh show)
    echo -e "  ✅ Version script works: $CURRENT_VERSION"
else
    echo -e "  ❌ Version script is not executable"
    VERSION_OK=false
fi

echo ""
echo -e "${YELLOW}4. Testing Build Process${NC}"
BUILD_OK=true
if make build &> /dev/null; then
    if [ -f "openmorph" ]; then
        VERSION_OUTPUT=$(./openmorph --version 2>&1)
        echo -e "  ✅ Build successful: $VERSION_OUTPUT"
        rm -f openmorph  # Clean up
    else
        echo -e "  ❌ Build did not produce binary"
        BUILD_OK=false
    fi
else
    echo -e "  ❌ Build failed"
    BUILD_OK=false
fi

echo ""
echo -e "${YELLOW}5. Testing GoReleaser Configuration${NC}"
GORELEASER_OK=true
if command -v goreleaser &> /dev/null; then
    if goreleaser check &> /dev/null; then
        echo -e "  ✅ GoReleaser configuration is valid"
    else
        echo -e "  ❌ GoReleaser configuration has errors"
        GORELEASER_OK=false
    fi
else
    echo -e "  ⚠️  GoReleaser not installed (will be installed by GitHub Actions)"
    GORELEASER_OK=true  # This is OK for local development
fi

echo ""
echo -e "${YELLOW}6. Checking GitHub Configuration${NC}"
GITHUB_OK=true
if [ -f ".github/workflows/release.yml" ]; then
    if grep -q "GITHUB_TOKEN" ".github/workflows/release.yml"; then
        echo -e "  ✅ GitHub Actions workflow configured"
    else
        echo -e "  ❌ GitHub Actions workflow missing GITHUB_TOKEN"
        GITHUB_OK=false
    fi
else
    echo -e "  ❌ GitHub Actions workflow not found"
    GITHUB_OK=false
fi

echo ""
echo -e "${YELLOW}7. Package Manager Repository Names${NC}"
REPOS_OK=true
if grep -q "homebrew-openmorph" ".goreleaser.yml"; then
    echo -e "  ✅ Homebrew repository name: homebrew-openmorph"
else
    echo -e "  ❌ Homebrew repository name not configured correctly"
    REPOS_OK=false
fi

if grep -q "scoop-openmorph" ".goreleaser.yml"; then
    echo -e "  ✅ Scoop repository name: scoop-openmorph"
else
    echo -e "  ❌ Scoop repository name not configured correctly"
    REPOS_OK=false
fi

echo ""
echo "======================================"
echo -e "${BLUE}Validation Summary${NC}"
echo ""

ALL_OK=true
if $PREREQ_OK; then
    echo -e "  ✅ Prerequisites"
else
    echo -e "  ❌ Prerequisites"
    ALL_OK=false
fi

if $FILES_OK; then
    echo -e "  ✅ Project Files"
else
    echo -e "  ❌ Project Files"
    ALL_OK=false
fi

if $VERSION_OK; then
    echo -e "  ✅ Version Management"
else
    echo -e "  ❌ Version Management"
    ALL_OK=false
fi

if $BUILD_OK; then
    echo -e "  ✅ Build Process"
else
    echo -e "  ❌ Build Process"
    ALL_OK=false
fi

if $GORELEASER_OK; then
    echo -e "  ✅ GoReleaser Configuration"
else
    echo -e "  ❌ GoReleaser Configuration"
    ALL_OK=false
fi

if $GITHUB_OK; then
    echo -e "  ✅ GitHub Configuration"
else
    echo -e "  ❌ GitHub Configuration"
    ALL_OK=false
fi

if $REPOS_OK; then
    echo -e "  ✅ Repository Names"
else
    echo -e "  ❌ Repository Names"
    ALL_OK=false
fi

echo ""
if $ALL_OK; then
    echo -e "${GREEN}🎉 All validations passed! Your release setup is ready.${NC}"
    echo ""
    echo -e "${BLUE}Next steps:${NC}"
    echo "1. Run: ./scripts/setup-package-managers.sh"
    echo "2. Test release: ./scripts/version.sh release"
    echo ""
    exit 0
else
    echo -e "${RED}❌ Some validations failed. Please fix the issues above.${NC}"
    echo ""
    exit 1
fi
