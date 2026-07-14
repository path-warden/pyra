---
type: API Reference
title: CLI Commands
description: Complete reference for all Pyra command-line commands and options.
tags:
  - cli
  - reference
  - commands
---
# CLI Commands

## pyra crawl

Crawl a documentation website and create an OKF bundle.

```bash
pyra crawl <url> --out <dir> [options]
```

### Options

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

## pyra import

Import local files into an OKF bundle.

```bash
pyra import <path> --out <dir> [options]
```

### Options

- `--out` - Output directory (required)
- `--source-name` - Bundle title
- `--include` - Include patterns
- `--exclude` - Exclude patterns
- `--force` - Overwrite output directory

## pyra validate

Validate an OKF bundle.

```bash
pyra validate <bundle> [--json]
```

## pyra inspect

Display bundle statistics.

```bash
pyra inspect <bundle>
```

## pyra serve

Start an MCP server for a bundle.

```bash
pyra serve <bundle> --mcp
```

## pyra demo

Run an offline demo with a bundled example.

```bash
pyra demo [--serve]
```
