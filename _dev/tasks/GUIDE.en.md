# Maintainer Tasks Guide (`_dev/tasks`)

This folder hosts short, action-oriented task documents for maintainers (implementation plans, acceptance criteria, and test notes). These files are not user-facing docs and are excluded from Go packages because the directory name starts with an underscore.

## What belongs here

- Implementation plans for concrete changes (1–2 pages each)
- Acceptance criteria and test plans tied to a single deliverable
- Links to design decisions (ADR) and final specs

## What does not belong here

- Canonical design specs (put them under `design/`)
- User documentation (lives under `docs/` for MkDocs)
- Long-running roadmaps (use `design/` or separate planning docs)

## File naming

- Pattern: `YYYY-MM-DD-topic.lang.md`
  - Examples: `2025-09-27-disk-snapshot-unify.ja.md`, `2025-10-05-cli-refactor.en.md`
- Keep the topic as short as possible (for easy referencing); include a language suffix (`.ja.md` or `.en.md`).

## Workflow (lightweight)

1) Create a new task file using the template below
2) Keep it up to date while working (status, updated date, checklist)
3) When done, set `status: done` and leave the file in place (for history)
   - If superseded, set `status: superseded` and add a link to the new task

Tip: Decisions go to ADRs. Tasks should link to ADRs and specs rather than duplicating them.

## Front matter schema (YAML)

Required
- `id` (string): unique task identifier (suggestion: `YYYY-MM-DD-topic`)
- `title` (string): short task title
- `status` (enum): see Status values below
- `updated` (date): ISO format `YYYY-MM-DD`
- `language` (enum): `ja | en`

Optional
- `owner` (string): GitHub handle or name
- `supersedes` (string|string[]): previous task id(s) this task replaces
- `supersededBy` (string): next task id that replaces this task

Status values
- `draft`: created and under planning/editing
- `active`: in progress
- `done`: completed and kept for history
- `canceled`: intentionally stopped and kept for history

Note: If you want to express lineage without adding more statuses, prefer `supersedes`/`supersededBy`. If you need a waiting state, consider adding `blocked` or `on-hold` as an extension, but keep the core set minimal.

Example
```yaml
---
id: 2025-09-27-disk-snapshot-unify
title: Disk/Snapshot unification (disk create -S)
status: draft
updated: 2025-09-27
language: ja
owner: your-handle
---
```

## Suggested content structure

- Goal: why this task exists and what success looks like
- Scope / Out of scope: keep it tight and testable
- Spec summary: the minimal spec changes (link to ADR/spec for details)
- Plan (checklist): small steps that can be checked off
- Progress: short notes (date + brief outcome)
- Tests: unit/integration/smoke and what they validate
- Acceptance criteria: concise, verifiable outcomes
- Notes: risks, follow-ups, migration reminders

## Template (copy and adapt)

```markdown
---
id: YYYY-MM-DD-topic
title: Short task title
status: draft
updated: YYYY-MM-DD
language: en
---
# Task: <Short title>

## Goal

- ...

## Scope / Out of scope

- In: ...
- Out: ...

## Spec summary

- ... (link to ADR/spec)

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

- YYYY-MM-DD: Brief progress note (e.g., implemented steps 1–3 in PR #<num>)

## References

- design/adr/K4x-ADR-00X.md
- design/v1/<Spec>.md
```

## Indexing

- Tasks are auto-indexed into `README.md` (and `README.ja.md`) via the Makefile target:
- `make gen-index`
