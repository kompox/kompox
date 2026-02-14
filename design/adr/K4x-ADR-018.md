---
id: K4x-ADR-018
title: Establish doc governance and dual index artifacts (Markdown + JSON)
status: accepted
updated: 2026-02-14T18:23:37Z
language: en
supersedes: []
supersededBy: []
---
# K4x-ADR-018: Establish doc governance and dual index artifacts (Markdown + JSON)

## Context

Kompox design documents are already organized by document type and indexed via generated category `README.md` files.

The current generator behavior is specified in [design/gen/README.md] and focuses on human-readable Markdown indexes. This has improved discoverability for maintainers.

However, AI coding agents and other automation workflows need a machine-readable inventory of document metadata (doc-id, status, updated, language, type, path, references) to:
- prioritize source-of-truth documents,
- detect stale docs (`out-of-sync`, stale `updated`),
- build reliable dependency graphs across ADR/Plan/Task/Spec docs,
- reduce ambiguity and redundant full-text scanning.

Without a canonical JSON index, each tool must parse many markdown files independently, causing inconsistent behavior and higher implementation cost.

## Decision

- Keep front matter as the single source of truth for document metadata.
- Continue generating human-readable Markdown indexes (`design/<category>/README.md`) as today.
- Add machine-readable JSON index artifacts generated from the same input during `make gen-index`.
- Define a minimal, stable JSON schema for indexed docs and category summaries.
- Treat JSON artifacts as generated files (not hand-edited), and keep their generation deterministic.

### Output model

- Category-level JSON index:
  - `design/<category>/index.json`
- Hub-level aggregated JSON index:
  - `design/index.json`
- Archived tasks JSON index:
  - `_dev/tasks/index.json`

The first two outputs are generated from front matter collected under `design/<category>/` with the same inclusion/exclusion rules used by the Markdown index generator.
The archived tasks output is generated from `_dev/tasks/*.ja.md`.

### Baseline JSON fields

Each indexed document should contain, at minimum:
- `id`
- `title`
- `status`
- `updated`
- `language`
- `version`
- `category`
- `relPath`
- `references` (doc-id-oriented references when resolvable)

Optional fields may be added in a backward-compatible way.

In addition, each JSON index should contain top-level:
- `docCount`

For archived tasks, document `category` is `old-tasks`.

## Alternatives Considered

- Keep Markdown-only indexes
  - Rejected: good for humans, but suboptimal for automation and AI-assisted workflows.
- Replace Markdown indexes with JSON-only output
  - Rejected: degrades maintainability and review ergonomics for humans.
- Store metadata in a separate manually-maintained JSON source
  - Rejected: introduces dual source-of-truth risk and drift.

## Consequences

- Pros
  - Improves AI agent and tooling performance for discovery, ranking, and impact analysis.
  - Preserves human-readable documentation workflows while adding machine-readability.
  - Avoids metadata drift by deriving all artifacts from front matter.
- Cons/Constraints
  - Generator complexity increases and requires schema/version discipline.
  - CI checks should be introduced/updated to ensure generated artifacts are current.
  - Consumers must tolerate additive JSON fields over time.

## Rollout

- Step 1: Document governance rules in generator/spec docs (source-of-truth and artifact boundaries).
- Step 2: Extend `design/gen` implementation to emit category and hub JSON indexes.
- Step 3: Add/adjust CI checks to fail when generated artifacts are stale.
- Step 4: Update maintainer guidance (`AGENTS.md` and relevant guides) to include JSON artifact usage.
- Step 5: Adopt JSON index consumption in automation and AI-agent workflows.

## References

- [design/README.md]
- [design/gen/README.md]
- [design/adr/GUIDE.md]
- [20260214a-new-design-docs]

[design/README.md]: ../README.md
[design/gen/README.md]: ../gen/README.md
[design/adr/GUIDE.md]: ./GUIDE.md
[20260214a-new-design-docs]: ../tasks/2026/02/14/20260214a-new-design-docs.ja.md
