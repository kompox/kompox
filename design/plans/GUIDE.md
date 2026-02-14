# Plans (design/plans)

Japanese: [GUIDE.ja.md]

This directory contains plan documents used when making large changes that require updating multiple design docs. A plan helps coordinate the intended diffs across documents and the order of updates.

Plans remain in the repository after the work is completed as a reference.

Example:
- [2026aa-kompox-box-update]

## Scope and purpose

Plans should:
- Describe the goal and non-goals
- List the affected design docs and the intended diffs at a high level
- Describe the sequence for updating those docs (and any migration notes)
- Capture dependencies, risks, and open questions
- Link to ADRs/specs/tasks rather than duplicating full details

Plans should avoid:
- Becoming a step-by-step execution checklist (use task files for that)
- Copying entire specs or code-level details

## Language policy

Plans may be written in Japanese or English. Use the filename suffix to indicate language:
- `.ja.md`: Japanese
- `.en.md`: English

## File naming and IDs

- Directory layout: `design/plans/<year>/`
- Filename (and id): `YYYYaa-<short-slug>.<lang>.md`
	- `YYYY`: year
	- `aa`, `ab`, ...: disambiguator within the year
	- `<short-slug>`: short, kebab-case name

Rule: `id` in the YAML front matter must equal the filename stem (filename without extension).

## Front matter (YAML) schema

Required
- id (string): Unique plan id (must match filename stem)
- title (string): Plan title
- status (enum): `draft | active | done | canceled | superseded`
- updated (timestamp): UTC timestamp in ISO 8601 `YYYY-MM-DDTHH:MM:SSZ`
- language (enum): `ja | en`
- adrs (string[]): Referenced ADR doc-ids for this plan (for example, `K4x-ADR-018`)

Optional
- version (string): Version label (for example, `v1`)
- tasks (string[]): Task doc-ids implemented under this plan (for example, `20260214a-new-design-docs`)

Cross-reference rule
- In plan docs, list referenced ADR doc-ids in `adrs`.
- In plan docs, list task doc-ids in `tasks`.
- Use doc-id values (no filename extension).

## Status lifecycle

- draft: Under active authoring
- active: In use as the working plan
- done: Completed; kept for reference
- canceled: Stopped intentionally; kept for reference
- superseded: Replaced by a newer plan; kept for reference

## Authoring guidelines

- Make it easy to see what will change across docs
- Prefer explicit rules and concrete examples over implied behavior
- Call out backwards-compatibility and migration notes when behavior changes
- Keep the plan stable and readable after completion
- This rule applies only to markdown documents that have front matter `id:`.
- `README.md`, `README.ja.md`, `GUIDE.md`, and `GUIDE.ja.md` are out of scope.

## Template

```markdown
---
id: YYYYaa-short-slug
title: Short plan title
version: v1
status: draft
updated: 2026-02-14T00:00:00Z
language: en
adrs: []
tasks: []
---

# Plan: Short plan title

## Goal

- ...

## Non-goals

- ...

## Background

- ...

## Affected design docs

- In scope:
	- design/v1/<doc>.md: ...
	- design/v2/<doc>.md: ...
	- design/adr/<adr>.md: ...
- Out of scope:
	- ...

## Intended diffs (summary)

- Doc A: ...
- Doc B: ...

## Update sequence

- Step 1: ...
- Step 2: ...

## Risks and open questions

- ...

## Migration notes

- ...

## References

- [design/plans/README.md]

[design/plans/README.md]: ./README.md
```

## Indexing

- Regenerate [README.md] using the Makefile target:
- `make gen-index`

## References

- [GUIDE.ja.md]
- [README.md]
- [2026aa-kompox-box-update]

[GUIDE.ja.md]: ./GUIDE.ja.md
[README.md]: ./README.md
[2026aa-kompox-box-update]: ./2026/2026aa-kompox-box-update.ja.md
