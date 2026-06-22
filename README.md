# OKF-CLI

<p align="center">
<img width="704" alt="okf-cli_github" src="https://github.com/user-attachments/assets/dcb42aa0-9ad5-4c05-810c-db8a19bda9b1" />
</p>

OKF-CLI converts documentation websites and local Markdown folders into extended Open Knowledge Format (OKF) bundles. These bundles implement a filing cabinet concept architecture pattern for AI agents, providing durable structured artifacts with summary-first navigation that scales sublinearly with corpus size.

## Why?
Current AI memory systems can be broken down into three types:

- **Notebook**: Built for capture, drafting, and personal connection-making. 
  - Example: An Obsidian vault is a platonic notebook: a folder of markdown files, freeform linking, and portable. Its strengths are also its limits however. It has no structured retrieval as to perform a search you need to be a full text search. It has no referential integrity as you can link to a note that doesn't exist and Obsidian will not stop you. It has no concurrency safety so it does not support two writers or you end up with conflicts. It scales to any number of notes for _one person doing their own thinking_. As soon as multiple parties and writes occur, it falters.
- **Database**: Engineered for large-scale, multi-user, precision retrieval. These are commonly vector stores (like Pinecone or Milvus) and relational databases (like PostgreSQL). They can scale to millions of items, support concurrent writes, provide ACID guarantees, and produce audit logs. The cost is complex setup, operational maintenance, heavy infrastructure, opaque embeddings that can mislead, and a deployment story that does not look anywhere as simple as a "drop these files in a folder" situation.

There needs to be a middle ground that can serve most functions without the extremes of both ends.
- A **filing cabinet** is a notebook with a structured layer on top. The cabinet's drawers: the file folders, the labels, the sorting rules, etc., these are not the documents themselves. They are a navigation system that makes the documents findable without reading all of them. In the vault concept, this means frontmatter conventions, summary callouts, an index file, a backlinking discipline, and an agent loop that maintains all of that. The strengths are summary-first navigation, traceable answers, and no vector infrastructure. 
  - The limits are real too though as at a scale of up to about 100 articles and roughly 400,000 words, any LLM's ability to navigate via summaries and index pages appears sufficient and the overhead and complexity of a full RAG stack would likely introduce more latency and retrieval noise than it removes. But past that ceiling, summary navigation starts producing noise faster than it removes it. A RAG search and retrieval system becomes less an additional burden and more of a requirement.

## The Filing Cabinet Architecture

