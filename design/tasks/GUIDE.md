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

## Lightweight workflow

1) Create a new task file using the template below
2) Update it during work (status, updated, checklist)
3) When finished, set status: done and keep the file for history
   - If replaced, set status: superseded and add a link to the successor task

Tip: Record decisions in ADRs, and link from the task to ADR/spec to avoid duplication.

## Front matter (YAML) schema

Required
- id (string): Unique task id (recommended: YYYYMMDDa-short-description)
- title (string): Short title
- status (enum): See Status values below
- updated (timestamp): UTC timestamp in ISO 8601 `YYYY-MM-DDTHH:MM:SSZ`
- language (enum): ja | en

Optional
- owner (string): GitHub handle or name
- supersedes (string|string[]): Task id(s) this task replaces
- supersededBy (string): Task id that replaces this task

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
updated: 2026-02-14T00:00:00Z
language: en
owner:
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

- YYYY-MM-DD: ...

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
