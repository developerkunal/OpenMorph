# OpenMorph Auto-Release Setup Guide

This guide provides step-by-step instructions to set up automatic releases for OpenMorph with Homebrew and Scoop package managers.

## 🎯 Overview

Your OpenMorph project now includes:
- **Version Management**: Using `.version` file with automated scripts
- **GoReleaser Integration**: For building cross-platform binaries
- **Package Manager Support**: Homebrew (macOS/Linux) and Scoop (Windows)
- **GitHub Actions**: Automated release workflow
- **Validation Tools**: Scripts to verify setup integrity

## 📋 Prerequisites

Before proceeding, ensure you have:
- [ ] Go 1.24+ installed
- [ ] Git configured with your GitHub credentials
- [ ] GitHub CLI (`gh`) installed and authenticated
- [ ] Make utility available
- [ ] Write access to your GitHub repository

## 🚀 Step-by-Step Setup

### Step 1: Validate Your Setup

First, run the validation script to ensure everything is properly configured:

```bash
# Using make (recommended)
make validate

# Or directly
./scripts/validate-setup.sh
```

This will check:
- Prerequisites (Go, Git, Make)
- Project files and configuration
- Version management system
- Build process
- GoReleaser configuration
- GitHub Actions setup

### Step 2: Create Package Manager Repositories

Run the setup script to create the required repositories:

```bash
# Using make
make setup-packages

# Or directly
./scripts/setup-package-managers.sh
```

This will:
- Create `homebrew-openmorph` repository
- Create `scoop-openmorph` repository  
- Set up proper directory structure and README files
- Configure repository permissions

**Note**: You need GitHub CLI authentication for this step.

### Step 3: Test the Release Process

#### Option A: Test Release (Recommended for first time)

```bash
# Bump patch version and create release
make version-release

# This will:
# 1. Bump version from 0.1.0 to 0.1.1
# 2. Commit the version change
# 3. Push to GitHub
# 4. Create and push tag v0.1.1
# 5. Trigger GitHub Actions workflow
```

#### Option B: Manual Version Control

```bash
# Set specific version
make version-set
# Enter: 1.0.0

# Create release
make version-tag
```

### Step 4: Monitor Release Process

After pushing a tag, monitor the process:

1. **GitHub Actions**: Check workflow at `https://github.com/developerkunal/OpenMorph/actions`
2. **Release Creation**: Verify release created at `https://github.com/developerkunal/OpenMorph/releases`
3. **Package Updates**: Check that package manager repositories were updated:
   - Homebrew: `https://github.com/developerkunal/homebrew-openmorph`
   - Scoop: `https://github.com/developerkunal/scoop-openmorph`

### Step 5: Test Installation

Once the release is complete, test installation:

#### Homebrew (macOS/Linux)
```bash
# Add tap
brew tap developerkunal/openmorph

# Install
brew install openmorph

# Test
openmorph --version
```

#### Scoop (Windows)
```powershell
# Add bucket
scoop bucket add openmorph https://github.com/developerkunal/scoop-openmorph

# Install
scoop install openmorph

# Test
openmorph --version
```

## 🔧 Version Management Commands

### Quick Reference

```bash
# Show current version
make version-show

# Bump versions
make version-bump-patch    # 0.1.0 → 0.1.1
make version-bump-minor    # 0.1.0 → 0.2.0  
make version-bump-major    # 0.1.0 → 1.0.0

# Set specific version
make version-set

# Create and push tag
make version-tag

# Full release (bump patch + commit + tag + push)
make version-release

# Validate setup
make validate

# Setup package managers
make setup-packages
```

### Version Script Commands

```bash
# Direct script usage
./scripts/version.sh show
./scripts/version.sh bump patch
./scripts/version.sh set 2.1.0
./scripts/version.sh tag
./scripts/version.sh release
```

## 🎛️ Configuration Files

### Key Configuration Files

- **`.version`**: Contains current semantic version (e.g., `0.1.0`)
- **`.goreleaser.yml`**: GoReleaser configuration for builds and package managers
- **`.github/workflows/release.yml`**: GitHub Actions workflow for releases
- **`Makefile`**: Build and version management commands

### Repository Names

- **Homebrew**: `developerkunal/homebrew-openmorph`
- **Scoop**: `developerkunal/scoop-openmorph`

### Installation Commands for Users

```bash
# Homebrew (macOS/Linux)
brew tap developerkunal/openmorph
brew install openmorph

# Scoop (Windows)  
scoop bucket add openmorph https://github.com/developerkunal/scoop-openmorph
scoop install openmorph
```

## 🔍 Development Workflow

### Regular Development

```bash
# Make code changes
git add .
git commit -m "Add new feature"
git push

# Version is automatically picked up from .version file
make build
./openmorph --version  # Shows current version
```

### Creating Releases

```bash
# Option 1: Automatic patch release
make version-release

# Option 2: Custom version
make version-set      # Enter new version
make version-tag      # Create and push tag

# Option 3: Manual bump
make version-bump-minor  # or major/patch
git add .version
git commit -m "Bump version to $(cat .version)"
git push
make version-tag
```

### Testing Without Releasing

```bash
# Test build without releasing
make snapshot

# This creates local builds in dist/ without publishing
```

## 🛠️ Troubleshooting

### Common Issues

#### 1. GoReleaser Errors
```bash
# Check configuration
goreleaser check

# Test local build
make snapshot
```

#### 2. Version Script Issues
```bash
# Ensure script is executable
chmod +x scripts/version.sh

# Check version file format
cat .version
# Should contain only: X.Y.Z (e.g., 1.2.3)
```

#### 3. GitHub Actions Failures
- Check `GITHUB_TOKEN` permissions in repository settings
- Verify repository names in `.goreleaser.yml` exist
- Ensure package manager repositories were created

#### 4. Package Manager Installation Issues
- Verify repositories exist and are public
- Check that releases completed successfully
- Ensure package manager files were generated in release

### Validation

Always run validation before releases:
```bash
make validate
```

## 📚 Additional Resources

- **Detailed Version Management**: See `VERSION_MANAGEMENT.md`
- **GoReleaser Documentation**: https://goreleaser.com/
- **GitHub Actions Documentation**: https://docs.github.com/en/actions

## ✅ Success Checklist

After setup, you should be able to:

- [ ] Run `make validate` successfully
- [ ] Build project with `make build`
- [ ] Show version with `./openmorph --version`
- [ ] Create snapshot with `make snapshot`
- [ ] Bump version with `make version-bump-patch`
- [ ] Create release with `make version-release`
- [ ] Install via Homebrew: `brew install openmorph`
- [ ] Install via Scoop: `scoop install openmorph`

🎉 **Congratulations!** Your OpenMorph project now has a complete auto-release setup with package manager support!
