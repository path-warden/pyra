---
type: Concept
title: MCP Protocol
description: The Model Context Protocol enables AI agents to access documentation through standardized tools.
tags:
  - mcp
  - protocol
  - ai-agents
---
# MCP Protocol

The Model Context Protocol (MCP) is a standard for AI agents to interact with external tools and data sources.

## How Pyra Uses MCP

Pyra serves OKF bundles through an MCP stdio server, exposing tools that AI agents can use:

### Available Tools

1. **search_concepts** - Full-text search across the bundle
2. **read_concept** - Read a specific concept's content
3. **get_neighbors** - Find related concepts via links
4. **list_types** - List all concept types in the bundle
5. **list_tags** - List all tags in the bundle
6. **bundle_summary** - Get bundle statistics

## Configuration

Add to your MCP client config:

```json
{
  "mcpServers": {
    "my-docs": {
      "command": "pyra",
      "args": ["serve", "./my-bundle", "--mcp"]
    }
  }
}
```
