#!/bin/bash

# Setup script for OpenMorph package manager repositories
# This script helps create and configure the required repositories for Homebrew and Scoop

set -e

GITHUB_USERNAME="developerkunal"
HOMEBREW_REPO="homebrew-openmorph"
SCOOP_REPO="scoop-openmorph"

echo "🚀 OpenMorph Package Manager Setup"
echo "=================================="
echo ""

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo "❌ GitHub CLI (gh) is not installed."
    echo "Please install it first: https://cli.github.com/"
    exit 1
fi

# Check if user is authenticated
if ! gh auth status &> /dev/null; then
    echo "❌ Not authenticated with GitHub CLI."
    echo "Please run: gh auth login"
    exit 1
fi

echo "✅ GitHub CLI is installed and authenticated"
echo ""

# Function to create repository
create_repository() {
    local repo_name=$1
    local description=$2
    
    echo "📦 Creating repository: $repo_name"
    
    if gh repo view "$GITHUB_USERNAME/$repo_name" &> /dev/null; then
        echo "ℹ️  Repository $repo_name already exists"
        return 0
    fi
    
    gh repo create "$repo_name" \
        --description "$description" \
        --public \
        --add-readme
    
    echo "✅ Created repository: $repo_name"
}

# Function to setup repository files
setup_homebrew_repo() {
    echo "🍺 Setting up Homebrew tap repository..."
    
    # Clone the repository
    if [ ! -d "$HOMEBREW_REPO" ]; then
        gh repo clone "$GITHUB_USERNAME/$HOMEBREW_REPO"
    fi
    
    cd "$HOMEBREW_REPO"
    
    # Create Formula directory
    mkdir -p Formula
    
    # Copy README
    cp "../templates/homebrew-tap-README.md" "README.md"
    cp "../templates/Formula/.gitkeep" "Formula/.gitkeep"
    
    # Commit changes
    git add .
    git diff --staged --quiet || git commit -m "Initial setup for Homebrew tap"
    git push
    
    cd ..
    echo "✅ Homebrew tap repository setup complete"
}

setup_scoop_repo() {
    echo "🪣 Setting up Scoop bucket repository..."
    
    # Clone the repository
    if [ ! -d "$SCOOP_REPO" ]; then
        gh repo clone "$GITHUB_USERNAME/$SCOOP_REPO"
    fi
    
    cd "$SCOOP_REPO"
    
    # Create bucket directory
    mkdir -p bucket
    
    # Copy README
    cp "../templates/scoop-bucket-README.md" "README.md"
    cp "../templates/bucket/.gitkeep" "bucket/.gitkeep"
    
    # Commit changes
    git add .
    git diff --staged --quiet || git commit -m "Initial setup for Scoop bucket"
    git push
    
    cd ..
    echo "✅ Scoop bucket repository setup complete"
}

# Main execution
echo "Creating repositories..."
create_repository "$HOMEBREW_REPO" "Homebrew tap for OpenMorph"
create_repository "$SCOOP_REPO" "Scoop bucket for OpenMorph"

echo ""
echo "Setting up repository files..."
setup_homebrew_repo
setup_scoop_repo

# Clean up
rm -rf "$HOMEBREW_REPO" "$SCOOP_REPO"

echo ""
echo "🎉 Setup Complete!"
echo "=================="
echo ""
echo "Next steps:"
echo "1. Your repositories are now created and configured"
echo "2. Create a new release tag to test the process:"
echo "   git tag v1.0.0"
echo "   git push origin v1.0.0"
echo ""
echo "3. Users can now install OpenMorph using:"
echo ""
echo "   Homebrew (macOS/Linux):"
echo "   brew tap $GITHUB_USERNAME/openmorph"
echo "   brew install openmorph"
echo ""
echo "   Scoop (Windows):"
echo "   scoop bucket add openmorph https://github.com/$GITHUB_USERNAME/$SCOOP_REPO"
echo "   scoop install openmorph"
echo ""
echo "4. Check the GitHub Actions workflow after pushing a tag"
echo "5. Update your main README with installation instructions"
