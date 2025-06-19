# OpenMorph

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/developerkunal/OpenMorph/main/.github/openmorph_darkmode.png">
    <source media="(prefers-color-scheme: light)" srcset="https://raw.githubusercontent.com/developerkunal/OpenMorph/main/.github/openmorph_lightmode.png">
    <img src="https://raw.githubusercontent.com/developerkunal/OpenMorph/main/.github/openmorph_lightmode.png" alt="OpenMorph Logo" width="180"/>
  </picture>
</p>

[![Go Reference](https://pkg.go.dev/badge/github.com/developerkunal/OpenMorph.svg)](https://pkg.go.dev/github.com/developerkunal/OpenMorph)
[![CI](https://github.com/developerkunal/OpenMorph/actions/workflows/ci.yml/badge.svg)](https://github.com/developerkunal/OpenMorph/actions/workflows/ci.yml)
[![MIT License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

---

OpenMorph is a production-grade CLI and TUI tool for transforming OpenAPI vendor extension keys across YAML/JSON files. It supports interactive review, dry-run previews, backups, robust mapping/exclusion logic, vendor-specific pagination extensions, and is designed for maintainability and extensibility.

## Features

- Transform OpenAPI vendor extension keys in YAML/JSON
- **Default values injection** - Automatically set default values for parameters, schemas, and responses with rule-based matching
- **Vendor-specific pagination extensions** - Auto-inject Fern, Speakeasy, and other vendor pagination metadata
- **Auto-detection of array fields** - Automatically find results arrays in response schemas
- Interactive TUI for reviewing and approving changes
- Colorized before/after diffs (CLI and TUI)
- Dry-run mode for safe previews
- Backup support
- Config file and CLI flag merging
- Exclude keys from transformation
- OpenAPI validation integration
- **Pagination priority support** - Remove lower-priority pagination strategies
- **Consistent JSON formatting** - Maintains clean, multi-line array formatting
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
| `--vendor-providers`    | Specific vendor providers to apply (e.g., fern,speakeasy). If empty, applies all.      |
| `--flatten-responses`   | Flatten oneOf/anyOf/allOf with single $ref after pagination processing.                |
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
vendor_extensions:
  enabled: true
  providers:
    fern:
      extension_name: "x-fern-pagination"
      target_level: "operation"
      methods: ["get"]
      field_mapping:
        request_params:
          cursor: ["cursor", "after"]
          limit: ["limit", "size"]
      strategies:
        cursor:
          template:
            type: "cursor"
            cursor_param: "$request.{cursor_param}"
            page_size_param: "$request.{limit_param}"
            results_path: "$response.{results_field}"
          required_fields: ["cursor_param", "results_field"]
```

### Example: With Backup

```sh
openmorph --input ./openapi --mapping x-foo=x-bar --backup
```

### Example: Validate After Transform

```sh
openmorph --input ./openapi --mapping x-foo=x-bar --validate
```

### Example: Add Vendor Extensions

Add vendor extensions (auto-enabled when configured):

```sh
openmorph --input ./openapi --config fern-config.yaml
```

Add extensions for specific providers only:

```sh
openmorph --input ./openapi --vendor-providers fern --config config.yaml
```

### Example: Complete Transformation

Transform keys, clean up pagination, add vendor extensions, and set default values:

```sh
openmorph --input ./openapi \
  --mapping x-operation-group-name=x-fern-sdk-group-name \
  --pagination-priority cursor,offset,none \
  --vendor-providers fern \
  --flatten-responses \
  --backup \
  --config ./config.yaml
```

Your `config.yaml` can include all features:

```yaml
mappings:
  x-foo: x-bar
vendor_extensions:
  enabled: true
  providers:
    fern:
      # ... fern config
default_values:
  enabled: true
  rules:
    # ... default value rules
```

### Example: Pagination Priority

Transform APIs to use only checkpoint pagination (highest priority):

```sh
openmorph --input ./openapi --pagination-priority checkpoint,offset,none
```

Remove lower-priority pagination strategies and add vendor extensions:

```sh
openmorph --input ./openapi \
  --pagination-priority cursor,page,offset,none \
  --vendor-providers fern \
  --config fern-config.yaml \
  --dry-run
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

## Vendor Extensions

OpenMorph provides powerful vendor-specific extension injection capabilities, automatically adding pagination metadata like `x-fern-pagination` to your OpenAPI specifications. The system is provider-agnostic, configurable, and includes intelligent auto-detection of pagination patterns and array fields.

> **📝 Configuration Required**: Vendor extensions are configured via config files only (not CLI flags) due to their complexity. This design ensures maintainability, reusability, and supports advanced features like multiple providers, strategies, and field mappings.

### Key Features

- **Generic & Extensible** - Provider-agnostic configuration system supporting any vendor
- **Auto-Detection** - Automatically detects pagination patterns and result array fields
- **Smart Field Mapping** - Flexible parameter and response field mapping
- **Template-Based** - Configurable templates for vendor-specific extensions
- **Format Preservation** - Maintains consistent JSON/YAML formatting
- **Multi-Strategy Support** - Supports cursor, offset, page, and checkpoint pagination
- **Config-File Based** - Rich configuration via YAML/JSON for maximum flexibility

### Supported Vendors

- **Fern** - Adds `x-fern-pagination` extensions with full strategy support
- **Extensible Architecture** - Easy to add Speakeasy, OpenAPI Generator, and other vendors

### How It Works

1. **Scans API Operations** - Analyzes GET endpoints for pagination parameters
2. **Auto-Detects Strategies** - Identifies cursor, offset, page, and checkpoint patterns
3. **Finds Result Arrays** - Automatically locates array fields in response schemas (`data`, `items`, `results`, etc.)
4. **Applies Templates** - Uses configurable templates to generate vendor extensions
5. **Preserves Structure** - Maintains original OpenAPI formatting and structure

### Configuration

Vendor extensions are configured through the config file using the `vendor_extensions` section:

```yaml
vendor_extensions:
  enabled: true
  providers:
    fern:
      extension_name: "x-fern-pagination"
      target_level: "operation" # operation | path | global
      methods: ["get"] # HTTP methods to process
      field_mapping:
        request_params:
          cursor: ["cursor", "next_cursor", "after"]
          limit: ["limit", "size", "page_size", "per_page", "take"]
          offset: ["offset", "skip"]
          page: ["page", "page_number"]
          # results field mapping not needed - auto-detected!
      strategies:
        cursor:
          template:
            type: "cursor"
            cursor_param: "$request.{cursor_param}"
            page_size_param: "$request.{limit_param}"
            results_path: "$response.{results_field}"
          required_fields: ["cursor_param", "results_field"]
        offset:
          template:
            type: "offset"
            offset_param: "$request.{offset_param}"
            limit_param: "$request.{limit_param}"
            results_path: "$response.{results_field}"
          required_fields: ["offset_param", "results_field"]
        page:
          template:
            type: "page"
            page_param: "$request.{page_param}"
            page_size_param: "$request.{limit_param}"
            results_path: "$response.{results_field}"
          required_fields: ["page_param", "results_field"]
        checkpoint:
          template:
            type: "checkpoint"
            cursor_param: "$request.{cursor_param}"
            page_size_param: "$request.{limit_param}"
            results_path: "$response.{results_field}"
          required_fields: ["cursor_param", "results_field"]
```

### Usage Examples

**Add vendor extensions to all APIs:**

```sh
openmorph --input ./openapi --config config.yaml
```

**Add extensions for specific providers only:**

```sh
openmorph --input ./openapi --vendor-providers fern --config config.yaml
```

**Combine with other transformations:**

```sh
openmorph --input ./openapi \
  --mapping x-operation-group-name=x-fern-sdk-group-name \
  --vendor-providers fern \
  --pagination-priority cursor,offset,none \
  --flatten-responses \
  --backup \
  --config config.yaml
```

**Preview changes with dry-run:**

```sh
openmorph --input ./openapi --dry-run --config config.yaml
```

> **💡 Pro Tip**: Vendor extensions auto-enable when configured in your config file. The `--vendor-providers` flag filters which providers from your config are applied, allowing you to test specific vendors without modifying your config file.

### Auto-Detection Features

**Array Field Detection**: Automatically finds array fields in response schemas:

- `data`, `items`, `results`, `users`, `products`, etc.
- Works with complex schemas including `$ref`, `oneOf`, `anyOf`, `allOf`
- No manual configuration required!

**Parameter Mapping**: Maps request parameters to template variables:

- `cursor` → `$request.cursor`
- `limit` → `$request.limit`
- `page` → `$request.page`

### Example Output

**Before:**

```yaml
/users:
  get:
    parameters:
      - name: cursor
        in: query
      - name: size
        in: query
    responses:
      "200":
        content:
          application/json:
            schema:
              properties:
                data:
                  type: array
                  items: { type: object }
```

**After:**

```yaml
/users:
  get:
    parameters:
      - name: cursor
        in: query
      - name: size
        in: query
    responses:
      "200":
        content:
          application/json:
            schema:
              properties:
                data:
                  type: array
                  items: { type: object }
    x-fern-pagination:
      type: "cursor"
      cursor_param: "$request.cursor"
      page_size_param: "$request.size"
      results_path: "$response.data"
```

## Default Values

OpenMorph includes a powerful default values feature that allows you to automatically set default values throughout your OpenAPI specifications. This feature supports complex rule-based matching and can be applied to parameters, request bodies, response schemas, and component schemas.

### Overview

The Default Values feature allows you to:

- Set defaults for **parameter schemas** (path, query, header, cookie)
- Set defaults for **request body schemas**
- Set defaults for **response schemas**
- Set defaults for **component schemas** (reusable objects)
- Apply defaults to **arrays** and **enum fields**
- Use **regex patterns** for flexible property matching
- Configure **rule priorities** for precise control

### Configuration

Default values are configured through the `default_values` section in your config file:

```yaml
default_values:
  enabled: true
  rules:
    # Set default limit for pagination parameters
    query_limit_defaults:
      target:
        location: "parameter"
      condition:
        parameter_in: "query"
        type: "integer"
        property_name: "(limit|size|page_size|per_page)"
      value: 20
      priority: 10

    # Set default sort direction
    query_sort_defaults:
      target:
        location: "parameter"
      condition:
        parameter_in: "query"
        type: "string"
        property_name: "(sort|order|direction)"
      value: "asc"
      priority: 9

    # Boolean fields default to true
    boolean_defaults:
      target:
        location: "component"
      condition:
        type: "boolean"
        property_name: "(active|enabled|is_.*)"
      value: true
      priority: 8
```

### Configuration Options

#### Target

- `location`: Where to apply defaults
  - `"parameter"` - URL parameters (query, path, header, cookie)
  - `"request_body"` - Request body schemas
  - `"response"` - Response body schemas
  - `"component"` - Component schemas (reusable objects)
- `property`: Optional specific property name to target
- `path`: Optional JSONPath-like selector for precise targeting

#### Conditions

- `type`: Schema type constraint (`"string"`, `"integer"`, `"boolean"`, `"array"`, `"object"`)
- `parameter_in`: For parameters - where they're located (`"query"`, `"path"`, `"header"`, `"cookie"`)
- `http_methods`: List of HTTP methods to target (`["get", "post"]`)
- `path_patterns`: List of regex patterns for API paths (`["/api/v1/.*"]`)
- `has_enum`: Only apply to fields with enum constraints
- `is_array`: Only apply to array-type fields
- `property_name`: Regex pattern to match property names (`"(limit|size|page_size)"`)
- `required`: Apply only to required (`true`) or optional (`false`) fields

#### Values

- `value`: Simple default value (string, number, boolean, array, object)
- `template`: Complex template object for structured defaults
- `priority`: Rule priority (higher numbers = higher priority)

### Usage Examples

**Apply defaults using config file:**

```bash
openmorph --input ./openapi --config config.yaml
```

**Preview defaults changes:**

```bash
openmorph --input ./openapi --config config.yaml --dry-run
```

**Combine with other transformations:**

```bash
openmorph --input ./openapi \
  --mapping x-operation-group-name=x-fern-sdk-group-name \
  --vendor-providers fern \
  --pagination-priority cursor,offset,none \
  --flatten-responses \
  --config config.yaml
```

### Advanced Examples

**Complex object defaults:**

```yaml
default_values:
  enabled: true
  rules:
    settings_defaults:
      target:
        location: "component"
      condition:
        type: "object"
        property_name: "settings"
      template:
        theme: "light"
        notifications: true
        language: "en"
      priority: 4
```

**Array response defaults:**

```yaml
default_values:
  enabled: true
  rules:
    array_defaults:
      target:
        location: "response"
      condition:
        type: "array"
        http_methods: ["get"]
        path_patterns: ["/api/v1/.*"]
      value: []
      priority: 5
```

### Rule Priority and Ordering

Rules are processed in priority order (highest priority first), allowing you to:

1. Set broad defaults with low priority
2. Override with specific defaults using higher priority
3. Ensure consistent application order across runs

```yaml
default_values:
  enabled: true
  rules:
    # Broad rule - low priority
    all_strings:
      condition:
        type: "string"
      value: "default"
      priority: 1

    # Specific rule - high priority (overrides above)
    user_names:
      condition:
        type: "string"
        property_name: "name"
      value: "Anonymous"
      priority: 10
```

### Best Practices

1. **Use Regex Patterns**: Property name matching supports regex for flexible targeting
2. **Prioritize Rules**: Use priority to control application order
3. **Test with Dry-Run**: Always preview changes before applying
4. **Backup Files**: Enable backup for safe operations
5. **Combine Features**: Use alongside vendor extensions and other transformations
6. **Document Rules**: Use clear rule names and comments in config files

### Integration

The defaults feature integrates seamlessly with other OpenMorph features:

- **Vendor Extensions**: Applied before vendor extensions
- **Response Flattening**: Works on original schemas before flattening
- **Validation**: Validates resulting OpenAPI specs
- **Interactive Mode**: Preview all changes together
- **Backup**: Automatic backup before modifications

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
