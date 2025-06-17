# OpenMorph

<p align="center">
  <img src="https://raw.githubusercontent.com/developerkunal/OpenMorph/main/.github/logo.png" alt="OpenMorph Logo" width="180"/>
</p>

[![Go Reference](https://pkg.go.dev/badge/github.com/developerkunal/OpenMorph.svg)](https://pkg.go.dev/github.com/developerkunal/OpenMorph)
[![CI](https://github.com/developerkunal/OpenMorph/actions/workflows/ci.yml/badge.svg)](https://github.com/developerkunal/OpenMorph/actions/workflows/ci.yml)
[![MIT License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

---

OpenMorph is a production-grade CLI and TUI tool for transforming OpenAPI vendor extension keys across YAML/JSON files. It supports interactive review, dry-run previews, backups, robust mapping/exclusion logic, and is designed for maintainability and extensibility.

## Features

- Transform OpenAPI vendor extension keys in YAML/JSON
- Interactive TUI for reviewing and approving changes
- Colorized before/after diffs (CLI and TUI)
- Dry-run mode for safe previews
- Backup support
- Config file and CLI flag merging
- Exclude keys from transformation
- OpenAPI validation integration
- **Pagination priority support** - Remove lower-priority pagination strategies
- Modern, maintainable Go codebase

## Credits / Acknowledgements

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) for TUI
- [spf13/cobra](https://github.com/spf13/cobra) for CLI
- [gopkg.in/yaml.v3](https://github.com/go-yaml/yaml) for YAML parsing
- [Contributor Covenant](https://www.contributor-covenant.org/) for Code of Conduct

## Installation

### Package Managers (Recommended)

#### Homebrew (macOS/Linux)

```bash
# Add the tap
brew tap developerkunal/openmorph

# Install OpenMorph
brew install openmorph
```

#### Scoop (Windows)

```powershell
# Add the bucket
scoop bucket add openmorph https://github.com/developerkunal/scoop-openmorph

# Install OpenMorph
scoop install openmorph
```

### From Source

#### Prerequisites

- Go 1.24 or later

#### Build from source

```bash
# Clone the repository
git clone https://github.com/developerkunal/OpenMorph.git
cd OpenMorph

# Build the binary
make build
# or
go build -o openmorph main.go
```

#### Install from source

```bash
# Build and install to GOPATH/bin
make install
```

## Usage

```sh
openmorph [flags]
```

### Flags and Options

| Flag                    | Description                                                                            |
| ----------------------- | -------------------------------------------------------------------------------------- |
| `--input`               | Path to the input directory or file (YAML/JSON). Required.                             |
| `--mapping`             | Key mapping(s) in the form `old=new`. Can be specified multiple times.                 |
| `--exclude`             | Key(s) to exclude from transformation. Can be specified multiple times.                |
| `--dry-run`             | Show a preview of changes (with colorized before/after diffs) without modifying files. |
| `--backup`              | Create `.bak` backup files before modifying originals.                                 |
| `--interactive`         | Launch an interactive TUI for reviewing and approving changes before applying them.    |
| `--config`              | Path to a YAML/JSON config file with mappings/excludes.                                |
| `--no-config`           | Ignore all config files and use only CLI flags.                                        |
| `--validate`            | Run OpenAPI validation (requires `swagger-cli` in PATH).                               |
| `--pagination-priority` | Pagination strategy priority order (e.g., checkpoint,offset,page,cursor,none).         |
| `--version`             | Show version and exit.                                                                 |
| `-h`, `--help`          | Show help message.                                                                     |

### Example: Basic CLI Usage

Transform all `x-foo` keys to `x-bar` in a directory:

```sh
openmorph --input ./openapi --mapping x-foo=x-bar
```

### Example: Exclude Keys

```sh
openmorph --input ./openapi --mapping x-foo=x-bar --exclude x-ignore
```

### Example: Dry Run (Preview Only)

```sh
openmorph --input ./openapi --mapping x-foo=x-bar --dry-run
```

**Note:** In dry-run mode, transformations (pagination and response flattening) are previewed independently based on the original file. In actual execution, they are applied sequentially, so later steps may show different results. Use `--interactive` mode to see the exact cumulative effects of all transformations.

### Example: Interactive Review (TUI)

```sh
openmorph --input ./openapi --mapping x-foo=x-bar --interactive
```

### Example: Using a Config File

```sh
openmorph --input ./openapi --config ./morph.yaml
```

#### Example morph.yaml

```yaml
mappings:
  x-foo: x-bar
  x-baz: x-qux
exclude:
  - x-ignore
pagination_priority:
  - checkpoint
  - offset
  - page
  - cursor
  - none
```

### Example: With Backup

```sh
openmorph --input ./openapi --mapping x-foo=x-bar --backup
```

### Example: Validate After Transform

```sh
openmorph --input ./openapi --mapping x-foo=x-bar --validate
```

### Example: Pagination Priority

Transform APIs to use only checkpoint pagination (highest priority):

```sh
openmorph --input ./openapi --pagination-priority checkpoint,offset,none
```

Remove lower-priority pagination strategies:

```sh
openmorph --input ./openapi --pagination-priority cursor,page,offset,none --dry-run
```

## Pagination Priority

The pagination priority feature allows you to enforce a single pagination strategy across your OpenAPI specifications by removing lower-priority pagination parameters and responses.

### How It Works

When pagination priority is configured, OpenMorph:

1. **Detects** all pagination strategies in each endpoint (parameters and responses)
2. **Selects** the highest priority strategy from those available
3. **Removes** parameters and response schemas belonging to lower-priority strategies
4. **Preserves** OpenAPI structure integrity (handles `oneOf`, `anyOf`, `allOf`)
5. **Cleans up** unused component schemas

### Supported Pagination Strategies

| Strategy   | Parameters                           | Response Fields                          |
| ---------- | ------------------------------------ | ---------------------------------------- |
| checkpoint | `from`, `take`, `after`              | `next`, `next_checkpoint`                |
| offset     | `offset`, `limit`, `include_totals`  | `total`, `offset`, `limit`, `count`      |
| page       | `page`, `per_page`, `include_totals` | `start`, `limit`, `total`, `total_count` |
| cursor     | `cursor`, `size`                     | `next_cursor`, `has_more`                |
| none       | (no parameters)                      | (no fields)                              |

### Example Transformations

**Before** (multiple pagination strategies):

```yaml
"/users":
  get:
    parameters:
      - name: offset
        in: query
      - name: from
        in: query
    responses:
      "200":
        content:
          application/json:
            schema:
              oneOf:
                - properties:
                    total: { type: integer } # offset
                    users: { type: array }
                - properties:
                    next: { type: string } # checkpoint
                    users: { type: array }
```

**After** (with priority `checkpoint,offset`):

```yaml
"/users":
  get:
    parameters:
      - name: from
        in: query
    responses:
      "200":
        content:
          application/json:
            schema:
              properties:
                next: { type: string }
                users: { type: array }
```

## Interactive TUI Controls

- `j`/`k` or `left`/`right`: Navigate files
- `a` or `enter`: Accept file changes
- `s`: Skip file
- `A`: Accept all
- `S`: Skip all
- `?`: Toggle help
- `q` or `ctrl+c`: Quit

## Output

- **Dry Run:** Shows colorized before/after diffs for each key change, grouped by file.
- **TUI:** Shows all key changes with navigation, full block diffs, and summary.
- **CLI:** Prints a summary of accepted/skipped/transformed files.

## Notes

- Both YAML and JSON are supported.
- All occurrences of a key are transformed, including in arrays/objects.
- Backups are only created if `--backup` is specified.
- Config file values are merged with CLI flags (CLI flags take precedence).

## Security & Privacy

- No secrets, credentials, or sensitive info are stored or required.
- Please report any security issues via GitHub issues.

## Development

### Release Management

This project uses automated release management with package managers support. See the [Auto-Release Guide](AUTO_RELEASE_GUIDE.md) for complete setup instructions.

Quick commands:

```bash
# Validate setup
make validate

# Create release
make version-release

# Setup package managers
make setup-packages
```

## License

MIT
