# BQS - BigQuery Schema Tool

A fast, lightweight CLI tool for BigQuery metadata inspection and schema operations.

## Overview

BQS is a Go-based command-line tool that provides a clean interface to BigQuery table and view metadata. It replaces complex bash scripts with a single binary that's easy to install and use.

## Features

- 🚀 **Fast**: Single binary with no dependencies
- 🔧 **Simple**: Clean command-line interface with full `bq show` parity
- 🌊 **Pipeable**: Supports Unix pipes for data processing
- 🎯 **Focused**: Designed specifically for BigQuery metadata operations
- ⚡ **Complete**: Supports tables, views, materialized views, and all output formats

## Prerequisites

- `bq` command-line tool installed and configured
- Google Cloud authentication set up (via `gcloud auth` or service account)

## Installation

### Build from Source

```bash
git clone https://github.com/yourusername/bqs.git
cd bqs
go build -o bqs .
```

### Usage

BQS currently supports the `show` command for displaying table and view metadata.

## Commands

### `bqs show`

Display metadata for BigQuery tables, views, and materialized views with full `bq show` command parity.

```bash
bqs show [flags] PROJECT.DATASET.TABLE
```

**Flags:**
- `--schema` - Show only the schema
- `--view` - Show view-specific details including SQL definition
- `--materialized-view` - Show materialized view details
- `--format` - Output format: `json`, `prettyjson`, `pretty`, `sparse`, `csv` (default: `prettyjson`)
- `--project` - Override project ID
- `--quiet` - Suppress status updates

**Examples:**

```bash
# Show complete table metadata (default prettyjson format)
bqs show my-project.analytics.user_events

# Show only the schema
bqs show --schema my-project.analytics.user_events

# Show view details including SQL definition
bqs show --view my-project.reporting.daily_summary

# Show materialized view with refresh info
bqs show --materialized-view my-project.analytics.user_summary_mv

# Different output formats
bqs show --format json my-project.analytics.user_events
bqs show --format pretty my-project.analytics.user_events
bqs show --format csv my-project.analytics.user_events

# Override project (useful for cross-project access)
bqs show --project other-project dataset.table

# Combine flags
bqs show --schema --format json --quiet my-project.analytics.user_events

# Pipe to jq for processing
bqs show --format json my-project.analytics.user_events | jq '.schema.fields[].name'

# Save schema to file
bqs show --schema --format json my-project.analytics.user_events > schema.json
```

## Output Format

The tool outputs complete BigQuery metadata in JSON format, including:

- Table/view schema with field definitions
- Table properties (creation time, modification time, etc.)
- Statistics (row count, size, etc.)
- Partitioning and clustering information
- View SQL definition (for views)

## How It Works

BQS provides a user-friendly wrapper around the `bq show` command with **complete parity**. It parses your input and translates flags to the equivalent `bq` command:

```bash
# Your command:
bqs show --schema --format json my-project.dataset.table

# Equivalent bq command:
bq show --project_id=my-project --schema --format=json dataset.table
```

## `bq show` Command Parity

✅ **Full compatibility** with `bq show` functionality:

| Feature | `bq show` | `bqs show` | Notes |
|---------|-----------|------------|---------|
| Tables | ✅ | ✅ | Complete metadata |
| Views | ✅ | ✅ | Includes SQL definition with `--view` |
| Materialized Views | ✅ | ✅ | Refresh policies with `--materialized-view` |
| Schema only | `--schema` | `--schema` | Schema fields only |
| Output formats | `--format` | `--format` | json, prettyjson, pretty, sparse, csv |
| Project override | `--project_id` | `--project` | Cross-project access |
| Quiet mode | `--quiet` | `--quiet` | Suppress status messages |

**Advantages over direct `bq show`:**
- ✅ Simpler syntax (no need to set project context)
- ✅ Doesn't modify your `gcloud` configuration
- ✅ Consistent project.dataset.table format
- ✅ Enhanced help and documentation
- ✅ Future extensibility for additional features

## Development

### Project Structure

```
bqs/
├── cmd/
│   ├── root.go     # Root command and CLI setup
│   └── show.go     # Show command implementation
├── main.go         # Application entry point
├── go.mod          # Go module definition
└── README.md       # This file
```

### Building

```bash
# Build binary
go build -o bqs .

# Run tests
go test ./...

# Run with go
go run . show PROJECT.DATASET.TABLE
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

MIT License - see LICENSE file for details.

## Roadmap

### Completed ✅
- [x] Full `bq show` command parity
- [x] All output formats (json, prettyjson, pretty, sparse, csv)
- [x] Schema-only display (`--schema`)
- [x] View and materialized view support
- [x] Project override functionality

### In Progress 🚧
- [ ] Formatted table output with colors and improved readability
- [ ] Stdin support for piping table identifiers
- [ ] Support for dataset listing (`bqs list`)

### Future 🔮
- [ ] GoReleaser setup for automated releases
- [ ] Homebrew distribution
- [ ] Configuration file support
- [ ] Additional output formats (yaml)
- [ ] Multi-region location support