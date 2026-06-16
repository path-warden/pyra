---
type: API Reference
title: CLI Commands
description: Complete reference for all OKFy command-line commands and options.
tags:
  - cli
  - reference
  - commands
---
# CLI Commands

## okf-cli crawl

Crawl a documentation website and create an OKF bundle.

```bash
okf-cli crawl <url> --out <dir> [options]
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

## okf-cli import

Import local files into an OKF bundle.

```bash
okf-cli import <path> --out <dir> [options]
```

### Options

- `--out` - Output directory (required)
- `--source-name` - Bundle title
- `--include` - Include patterns
- `--exclude` - Exclude patterns
- `--force` - Overwrite output directory

## okf-cli validate

Validate an OKF bundle.

```bash
okf-cli validate <bundle> [--json]
```

## okf-cli inspect

Display bundle statistics.

```bash
okf-cli inspect <bundle>
```

## okf-cli serve

Start an MCP server for a bundle.

```bash
okf-cli serve <bundle> --mcp
```

## okf-cli demo

Run an offline demo with a bundled example.

```bash
okf-cli demo [--serve]
```
