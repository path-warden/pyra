# TypeScript Parity Notes

This document describes the behavioral parity between the Go implementation of OKFy and the original TypeScript implementation.

## Feature Parity

The Go implementation provides full feature parity with the TypeScript version:

| Feature | TypeScript | Go | Notes |
|---------|------------|-----|-------|
| crawl command | ✅ | ✅ | All options supported |
| import command | ✅ | ✅ | All options supported |
| validate command | ✅ | ✅ | JSON output supported |
| inspect command | ✅ | ✅ | Same statistics |
| serve command | ✅ | ✅ | MCP stdio transport |
| demo command | ✅ | ✅ | Embedded bundle |

## MCP Tools

All 6 MCP tools are implemented with identical schemas:

- `search_concepts` - Full-text search with fuzzy matching
- `read_concept` - Read concept with truncation support
- `get_neighbors` - Graph traversal with depth limit
- `list_types` - List all concept types
- `list_tags` - List all tags
- `bundle_summary` - Bundle statistics

## Known Differences

### HTML to Markdown Conversion

The Go implementation uses `JohannesKaufmann/html-to-markdown` while TypeScript uses `turndown`. Minor formatting differences may occur:

- Table formatting may vary slightly
- Code block language detection may differ
- Some edge cases in nested list handling

### Search Scoring

The Go implementation uses Bleve for search while TypeScript uses a custom implementation. Search result ordering may differ slightly for queries with similar relevance scores.

### Timestamp Format

Both implementations use RFC3339 format. The Go implementation uses `time.RFC3339` which produces slightly different formatting than JavaScript's `toISOString()`:

- Go: `2024-01-15T10:30:00Z`
- TypeScript: `2024-01-15T10:30:00.000Z` (includes milliseconds)

Use `--stable-timestamps` for reproducible builds in both implementations.

## Testing Verification

To verify parity between implementations:

1. **Import the same folder**:
   ```bash
   # TypeScript
   npx pyra import ./docs --out ./bundle-ts
   
   # Go
   pyra import ./docs --out ./bundle-go
   ```

2. **Compare bundle structure**:
   - Same number of concept files
   - Same directory structure
   - Same frontmatter fields (type, title, tags, etc.)

3. **Validate both bundles**:
   ```bash
   pyra validate ./bundle-ts
   pyra validate ./bundle-go
   ```

4. **Test MCP tools**:
   - Search for the same queries
   - Read the same concepts
   - Compare results

## Reporting Issues

If you find behavioral differences not documented here, please open an issue with:

1. Command used
2. Input data (or minimal reproduction)
3. Expected output (from TypeScript)
4. Actual output (from Go)
