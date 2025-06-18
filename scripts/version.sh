#!/bin/bash

# Version management script for OpenMorph
# This script helps manage versioning using the .version file

set -e

VERSION_FILE=".version"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
VERSION_FILE_PATH="$PROJECT_DIR/$VERSION_FILE"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to display usage
usage() {
    echo "Usage: $0 [COMMAND] [OPTIONS]"
    echo ""
    echo "Commands:"
    echo "  show               Show current version"
    echo "  bump LEVEL         Bump version (major|minor|patch)"
    echo "  set VERSION        Set specific version"
    echo "  tag                Create and push git tag with current version"
    echo "  release [LEVEL]    Create release (defaults to patch)"
    echo "  major-release      Bump major version and create release"
    echo "  minor-release      Bump minor version and create release"
    echo "  patch-release      Bump patch version and create release (same as release)"
    echo "  preview LEVEL      Show preview of release actions (dry run)"
    echo ""
    echo "Examples:"
    echo "  $0 show"
    echo "  $0 bump patch"
    echo "  $0 bump minor" 
    echo "  $0 bump major"
    echo "  $0 set 2.1.0"
    echo "  $0 tag"
    echo "  $0 release"
    echo "  $0 release minor"
    echo "  $0 major-release"
    echo "  $0 minor-release"
    echo "  $0 patch-release"
    echo "  $0 preview major"
}

# Function to get current version
get_version() {
    if [ ! -f "$VERSION_FILE_PATH" ]; then
        echo "0.0.0"
        return
    fi
    cat "$VERSION_FILE_PATH" | tr -d '\n'
}

# Function to set version
set_version() {
    local new_version=$1
    if [[ ! $new_version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        echo -e "${RED}Error: Version must be in format X.Y.Z${NC}" >&2
        exit 1
    fi
    echo "$new_version" > "$VERSION_FILE_PATH"
    echo -e "${GREEN}Version set to: $new_version${NC}"
}

# Function to bump version
bump_version() {
    local level=$1
    local current_version=$(get_version)
    
    IFS='.' read -r major minor patch <<< "$current_version"
    
    case $level in
        major)
            major=$((major + 1))
            minor=0
            patch=0
            ;;
        minor)
            minor=$((minor + 1))
            patch=0
            ;;
        patch)
            patch=$((patch + 1))
            ;;
        *)
            echo -e "${RED}Error: Level must be major, minor, or patch${NC}" >&2
            exit 1
            ;;
    esac
    
    local new_version="$major.$minor.$patch"
    set_version "$new_version"
}

# Function to create and push git tag
create_tag() {
    local version=$(get_version)
    local tag="v$version"
    
    echo -e "${BLUE}Creating tag: $tag${NC}"
    
    # Check if tag already exists
    if git tag -l | grep -q "^$tag$"; then
        echo -e "${YELLOW}Warning: Tag $tag already exists${NC}"
        read -p "Do you want to delete and recreate it? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            git tag -d "$tag"
            git push origin ":refs/tags/$tag" 2>/dev/null || true
        else
            echo -e "${RED}Aborted${NC}"
            exit 1
        fi
    fi
    
    # Create tag
    git tag -a "$tag" -m "Release $tag"
    git push origin "$tag"
    
    echo -e "${GREEN}Tag $tag created and pushed${NC}"
}

# Function to do a full release
do_release() {
    local level=${1:-patch}  # Default to patch if no level specified
    
    echo -e "${BLUE}Starting $level release process...${NC}"
    
    # Check if working directory is clean
    if [ -n "$(git status --porcelain)" ]; then
        echo -e "${RED}Error: Working directory is not clean. Please commit or stash changes.${NC}"
        exit 1
    fi
    
    # Check if we're on main branch
    current_branch=$(git branch --show-current)
    if [ "$current_branch" != "main" ] && [ "$current_branch" != "master" ]; then
        echo -e "${YELLOW}Warning: You're not on main/master branch (current: $current_branch)${NC}"
        read -p "Continue anyway? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            echo -e "${RED}Aborted${NC}"
            exit 1
        fi
    fi
    
    local current_version=$(get_version)
    echo -e "${BLUE}Current version: $current_version${NC}"
    
    # Validate release level
    case $level in
        major|minor|patch)
            ;;
        *)
            echo -e "${RED}Error: Release level must be major, minor, or patch${NC}" >&2
            exit 1
            ;;
    esac
    
    # Show what will happen
    IFS='.' read -r major minor patch_num <<< "$current_version"
    case $level in
        major)
            new_major=$((major + 1))
            new_minor=0
            new_patch=0
            ;;
        minor)
            new_major=$major
            new_minor=$((minor + 1))
            new_patch=0
            ;;
        patch)
            new_major=$major
            new_minor=$minor
            new_patch=$((patch_num + 1))
            ;;
    esac
    local new_version="$new_major.$new_minor.$new_patch"
    
    echo -e "${YELLOW}This will create a $level release:${NC}"
    echo -e "  Current version: ${RED}$current_version${NC}"
    echo -e "  New version:     ${GREEN}$new_version${NC}"
    echo ""
    read -p "Continue with $level release? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${RED}Aborted${NC}"
        exit 1
    fi
    
    # Bump version
    bump_version "$level"
    local final_version=$(get_version)
    
    # Commit version change
    git add "$VERSION_FILE_PATH"
    git commit -m "Bump version to $final_version ($level release)"
    git push
    
    # Create and push tag
    create_tag
    
    echo -e "${GREEN}$level release $final_version completed!${NC}"
    echo -e "${BLUE}GitHub Actions will now build and publish the release.${NC}"
}

