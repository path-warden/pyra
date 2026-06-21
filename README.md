# OKFy

OKFy converts documentation websites and local Markdown folders into Open Knowledge Format (OKF) bundles. These bundles can be served via MCP (Model Context Protocol) to AI agents like Claude, Codex, or Cursor.

## Installation

### Download Binary

Download the latest binary for your platform from the [releases page](https://github.com/okf-cli/okf-mcp/releases).

### Build from Source

```bash
go install github.com/okf-cli/okf-mcp/cmd/okf-cli@latest
```

Or clone and build:

```bash
git clone https://github.com/okf-cli/okf-mcp.git
cd okf-mcp
make build
```

## Quick Start

### 1. Crawl a Documentation Site

```bash
okf-cli crawl https://docs.example.com --out ./my-bundle
```

### 2. Or Import Local Markdown

```bash
okf-cli import ./docs --out ./my-bundle
```

### 3. Validate Your Bundle

```bash
okf-cli validate ./my-bundle
```

### 4. Serve via MCP

```bash
okf-cli serve ./my-bundle --mcp
```

### 5. Configure Your AI Client

Add to your MCP client configuration (e.g., Claude Desktop):

```json
{
  "mcpServers": {
    "my-docs": {
      "command": "okf-cli",
      "args": ["serve", "./my-bundle", "--mcp"]
    }
  }
}
```

## Commands

### `okf-cli crawl <url>`

Crawl a documentation website and create an OKF bundle.

```bash
okf-cli crawl https://docs.example.com --out ./bundle [options]
```

Options:
- `--out, -o` - Output directory (required)
- `--max-pages` - Maximum pages to crawl (default: 100)
- `--max-depth` - Maximum crawl depth (default: 4)
- `--include` - Include patterns (glob or regex)
- `--exclude` - Exclude patterns
- `--same-origin` - Stay on same origin (default: true)
- `--respect-robots` - Respect robots.txt (default: true)
- `--concurrency` - Fetch concurrency (default: 4)
- `--force` - Overwrite output directory
- `--dry-run` - List pages without crawling

### `okf-cli import <path>`

Import local files into an OKF bundle.

```bash
okf-cli import ./docs --out ./bundle [options]
```

Options:
- `--out` - Output directory (required)
- `--source-name` - Bundle title
- `--include` - Include patterns
- `--exclude` - Exclude patterns
- `--force` - Overwrite output directory

### `okf-cli validate <bundle>`

Validate an OKF bundle.

```bash
okf-cli validate ./bundle [--json]
```

### `okf-cli inspect <bundle>`

Display bundle statistics.

```bash
okf-cli inspect ./bundle
```

### `okf-cli serve <bundle>`

Start an MCP server for a bundle.

```bash
okf-cli serve ./bundle --mcp
```

Options:
- `--mcp` - Use MCP stdio transport (default: true)
- `--name` - Server name
- `--max-result-chars` - Maximum characters in tool results (default: 12000)

### `okf-cli update <bundle>`

Update an existing OKF bundle from its original source.

```bash
okf-cli update ./bundle [options]
```

The source is automatically read from the bundle's `changelog.txt` file (created during crawl or import). You can override it with the `--source` flag.

Options:
- `--source, -s` - Override source URL or path
- `--force` - Apply all changes without prompting
- `--dry-run` - Show changes without applying them
- `--max-pages` - Maximum pages to crawl, for URL sources (default: 100)
- `--max-depth` - Maximum crawl depth, for URL sources (default: 4)
- `--concurrency` - Fetch concurrency, for URL sources (default: 4)
- `--include` - Include patterns
- `--exclude` - Exclude patterns

Example workflow:
```bash
# Initial crawl
okf-cli crawl https://docs.example.com --out ./my-bundle

# Later, update with changes
okf-cli update ./my-bundle --dry-run  # Preview changes
okf-cli update ./my-bundle --force    # Apply all changes
okf-cli update ./my-bundle            # Interactive mode
```

### `okf-cli demo`

Run an offline demo with the bundled example.

```bash
okf-cli demo [--serve]
```

## MCP Tools

When serving a bundle via MCP, the following tools are available to AI agents:

### Search & Read Tools

| Tool | Description |
|------|-------------|
| `search_concepts` | Full-text search across concepts with token budget control |
| `read_concept` | Read a specific concept's content with compression options |
| `get_neighbors` | Find related concepts via links |
| `get_context` | Smart context assembly for a topic with token budget |
| `list_types` | List all concept types in the bundle |
| `list_tags` | List all tags in the bundle |
| `bundle_summary` | Get bundle statistics |

### Live Update Tools

| Tool | Description |
|------|-------------|
| `check_updates` | Check if the bundle source has updates available |
| `apply_updates` | Apply pending updates from the source |
| `bundle_health` | Check bundle health, staleness, and source reachability |

### Utility Tools

| Tool | Description |
|------|-------------|
| `compression_stats` | View token compression statistics for this session |

### Token Budget & Compression

The search and read tools support token-aware responses to help AI agents manage context windows efficiently:

**Parameters:**
- `token_budget` - Maximum tokens for the response (estimates using cl100k_base encoding)
- `compression` - Compression level: `none`, `light`, `medium`, `aggressive`
- `detail_level` - Detail level 0-3 (0=minimal, 3=full content)

**Compression Levels:**
| Level | Effect |
|-------|--------|
| `none` | No compression, full content |
| `light` | Normalize whitespace, collapse blank lines |
| `medium` | Light + truncate to section boundaries with outline |
| `aggressive` | Medium + aggressive truncation with retrieval hints |

**Example: Budget-Aware Search**
```json
{
  "tool": "search_concepts",
  "arguments": {
    "query": "authentication",
    "token_budget": 2000,
    "compression": "medium",
    "detail_level": 2
  }
}
```

**Example: Get Context for a Topic**
```json
{
  "tool": "get_context",
  "arguments": {
    "query": "how to authenticate users",
    "token_budget": 4000,
    "compression": "light"
  }
}
```

### Live Updates

Bundles can be updated from their original source while the MCP server is running:

**Check for Updates**
```json
{
  "tool": "check_updates",
  "arguments": {
    "timeout_seconds": 30
  }
}
```

Response includes `has_changes`, `added`, `modified`, `deleted` counts.

**Apply Updates**
```json
{
  "tool": "apply_updates",
  "arguments": {
    "confirm": true
  }
}
```

Use `dry_run: true` to preview changes without applying them.

## Open Knowledge Format

OKF bundles are directories containing Markdown files with YAML frontmatter:

```markdown
---
type: Guide
title: Getting Started
description: Learn how to get started with the product.
tags:
  - quickstart
  - tutorial
resource: https://docs.example.com/getting-started
---
# Getting Started

Your content here...
```

### Required Fields

- `type` - Concept type (Guide, API Reference, Concept, etc.)

### Optional Fields

- `title` - Document title
- `description` - Brief description (max 180 chars)
- `tags` - Array of topic tags
- `resource` - Original source URL
- `timestamp` - Last modified date

## Development

```bash
# Run tests
make test

# Run linter
make lint

# Build for all platforms
make build-all

# Run demo
make demo
```

## License

MIT
