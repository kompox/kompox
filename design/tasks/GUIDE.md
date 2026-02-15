# Maintainer Task Guide (design/tasks)

Japanese: [GUIDE.ja.md]

This folder contains short task documents that help maintainers implement changes (implementation plan, acceptance criteria, test notes, and similar). It is not user-facing documentation.

## What belongs here

- Concrete implementation plans (about 1-2 pages)
- Acceptance criteria and test plans tied to a single deliverable
- Links to ADRs and specs as needed

## What does not belong here

- Formal design specifications (for example, design/v1 and design/v2)
- User documentation (docs/ and MkDocs)
- Long-term roadmaps (keep them elsewhere under design/)

## File naming

- Format: design/tasks/YYYY/MM/DD/YYYYMMDDa-short-description.<lang>.md
  - `<lang>` is `ja` or `en`
  - Example: design/tasks/2026/02/14/20260214a-new-design-docs.ja.md
- Keep short-description as short as practical.

## Workflow

1) Create a new task file using the template below
2) Update it during work (status, updated, checklist, progress)
3) When finished, set status: done and keep the file for history
   - If replaced, set status: superseded and add a link to the successor task

Tip: Record decisions in ADRs, and link from the task to ADR/spec to avoid duplication.

Timestamp command (for `updated` and `Progress`)
- Generate timestamps each time via shell command (as listed in AGENTS.md).
- Bash: `date -u +"%Y-%m-%dT%H:%M:%SZ"`
- PowerShell: `(Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ss'Z'")`

Progress section writing rules
- Use bullet items with a UTC ISO 8601 timestamp prefix: `YYYY-MM-DDTHH:MM:SSZ`.
- For single-line entries, write `TIMESTAMP` + space + message (no colon after timestamp).
- Generate each progress timestamp each time you add an entry, using the commands in "Timestamp command (for `updated` and `Progress`)".
- As a rule, do not edit past entries; append new entries only.

References section writing rules
- This rule applies only to markdown documents that have front matter `id:`.
- `README.md`, `README.ja.md`, `GUIDE.md`, and `GUIDE.ja.md` are out of scope.

## Front matter (YAML) schema

Required
- id (string): Unique task id (recommended: YYYYMMDDa-short-description)
- title (string): Short title
- status (enum): See Status values below
- updated (timestamp): UTC timestamp in ISO 8601 `YYYY-MM-DDTHH:MM:SSZ`
  - Generate `updated` each time you modify it, using the commands in "Timestamp command (for `updated` and `Progress`)".
- language (enum): ja | en
- adrs (string[]): Referenced ADR doc-ids for this task (for example, `K4x-ADR-018`)

Optional
- owner (string): GitHub handle or name
- plans (string[]): Referenced plan doc-ids for this task (for example, `2026aa-kompox-box-update`)
- supersedes (string|string[]): Task id(s) this task replaces
- supersededBy (string): Task id that replaces this task

Cross-reference rule
- In task docs, list referenced ADR doc-ids in `adrs`.
- In task docs, list referenced plan doc-ids in `plans`.
- Use doc-id values (no filename extension).

Status values (recommended)
- draft: Newly created or being edited
- active: In progress
- blocked: Waiting on a dependency
- done: Completed (kept for history)
- canceled: Stopped intentionally (kept for history)
- superseded: Replaced by a newer task (kept for history)

## Template (copy and adjust)

```markdown
---
id: YYYYMMDDa-short-description
title: Short title
status: draft
updated: YYYY-MM-DDTHH:MM:SSZ
language: en
owner:
adrs: []
plans: []
supersedes: []
supersededBy:
---
# Task: <Short title>

## Goal

- ...

## Scope / Out of scope

- In: ...
- Out: ...

## Spec summary

- ... (link to ADR/spec for details)

## Plan (checklist)

- [ ] Step 1
- [ ] Step 2

## Tests

- Unit: ...
- Smoke: ...

## Acceptance criteria

- ...

## Notes

- Risks: ...
- Follow-ups: ...

## Progress

- YYYY-MM-DDTHH:MM:SSZ Task file created

## References

- [design/tasks/README.md]

[design/tasks/README.md]: ./README.md
```

## Indexing

- Regenerate design/tasks/README.md using the Makefile target:
- make gen-index

## References

- [GUIDE.ja.md]

[GUIDE.ja.md]: ./GUIDE.ja.md
