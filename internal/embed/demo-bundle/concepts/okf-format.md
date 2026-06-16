---
type: Concept
title: Open Knowledge Format
description: The Open Knowledge Format (OKF) is a specification for organizing documentation into structured bundles.
tags:
  - okf
  - format
  - specification
---
# Open Knowledge Format

The Open Knowledge Format (OKF) is a specification for organizing documentation into structured bundles that AI agents can efficiently search and read.

## Bundle Structure

An OKF bundle is a directory containing:

- `index.md` - Root index with `okf_version` frontmatter
- Concept files (`.md`) - Individual documentation pages
- Subdirectories with their own `index.md` files

## Frontmatter

Each concept file starts with YAML frontmatter:

```yaml
---
type: Guide
title: My Document Title
description: A brief description
tags:
  - tag1
  - tag2
resource: https://original-source.com/page
---
```

## Required Fields

- `type` - The concept type (Guide, API Reference, Concept, etc.)

## Optional Fields

- `title` - Document title
- `description` - Brief description (max 180 chars)
- `tags` - Array of topic tags
- `resource` - Original source URL
- `timestamp` - Last modified date
