# Design Index Generator Specification

This document defines the **complete, authoritative behavior** of the design index generator implemented in [design/gen/main.go].

## Overview

The generator produces:

- A **category** index at `design/<category>/README.md` for each discovered category.
- A **category JSON** index at `design/<category>/index.json` for each discovered category.
- A **hub JSON** index at `design/index.json` aggregating all category documents.
- An **archived tasks JSON** index at `_dev/tasks/index.json` for `_dev/tasks/*.ja.md`.

Categories are discovered from templates under `design/gen/<category>/README.md`.

## How to run

The generator is invoked by `make gen-index` and can also be run directly:

```bash
go run ./design/gen -design-dir design
```

- `-design-dir` defaults to `.` when omitted.
- The generator exits non-zero on any error.

## Category discovery

The generator discovers categories by scanning the `design/gen/` directory:

- It lists immediate subdirectories of `design/gen/`.
- For each subdirectory `<category>`, if `design/gen/<category>/README.md` exists, it is treated as a category template.
- If **no** category templates are found, the generator fails.

To add a new category:

1. Create `design/gen/<category>/README.md` (template).
2. Create and maintain docs under `design/<category>/`.

The generator will:

- Ensure `design/<category>/` exists.
- Generate `design/<category>/README.md`.
- Link the category from the hub `design/README.md`.

## Inputs: documents collected per category

For each category `<category>`, the generator recursively scans `design/<category>/`.

In addition to category scanning, the generator also scans archived maintainer tasks under `_dev/tasks/` and collects only `*.ja.md` files with front matter for `_dev/tasks/index.json`.

### Included files

A file is considered for indexing only when **all** of the following are true:

- It is a regular file with the `.md` extension (case-insensitive).
- It contains YAML front matter delimited by:
  - the first line exactly `---`
  - and a later line exactly `---` (closing delimiter)

If a markdown file has no front matter, it is ignored (not indexed).

### Excluded files

The following are excluded by filename (case-insensitive):

- `README.md`, and anything starting with `readme.` (e.g., `README.ja.md`)
- anything starting with `index.`
- `GUIDE.md`, and anything starting with `guide.` (e.g., `GUIDE.ja.md`)

Additionally, a document is excluded if its front matter has:

- `version: meta` (case-insensitive)

### Front matter fields

Front matter is parsed as YAML and mapped into a `Doc` record with the following fields:

- `id` (string)
- `title` (string)
- `version` (string)
- `status` (string)
- `updated` (string)
- `language` (string)

If parsing fails (invalid YAML, missing closing delimiter, etc.), generation fails.

### Defaulting behavior

After parsing front matter:

- Markdown template field `RelPath` is set to a path relative to each category directory, prefixed with `./`.
  - Example in `design/tasks/README.md`: `./2026/02/14/20260214a-new-design-docs.ja.md`
- JSON field `relPath` is set to a repository-root-relative path, normalized to forward slashes, without `./` prefix.
  - Example: `design/tasks/2026/02/14/20260214a-new-design-docs.ja.md`
- If `id` is empty, it defaults to the filename stem (without extension).
- If `title` is empty, it defaults to a title derived from the filename stem with `-` replaced by spaces.
- `updated` is normalized:
  - If empty: `-`
  - If parseable as RFC3339 or `YYYY-MM-DD` / `YYYY/M/D` variants: normalized to RFC3339 (`YYYY-MM-DDTHH:MM:SSZ`)
  - Otherwise kept as-is

Additionally, all front matter fields are preserved and emitted to JSON by merging into each `docs[]` object (including fields beyond the standard set).

## Sorting rules

### ADR category

If the category name is `adr` (case-insensitive), documents are sorted by ADR number ascending.

ADR number extraction:

1. Prefer front-matter `id`.
2. Fallback to filename stem.

The generator looks for the last occurrence of `ADR-` and parses the digits that follow.

- If no ADR number can be extracted, the document sorts last.
- Ties are broken by `RelPath`.

### Other categories