# Function to show what a release would do (dry run)
show_release_preview() {
    local level=${1:-patch}
    
    echo -e "${BLUE}Release Preview (Dry Run)${NC}"
    echo -e "${BLUE}=========================${NC}"
    
    local current_version=$(get_version)
    echo -e "Current version: ${YELLOW}$current_version${NC}"
    
    # Calculate new version
    IFS='.' read -r major minor patch_num <<< "$current_version"
    case $level in
        major)
            new_major=$((major + 1))
            new_minor=0
            new_patch=0
            ;;
        minor)
            new_major=$major
            new_minor=$((minor + 1))
            new_patch=0
            ;;
        patch)
            new_major=$major
            new_minor=$minor
            new_patch=$((patch_num + 1))
            ;;
        *)
            echo -e "${RED}Error: Release level must be major, minor, or patch${NC}" >&2
            exit 1
            ;;
    esac
    local new_version="$new_major.$new_minor.$new_patch"
    
    echo -e "Release level: ${BLUE}$level${NC}"
    echo -e "New version: ${GREEN}$new_version${NC}"
    echo -e "Git tag: ${GREEN}v$new_version${NC}"
    echo ""
    
    # Check working directory status
    if [ -n "$(git status --porcelain)" ]; then
        echo -e "${RED}⚠️  Working directory is not clean${NC}"
    else
        echo -e "${GREEN}✅ Working directory is clean${NC}"
    fi
    
    # Check branch
    current_branch=$(git branch --show-current)
    if [ "$current_branch" = "main" ] || [ "$current_branch" = "master" ]; then
        echo -e "${GREEN}✅ On $current_branch branch${NC}"
    else
        echo -e "${YELLOW}⚠️  On $current_branch branch (not main/master)${NC}"
    fi
    
    echo ""
    echo -e "${BLUE}Actions that would be performed:${NC}"
    echo -e "  1. Bump version from $current_version to $new_version"
    echo -e "  2. Update .version file"
    echo -e "  3. Commit: 'Bump version to $new_version ($level release)'"
    echo -e "  4. Push commit to remote"
    echo -e "  5. Create git tag: v$new_version"
    echo -e "  6. Push tag to remote"
    echo -e "  7. Trigger GitHub Actions for release build"
}

# Main script logic
case "${1:-}" in
    show)
        echo "$(get_version)"
        ;;
    bump)
        if [ -z "${2:-}" ]; then
            echo -e "${RED}Error: Please specify bump level (major|minor|patch)${NC}" >&2
            usage
            exit 1
        fi
        bump_version "$2"
        ;;
    set)
        if [ -z "${2:-}" ]; then
            echo -e "${RED}Error: Please specify version${NC}" >&2
            usage
            exit 1
        fi
        set_version "$2"
        ;;
    tag)
        create_tag
        ;;
    release)
        # Handle optional level parameter (defaults to patch)
        level=${2:-patch}
        case $level in
            major|minor|patch)
                do_release "$level"
                ;;
            *)
                echo -e "${RED}Error: Release level must be major, minor, or patch${NC}" >&2
                usage
                exit 1
                ;;
        esac
        ;;
    major-release)
        do_release major
        ;;
    minor-release)
        do_release minor
        ;;
    patch-release)
        do_release patch
        ;;
    preview)
        if [ -z "${2:-}" ]; then
            echo -e "${RED}Error: Please specify release level for preview (major|minor|patch)${NC}" >&2
            usage
            exit 1
        fi
        show_release_preview "$2"
        ;;
    help|--help|-h)
        usage
        ;;
    *)
        echo -e "${RED}Error: Unknown command '${1:-}'${NC}" >&2
        echo ""
        usage
        exit 1
        ;;
esac
