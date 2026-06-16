---
type: Guide
title: Getting Started with OKFy
description: Learn how to install and use OKFy to create OKF bundles from your documentation.
tags:
  - quickstart
  - installation
  - tutorial
---
# Getting Started with OKFy

OKFy is a tool that converts documentation websites and local Markdown folders into Open Knowledge Format (OKF) bundles. These bundles can be served via MCP to AI agents like Claude, Codex, or Cursor.

## Installation

Download the latest binary for your platform from the releases page, or build from source:

```bash
go install github.com/okf-cli/okf-mcp/cmd/okf-cli@latest
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

## Next Steps

- Read about [OKF Concepts](concepts/okf-format.md)
- Explore the [CLI Reference](cli/commands.md)