All other categories are sorted lexicographically by `RelPath`.

## Outputs

### Category index: `design/<category>/README.md`

For each discovered category template `design/gen/<category>/README.md`, the generator renders the template to:

- `design/<category>/README.md`

Template data is:

- `Title` (string): `Kompox Design <CATEGORY> Index` (CATEGORY is upper-cased)
- `Category` (string): the category name
- `Updated` (string): latest `updated` value in docs (RFC3339)
- `Docs` ([]Doc): collected and sorted documents

Each `Doc` contains the fields described above, including `RelPath` for linking.

### Category JSON index: `design/<category>/index.json`

For each discovered category template `design/gen/<category>/README.md`, the generator also writes:

- `design/<category>/index.json`

JSON payload shape:

- `category` (string)
- `updated` (string, latest `updated` in docs)
- `docCount` (number)
- `docs` ([]object)

Each `docs[]` object is a single merged map and contains at least:

- `id`
- `title`
- `version`
- `status`
- `updated`
- `language`
- `category`
- `relPath`
- `references` (reference-style link labels extracted from markdown definitions)

Any additional front matter keys are also included in the same object (no separate `frontMatter` field).

Ordering:

- Follows category markdown ordering rules (ADR number sort for `adr`; `RelPath` sort for others).

### Hub JSON index: `design/index.json`

The generator writes an aggregated JSON index at:

- `design/index.json`

JSON payload shape:

- `updated` (string)
- `docCount` (number)
- `categories` ([]category summary)
  - `category`
  - `updated`
  - `docCount`
  - `indexPath` (`design/<category>/index.json`)
- `docs` (flattened docs across categories)

`docs[].relPath` is repository-root-relative (for example `design/tasks/2026/02/14/20260214a-new-design-docs.ja.md`).

`design/index.json` includes both `design/*` category docs and archived `_dev/tasks/*.ja.md` docs.

### Archived tasks JSON index: `_dev/tasks/index.json`

The generator writes an archive JSON index at:

- `_dev/tasks/index.json`

Collection rules for archive tasks:

- Scan `_dev/tasks/` recursively.
- Include only `*.ja.md` files.
- Exclude `README.ja.md`, `GUIDE.ja.md`, and `index.*`.
- Require front matter; files without front matter are ignored.

JSON payload shape is the same as category JSON (`category`, `updated`, `docCount`, `docs`) with `category: "old-tasks"`.

`docs[].relPath` is repository-root-relative (for example `_dev/tasks/2025-10-23-protection.ja.md`).

### Hub markdown index: `design/README.md`

`design/README.md` is **static** and is not generated by this tool.

Keep it as a hub that links to:

- `design/gen/README.md` (this specification)
- each category index under `design/<category>/README.md`

## Language policy

- The generator produces category markdown index `README.md` and JSON indexes (`index.json`).
- It does **not** generate `README.ja.md`.

If `README.ja.md` exists in the repository (e.g., from older revisions), it should be removed to satisfy the “not generated and not present” policy.

## Template authoring notes

- Templates are Go `text/template` files.
- They are read from `design/gen/<category>/README.md`.
A template can safely link to a doc using `{{ .RelPath }}` for a path relative to the category directory.

## References

- Implementation: [design/gen/main.go]
- Hub output: [design/README.md]
- Category templates:
  - [design/gen/adr/README.md]
  - [design/gen/plans/README.md]
  - [design/gen/tasks/README.md]
  - [design/gen/v1/README.md]
  - [design/gen/v2/README.md]
  - [design/gen/pub/README.md]

[design/gen/main.go]: ./main.go
[design/README.md]: ../README.md
[design/gen/adr/README.md]: ./adr/README.md
[design/gen/plans/README.md]: ./plans/README.md
[design/gen/tasks/README.md]: ./tasks/README.md
[design/gen/v1/README.md]: ./v1/README.md
[design/gen/v2/README.md]: ./v2/README.md
[design/gen/pub/README.md]: ./pub/README.md
