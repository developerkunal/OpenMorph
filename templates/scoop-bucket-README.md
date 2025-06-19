# Scoop Bucket for OpenMorph

This is the official Scoop bucket for OpenMorph, a powerful CLI tool for transforming OpenAPI vendor extension keys in YAML/JSON files.

## Features

- 🔄 **Transform vendor extensions**: Convert OpenAPI vendor extension keys across YAML/JSON files
- ⚙️ **Default values injection**: Automatically set default values for parameters, schemas, and responses with rule-based matching
- 🏷️ **Vendor-specific pagination**: Auto-inject Fern, Speakeasy, and other vendor pagination metadata
- 🎯 **Interactive mode**: TUI for previewing and approving changes before applying
- 💾 **Safe operations**: Dry-run mode and automatic backup creation
- 📋 **Flexible mapping**: Support for config files and inline key mappings
- 🚀 **Pagination optimization**: Remove lower-priority pagination strategies
- 🔧 **Response flattening**: Flatten oneOf/anyOf/allOf with single $ref structures
- ✅ **Validation**: Integrated OpenAPI validation support

## Quick Start

```powershell
# Install OpenMorph
scoop bucket add openmorph https://github.com/developerkunal/scoop-openmorph
scoop install openmorph

# Transform vendor extensions in current directory
openmorph --input . --map "x-vendor=x-custom" --interactive
```

## Usage

```powershell
# Show help
openmorph --help

# Show version
openmorph --version

# Transform OpenAPI files with inline mapping
openmorph --input C:\path\to\openapi\files --map "x-old-key=x-new-key"

# Transform with config file
openmorph --input C:\path\to\specs --config mapping-config.yaml

# Interactive mode with preview
openmorph --input C:\path\to\specs --interactive --backup

# Dry run to preview changes
openmorph --input C:\path\to\specs --dry-run --map "x-vendor=x-custom"

# Transform with pagination priority
openmorph --input C:\path\to\specs --pagination-priority "cursor,offset,page"

# Set default values with config file
openmorph --input C:\path\to\specs --config config.yaml

# Complete transformation with all features
openmorph --input C:\path\to\specs `
  --mapping x-operation-group=x-fern-sdk-group `
  --vendor-providers fern `
  --pagination-priority cursor,offset,none `
  --flatten-responses `
  --backup `
  --config config.yaml
```

## Available Packages

- **openmorph**: A powerful CLI tool for transforming OpenAPI vendor extension keys with support for:
  - Interactive TUI for reviewing changes
  - Dry-run mode for safe previews
  - Backup file creation
  - Default values injection with rule-based matching
  - Vendor-specific pagination extensions (Fern, Speakeasy)
  - Pagination strategy optimization
  - Response flattening for complex schemas
  - Config file and inline mapping support

## Requirements

- Windows 10 or later
- PowerShell 5.0 or later
- [Scoop](https://scoop.sh/) package manager

## Issues

If you have issues with the Scoop manifest, please report them at:
https://github.com/developerkunal/OpenMorph/issues

## Contributing

This bucket is automatically maintained by GoReleaser. The manifests are updated automatically when new releases are published.

For manual updates or issues with the bucket itself, please open an issue in the main repository.
