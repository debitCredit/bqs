# BQS - BigQuery Schema Tool

A fast, lightweight CLI tool for BigQuery metadata inspection and schema operations.

## Overview

BQS is a Go-based command-line tool that provides a clean interface to BigQuery table and view metadata. It replaces complex bash scripts with a single binary that's easy to install and use.

## Features

- 🚀 **Fast**: Single binary with no dependencies
- 🔧 **Simple**: Clean command-line interface
- 🌊 **Pipeable**: Supports Unix pipes for data processing
- 🎯 **Focused**: Designed specifically for BigQuery metadata operations

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

Display complete metadata for a BigQuery table or view in JSON format.

```bash
bqs show PROJECT.DATASET.TABLE
```

**Examples:**

```bash
# Show table metadata
bqs show my-project.analytics.user_events

# Show view metadata
bqs show my-project.reporting.daily_summary

# Pipe output to jq for processing
bqs show my-project.analytics.user_events | jq '.schema.fields[].name'

# Save metadata to file
bqs show my-project.analytics.user_events > table_metadata.json

# Extract just the schema
bqs show my-project.analytics.user_events | jq '.schema'
```

## Output Format

The tool outputs complete BigQuery metadata in JSON format, including:

- Table/view schema with field definitions
- Table properties (creation time, modification time, etc.)
- Statistics (row count, size, etc.)
- Partitioning and clustering information
- View SQL definition (for views)

## How It Works

BQS parses your `project.dataset.table` input and executes:

```bash
bq show --project_id=PROJECT --format=prettyjson DATASET.TABLE
```

This approach:
- ✅ Uses existing `bq` authentication
- ✅ Doesn't modify your `gcloud` configuration
- ✅ Provides complete metadata in a single command
- ✅ Supports all BigQuery resources (tables, views, materialized views)

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

- [ ] Additional output formats (table, yaml)
- [ ] Schema-only display options
- [ ] Support for dataset listing
- [ ] Formatted table output with colors
- [ ] Configuration file support
- [ ] Homebrew distribution