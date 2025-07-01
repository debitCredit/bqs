# BQS - BigQuery Schema Tool

A fast, interactive CLI tool for exploring BigQuery datasets and tables with intelligent caching and a beautiful terminal interface.

## Overview

BQS is a Go-based command-line tool that transforms BigQuery exploration from complex bash scripts into an intuitive, interactive experience. It features persistent caching, a terminal-based UI, and multiple exploration modes.

## Features

### 🚀 Interactive Dataset Browser
- **Terminal UI**: Navigate datasets with keyboard shortcuts
- **Fast & Scalable**: Browse thousands of tables instantly with basic info
- **Rich Detail Views**: Get complete metadata when exploring specific tables
- **Cache Indicators**: Visual markers (✓) show which tables are cached for instant access

### ⚡ Smart Caching
- **Persistent Storage**: SQLite-based cache survives between sessions
- **TTL Management**: Different cache lifetimes for different data types
- **Automatic Cleanup**: Expired entries are automatically removed
- **Cache Status**: Always know what's cached vs. fresh from BigQuery

### 🎯 Multiple Commands
- `browse` - Interactive dataset exploration with TUI
- `show` - Display table metadata with optional editor integration
- `schema` - Pretty-print table schemas with nested field support

## Installation

```bash
# Install with Go
go install github.com/yourusername/bqs@latest

# Or build from source
git clone https://github.com/yourusername/bqs
cd bqs
go build -o bqs
```

## Quick Start

### Browse a Dataset Interactively
```bash
# Browse dataset with fast, scalable table list
bqs browse my-project.analytics

# Explore specific table with complete metadata
bqs browse my-project.analytics.events
```

### View Table Metadata
```bash
# Display metadata in terminal
bqs show my-project.analytics.events

# Open in your preferred editor
bqs show --editor code my-project.analytics.events
```

### Display Table Schema
```bash
# Pretty-formatted schema with nested fields
bqs schema my-project.analytics.events
```

## Interactive Browser Controls

| Key | Action |
|-----|--------|
| `↑↓` or `jk` | Navigate table list |
| `Enter` | Explore selected table |
| `Space` or `→` | Expand schema field |
| `←` or `h` | Collapse schema field |
| `b` or `Backspace` | Back to table list |
| `q` or `Ctrl+C` | Quit |

## Commands

### `bqs browse` - Interactive Dataset Browser

Explore BigQuery datasets interactively with a terminal-based UI.

```bash
bqs browse [flags] PROJECT.DATASET
```

**Features:**
- Navigate tables with arrow keys or vim-style controls  
- Visual cache indicators (✓) for fast table access
- Fast browsing of thousands of tables with basic info
- Rich metadata views when exploring specific tables
- Expandable schema exploration with nested fields
- Seamless fallback to static mode if terminal UI fails

### `bqs show` - Table Metadata Display

Display complete table metadata with optional editor integration.

```bash
bqs show [flags] PROJECT.DATASET.TABLE
```

**Flags:**
- `--editor` - Open metadata in specified editor (vim, code, zed, etc.)
- `--format` - Output format options
- `--project` - Override project ID

### `bqs schema` - Schema Display

Pretty-print table schemas with support for nested and repeated fields.

```bash
bqs schema PROJECT.DATASET.TABLE
```

**Features:**
- Hierarchical display of nested fields
- Field type and mode indicators (REQUIRED, REPEATED)
- Color-coded output for better readability

## Caching System

BQS uses intelligent caching to speed up repeated operations:

