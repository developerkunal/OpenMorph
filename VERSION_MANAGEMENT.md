# Version Management Guide

This guide explains how to manage versions and releases for OpenMorph using the integrated version management system.

## Overview

OpenMorph uses a `.version` file to track the current version, along with automated scripts for version bumping and release management.

## Version File

The `.version` file contains the current semantic version (e.g., `1.2.3`) without the `v` prefix.

## Version Management Commands

### Using the Version Script

```bash
# Show current version
./scripts/version.sh show

# Bump patch version (1.0.0 -> 1.0.1)
./scripts/version.sh bump patch

# Bump minor version (1.0.0 -> 1.1.0)
./scripts/version.sh bump minor

# Bump major version (1.0.0 -> 2.0.0)
./scripts/version.sh bump major

# Set specific version
./scripts/version.sh set 2.1.0

# Create and push git tag for current version
./scripts/version.sh tag

# Full release: bump patch, commit, tag, and push
./scripts/version.sh release
```

### Using Make Commands

```bash
# Show current version
make version-show

# Bump versions
make version-bump-patch
make version-bump-minor
make version-bump-major

# Set specific version (interactive)
make version-set

# Create tag
make version-tag

# Full release
make version-release
```

## Build Integration

The version is automatically integrated into builds:

### Local Development
- When building locally, the version from `.version` file is used
- Build command: `make build` or `go build -ldflags "-X github.com/developerkunal/OpenMorph/cmd.version=v$(VERSION)"`

### GoReleaser (CI/CD)
- When releasing via tags, GoReleaser sets the version from the git tag
- The ldflags inject the version into the binary at build time

### Version Resolution Order
1. **Build-time version**: If set via ldflags (e.g., by GoReleaser), this takes precedence
2. **File version**: If no build-time version, reads from `.version` file
3. **Development fallback**: If neither available, defaults to "dev"

## Release Workflow

### Option 1: Manual Release
```bash
# 1. Bump version
./scripts/version.sh bump patch

# 2. Commit changes
git add .version
git commit -m "Bump version to $(cat .version)"

# 3. Push changes
git push

# 4. Create and push tag
./scripts/version.sh tag
```

### Option 2: Automated Release
```bash
# This does everything in one command:
# - Bumps patch version
# - Commits the change
# - Pushes to remote
# - Creates and pushes tag
./scripts/version.sh release

# Or using make:
make version-release
```

### Option 3: Manual Version with Automated Release
```bash
# Set specific version
./scripts/version.sh set 2.0.0

# Then do automated release
./scripts/version.sh release
```

## Package Manager Integration

When a new tag is pushed, GitHub Actions automatically:

1. **Builds binaries** for multiple platforms using GoReleaser
2. **Updates Homebrew tap** at `developerkunal/homebrew-openmorph`
3. **Updates Scoop bucket** at `developerkunal/scoop-openmorph`
4. **Creates GitHub release** with release notes

## Installation Commands

After a successful release, users can install using:

### Homebrew (macOS/Linux)
```bash
brew tap developerkunal/openmorph
brew install openmorph
```

### Scoop (Windows)
```bash
scoop bucket add openmorph https://github.com/developerkunal/scoop-openmorph
scoop install openmorph
```

## Checking Version

Users can check the installed version:
```bash
openmorph --version
# Output: openmorph version v1.2.3
```

## Development Notes

- The version script includes safety checks (clean working directory, branch validation)
- All version operations are logged with colored output for clarity
- The `.version` file should always contain a valid semantic version
- Tags are always prefixed with `v` (e.g., `v1.2.3`) even though the file contains just `1.2.3`

## Troubleshooting

### Version not updating
- Ensure `.version` file exists and contains valid semver
- Check that ldflags in Makefile/GoReleaser are correct
- Verify the import path in ldflags matches your module

### Release failed
- Check GitHub Actions logs
- Ensure GITHUB_TOKEN has proper permissions
- Verify repository names in `.goreleaser.yml` exist

### Package managers not updating
- Check that the tap/bucket repositories exist
- Verify GitHub Actions completed successfully
- Ensure the repositories have proper write permissions
