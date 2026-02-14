# Design Docs (v1) Guide

Japanese: [GUIDE.ja.md]

This directory contains design documents for Kompox v1 (the current architecture/spec set). These docs are longer-lived than tasks and should remain useful as a reference even after implementation.

## Scope and purpose

Design docs (v1) should:
- Describe the intended behavior and contracts (CLI UX, config formats, domain models, driver interfaces)
- Capture non-obvious constraints and trade-offs
- Provide enough detail for implementation and review
- Link to ADRs and tasks rather than duplicating decision history or execution checklists

Design docs (v1) should avoid:
- Becoming a log of daily progress (use tasks)
- Duplicating large code listings or vendor docs

## Language policy

Design docs (v1) may be written in Japanese or English.

- Japanese docs use the filename suffix `.ja.md` and `language: ja`.
- English docs use `.md` (no language suffix) and `language: en`.

## File naming and IDs

- Directory: `design/v1/`
- Filename (and id): `Kompox-<short-slug>.<lang>.md`
  - `<short-slug>`: short PascalCase or kebab-case slug (match existing docs)
  - `<lang>`: `.ja.md` for Japanese, `.md` for English

Rule: `id` in the YAML front matter must equal the filename stem (filename without extension).

## Front matter (YAML) schema

Required
- id (string): Unique doc id (must match filename stem)
- title (string): Document title
- version (string): Use `v1`
- status (enum): `draft | synced | out-of-sync | archived`
- updated (timestamp): UTC timestamp in ISO 8601 `YYYY-MM-DDTHH:MM:SSZ`
- language (enum): `ja | en`

## Status guidance

- draft: Under discussion or not yet implemented
- synced: Implementation exists and this document reflects it correctly
- out-of-sync: Implementation exists but this document needs updates
- archived: Kept for historical reference; no longer maintained

## Authoring guidelines

- Prefer stable contracts, invariants, and examples over internal implementation notes
- Keep it skimmable: use headings and tables for key interfaces and inputs/outputs
- If a section is driven by a decision, link to the ADR (do not restate the ADR)
- This rule applies only to markdown documents that have front matter `id:`.
- `README.md`, `README.ja.md`, `GUIDE.md`, and `GUIDE.ja.md` are out of scope.

## Template

```markdown
---
id: Kompox-ShortSlug
title: Short title
version: v1
status: draft
updated: 2026-02-14T00:00:00Z
language: en
---

# Short title

## Overview

- ...

## Scope / Out of scope

- In: ...
- Out: ...

## Design

- ...

## Interfaces / Contracts

- ...

## Constraints

- ...

## Migration notes

- ...

## References

- [design/v1/README.md]

[design/v1/README.md]: ./README.md
```

## Indexing

- [README.md] is generated. Regenerate it via:
- `make gen-index`

## References

- [GUIDE.ja.md]
- [README.md]

[GUIDE.ja.md]: ./GUIDE.ja.md
[README.md]: ./README.md