- **Table Lists**: Cached for 5 minutes (datasets don't change often)
- **Table Metadata**: Cached for 15 minutes (balanced freshness/speed)
- **Table Schemas**: Cached for 30 minutes (schemas rarely change)

Cache is stored in `~/.cache/bqs/` (follows XDG standards).

### Cache Configuration
```bash
# Custom cache directory
export BQS_CACHE_DIR=/path/to/cache

# Use XDG cache directory
export XDG_CACHE_HOME=/custom/cache
```

## Examples

### Exploring a Dataset
```bash
$ bqs browse my-project.web_analytics

📊 my-project.web_analytics

Cache  Table           Type   Created
✓      events          TABLE  Dec 1 10:30
✓      sessions        TABLE  Dec 1 10:31
       page_views      VIEW   Dec 2 14:22
       user_metrics    TABLE  Dec 3 09:15

⌨️  [↑↓] Navigate • [Enter] Explore • [q] Quit • ✓ = Cached
```

### Viewing Table Details
```bash
$ bqs show my-project.web_analytics.events

📊 my-project.web_analytics.events (TABLE)
📈 1,234,567 rows • 💾 2.3 GB • 🕒 Modified Dec 3 14:30

Opens metadata in your preferred editor or displays in terminal
```

### Schema-Only View
```bash
$ bqs schema my-project.web_analytics.events

🌲 Schema: events
├─ event_id STRING REQUIRED
├─ user_id STRING
├─ timestamp TIMESTAMP REQUIRED
├─ event_data RECORD
│  ├─ page_url STRING
│  ├─ referrer STRING
│  └─ custom_params RECORD REPEATED
│     ├─ key STRING
│     └─ value STRING
└─ device_info RECORD
   ├─ browser STRING
   └─ platform STRING
```

## Configuration

### Global Flags
- `--project` - Override the default GCP project
- `--editor` - Set preferred editor (vim, code, zed, etc.)


### Environment Variables
- `BQS_CACHE_DIR` - Custom cache directory
- `XDG_CACHE_HOME` - XDG-compliant cache directory
- `GOOGLE_APPLICATION_CREDENTIALS` - Service account key file

## Requirements

- Google Cloud SDK (`gcloud`) installed and authenticated
- `bq` command-line tool (included with gcloud)
- Valid BigQuery access permissions

## Authentication

BQS uses your existing Google Cloud authentication:

```bash
# Login with your user account
gcloud auth login

# Or use application default credentials
gcloud auth application-default login

# Or set service account key
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/key.json
```

## Development

### Project Structure

```
bqs/
├── cmd/
│   ├── root.go           # Root command and CLI setup
│   ├── show.go           # Table metadata display with editor
│   ├── browse.go         # Interactive dataset browser (main logic)
│   ├── browser_model.go  # Bubble Tea model and state management
│   ├── browser_view.go   # UI rendering and display logic
│   ├── schema_tree.go    # Schema tree navigation and display
│   ├── cache.go          # Cache management commands
│   └── docs.go           # Documentation generation
├── internal/
│   ├── bigquery/         # BQ client wrapper (bq CLI integration)
│   │   ├── client.go     # Main BigQuery client with caching
│   │   └── *_test.go     # Comprehensive test suite
│   ├── cache/            # SQLite caching with TTL management
│   │   ├── cache.go      # Core cache implementation
│   │   ├── interface.go  # Cache service interface
│   │   ├── mock.go       # Mock implementation for testing
│   │   └── *_test.go     # Full test coverage
│   ├── config/           # Centralized configuration management
│   │   ├── config.go     # TTL constants and UI dimensions
│   │   └── config_test.go # Configuration validation tests
│   ├── utils/            # Shared utilities
│   │   ├── format.go     # Byte formatting utilities
│   │   ├── cache.go      # Cache initialization helpers
│   │   └── *_test.go     # Utility function tests
│   └── validation/       # Input validation and error handling
│       ├── input.go      # BigQuery identifier validation
│       └── input_test.go # Validation test suite
├── main.go               # Application entry point
├── go.mod                # Go module definition
├── CLAUDE.md             # Project memory and documentation
└── README.md             # This file
```

### Building
```bash
go build -o bqs
```

### Testing
```bash
go test ./...
```

### Architecture & Quality

**Clean Architecture:**
- **Modular Design**: Focused file separation with single-responsibility principle
- **Testable Interfaces**: Cache and validation layers with comprehensive mocking
- **Configuration Management**: Centralized constants and settings
- **Input Validation**: Robust BigQuery identifier validation with clear error messages

**Testing & Quality:**
- **100% Test Coverage**: All utility functions and core logic tested
- **Mock Implementations**: Full mock cache service for isolated testing
- **Integration Tests**: End-to-end validation of BigQuery client operations
- **Error Handling**: Graceful degradation and consistent error messages

### Dependencies
- `github.com/spf13/cobra` - CLI framework and command structure
- `github.com/charmbracelet/bubbletea` - Terminal UI and interactive components
- `github.com/charmbracelet/bubbles` - Pre-built UI components (tables, etc.)
- `github.com/charmbracelet/lipgloss` - Terminal styling and layout
- `github.com/jedib0t/go-pretty/v6` - Table formatting for static output
- `modernc.org/sqlite` - SQLite driver for persistent caching
- Native `bq` CLI tool for BigQuery operations (no API client dependencies)

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

MIT License - see LICENSE file for details.

## Acknowledgments

- Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the terminal UI
- Uses [Cobra](https://github.com/spf13/cobra) for CLI framework
- Caching powered by SQLite