Open Knowledge Format knowledge bundles [https://openknowledgeformat.com/what-is-okf] are designed to be a 'human and agent' readable bundle of Markdown files with YAML frontmatter. People can author it, agents can generate it, and tools can exchange it without a central registry or proprietary SDK.

OKF bundles can be extended to provide functionality as a "filing cabinet" for AI agents - a persistent, structured knowledge store that lives outside the context window. Key properties:

- **Summary-first navigation**: Each concept has a summary callout, and the index provides inline summaries so agents can decide what to read without paying full token cost
- **Bidirectional backlinks**: Concepts track both outbound links and backlinks in frontmatter, enabling graph traversal
- **Scale ceiling detection**: The `inspect` command warns when bundles exceed ~100 concepts or ~400K tokens, signaling when to consider adding a RAG implementation instead
- **Token-aware retrieval**: Tools support token budgets and compression levels to fit responses within context windows

This architecture allows AI agents to efficiently navigate large documentation sets by reading summaries first, then drilling into specific concepts as needed.

## Installation

### Download Binary

Download the latest binary for your platform from the [releases page](https://github.com/okf-cli/okf-mcp/releases).

### Build from Source

```bash
go install github.com/chasedputnam/okf-cli/cmd/okf-cli@latest
```

Or clone and build:

```bash
git clone https://github.com/chasedputnam/okf-cli.git
cd okf-cli
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

#### Ex: Enable an Existing Repository

Turn any repository with scattered Markdown files into a searchable knowledge bundle:

```bash
# Import all Markdown files from a repository
okf-cli import ~/repo/my-project --out ~/repo/my-project/.okf --source-name "My Project"

# Filter to specific directories or patterns
okf-cli import ~/repo/my-project \
  --out ~/repo/my-project/.okf \
  --source-name "My Project" \
  --include "docs/**/*.md" \
  --include "**/*.mdx" \
  --include "**/README.md" \
  --exclude "node_modules/**" \
  --exclude "vendor/**"

# Add .okf to .gitignore (optional)
echo ".okf/" >> ~/repo/my-project/.gitignore

# Serve to AI agents
okf-cli serve ~/repo/my-project/.okf --mcp
```

This creates a `.okf` bundle inside your repository that indexes all documentation, READMEs, ADRs, and other Markdown content. AI agents can then search and navigate your project's knowledge base.

**Example: Enable a monorepo**
```bash
okf-cli import ~/repo/cloud-platform \
  --out ~/repo/cloud-platform/.okf \
  --source-name "Cloud Platform" \
  --include "**/docs/**/*.md" \
  --include "**/README.md" \
  --include "**/ARCHITECTURE.md" \
  --include "**/adr/**/*.md" \
  --exclude "**/test/**" \
  --exclude "**/fixtures/**"
```

**Keep the bundle updated**
```bash
# Re-import when docs change
okf-cli update ~/repo/my-project/.okf --force
```

### 4. Validate Your Bundle

```bash
okf-cli validate ./my-bundle
```

### 5. Serve via MCP

```bash
okf-cli serve ./my-bundle --mcp
```

### 6. Configure Your AI Client

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

Validate an OKF bundle structure and health.

```bash
okf-cli validate ./bundle [--json]
```

Validates:
- Index structure and frontmatter
- Concept frontmatter (required `type` field)
- Internal link integrity (broken links)
- **Filing cabinet health**: missing summary callouts, summary length, scale ceiling

Missing summaries produce warnings (not errors) for backward compatibility with older bundles.

### `okf-cli inspect <bundle>`

Display bundle statistics and scale metrics.

```bash
okf-cli inspect ./bundle
okf-cli inspect ./bundle --recommendations
```

Output includes:
- Concept count, link count, broken links, orphan concepts
- Type and tag distribution
- **Scale metrics**: total tokens, average tokens per concept, index ratio
- **Scale status**: healthy, warning (approaching ceiling), or exceeded

Options:
- `--recommendations` - Show RAG graduation guidance if scale ceiling is exceeded

When a bundle exceeds ~100 concepts or ~400K tokens, `inspect` warns that the filing cabinet pattern is approaching its scale ceiling. Use `--recommendations` to see guidance on adding vector search.

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
| `get_neighbors` | Find related concepts via outbound links and backlinks |
| `get_context` | Smart context assembly for a topic with token budget |
| `list_types` | List all concept types in the bundle |
| `list_tags` | List all tags in the bundle |
| `bundle_summary` | Get bundle statistics, scale metrics, and index content |

### Live Update Tools

| Tool | Description |
|------|-------------|
| `check_updates` | Check if the bundle source has updates available |
| `apply_updates` | Apply pending updates from the source (regenerates summaries and backlinks) |
| `bundle_health` | Check bundle health, scale ceiling, missing summaries, and source reachability |

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

OKF bundles are directories containing Markdown files with YAML frontmatter. The format implements the filing cabinet pattern with summary callouts and bidirectional backlinks.

### Concept Format

```markdown
---
type: Guide
title: Getting Started
description: Learn how to get started with the product.
tags:
  - quickstart
  - tutorial
resource: https://docs.example.com/getting-started
backlinks:
  - concepts/authentication
  - concepts/installation
---
# Getting Started

> [!summary]
> Learn how to install and configure the product in under 5 minutes.

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
- `backlinks` - Array of concepts that link to this one (auto-generated)

### Index Format

The root `index.md` provides summary-first navigation:

```markdown
---
okf_version: "0.1"
total_concepts: 47
total_tokens: 125000
generated: 2024-01-15T10:30:00Z
---
# My Documentation Bundle

## Concepts (47)

- [[getting-started]] · Guide, quickstart, tutorial
  Learn how to install and configure the product in under 5 minutes.

- [[authentication]] · Guide, security, oauth
  Configure OAuth2 authentication with support for multiple providers.
```

### Summary Callouts

Each concept should have a summary callout after the title:

```markdown
> [!summary]
> A 1-2 sentence summary (max 200 characters) for navigation.
```

Summaries are auto-generated during `crawl` and `import` from:
1. Meta description (if present in source)
2. First meaningful paragraph
3. Document title (fallback)

## Scale Ceiling & RAG Graduation

The filing cabinet pattern works well for documentation sets up to ~100 concepts or ~400K tokens. Beyond this, query cost grows non-linearly because summary navigation produces too many candidates.

**Check your bundle's scale:**
```bash
okf-cli inspect ./my-bundle
```

**When the ceiling is exceeded:**
```bash
okf-cli inspect ./my-bundle --recommendations
```

This outputs guidance on adding vector search (RAG) alongside the wiki structure:
- Use header-based chunking (not token-count chunking)
- Recommended local vector stores: DuckDB with vss extension, ChromaDB
- Keep the wiki structure for synthesis questions; use vectors for precision lookups

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
