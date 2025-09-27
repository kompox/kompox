# Maintainer Tasks Guide (`_dev/tasks`)

This folder hosts short, action-oriented task documents for maintainers (implementation plans, acceptance criteria, and test notes). These files are not user-facing docs and are excluded from Go packages because the directory name starts with an underscore.

## What belongs here
- Implementation plans for concrete changes (1–2 pages each)
- Acceptance criteria and test plans tied to a single deliverable
- Links to design decisions (ADR) and final specs

What does not belong here
- Canonical design specs (put them under `design/`)
- User documentation (lives under `docs/` for MkDocs)
- Long-running roadmaps (use `design/` or separate planning docs)

## File naming
- Pattern: `YYYY-MM-DD-topic.lang.md`
  - Examples: `2025-09-27-disk-snapshot-unify.ja.md`, `2025-10-05-cli-refactor.en.md`
- Keep the topic short and descriptive; include a language suffix (`.ja.md` or `.en.md`).

## Workflow (lightweight)
1) Create a new task file using the template below
2) Keep it up to date while working (status, updated date, checklist)
3) When done, set `status: done` and leave the file in place (for history)
   - If superseded, set `status: superseded` and add a link to the new task

Tip: Decisions go to ADRs. Tasks should link to ADRs and specs rather than duplicating them.

## Front matter schema (YAML)
Required
- `id` (string): unique task identifier (suggestion: `YYYY-MM-topic`)
- `title` (string): short task title
- `status` (enum): `active | blocked | done | canceled | superseded`
- `updated` (date): ISO format `YYYY-MM-DD`
- `language` (enum): `ja | en`

Recommended
- `owner` (string): GitHub handle or name
- `references` (string[] or object[]): related ADRs/specs/PRs

Optional
- `category` (string): e.g., `cli`, `usecase`, `driver`, `docs`
- `priority` (enum): `P0 | P1 | P2 | P3`
- `risk` (string): brief risk note
- `tags` (string[]): free-form tags
- `related` (string[]): related task ids
- `started` (date): start date
- `due` (date): target date

Example
```yaml
---
id: 2025-09-disk-snapshot-unify
title: Disk/Snapshot unification (disk create -S)
status: active
owner: your-handle
updated: 2025-09-27
language: ja
references:
  - design/adr/K4x-ADR-002.md
  - design/v1/Kompox-CLI.ja.md
category: usecase
priority: P1
risk: Driver IF change; region/RBAC constraints
---
```

## Suggested content structure
- Goal: why this task exists and what success looks like
- Scope / Out of scope: keep it tight and testable
- Spec summary: the minimal spec changes (link to ADR/spec for details)
- Plan (checklist): small steps that can be checked off
- Tests: unit/integration/smoke and what they validate
- Acceptance criteria: concise, verifiable outcomes
- Notes: risks, follow-ups, migration reminders

## Template (copy and adapt)
```markdown
---
id: YYYY-MM-topic
title: Short task title
status: active
owner: 
updated: YYYY-MM-DD
language: en
references:
  - design/adr/K4x-ADR-00X.md
  - design/v1/<Spec>.md
category: 
priority: P2
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
```

## Indexing
- Tasks are auto-indexed into `README.md` (and `README.ja.md`) via the Makefile target:
- `make gen-index`
