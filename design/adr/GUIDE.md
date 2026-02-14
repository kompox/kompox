# Architecture Decision Records (ADR)

Japanese: [GUIDE.ja.md](./GUIDE.ja.md)

This directory contains Architecture Decision Records for Kompox. ADRs capture significant engineering decisions with their context, options, and consequences. They are concise and stable; detailed implementation plans live elsewhere (e.g., `_dev/tasks/`).

Language policy: ADRs must always be written in English. Japanese documentation may exist elsewhere (e.g., `GUIDE.ja.md`), but ADR files (`K4x-ADR-###.md`) are English-only.

## Scope and purpose

- Document decisions that affect public behavior, CLI UX, data contracts, provider/driver interfaces, or architecture boundaries
- Keep ADRs short (roughly 0.5-1 page). Link to specs and task docs rather than duplicating details

## File naming and structure

- Filename: `K4x-ADR-###.md` (numeric ID only). The title goes inside the document header/front matter
- One ADR per file; one decision per ADR
- Suggested header (front matter or top section):
  - id (format: `K4x-ADR-###`, same as filename stem)
  - title
  - status
  - date (ISO `YYYY-MM-DD`)

## Status lifecycle

The ADR group uses the following status values:
- proposed: Under discussion and not yet accepted
- accepted: Decided and in effect
- rejected: Decided not to implement
- deprecated: No longer recommended; kept for historical reference

Supersession is represented by dedicated headers (see below), not by a `superseded` status.

Guidance:
- Start with `proposed`. When the decision is made, change to `accepted`
- If the idea is discarded, mark `rejected`
- If an ADR is still true but no longer recommended, use `deprecated`

### Supersession headers

- `supersedes`: full ADR id(s) that this ADR replaces (e.g., `K4x-ADR-001` or `[K4x-ADR-001, K4x-ADR-007]`)
- `supersededBy`: full ADR id(s) that replace this ADR (e.g., `K4x-ADR-009` or `[K4x-ADR-009]`)

Notes:
- Keep status as `accepted` for both the old and new ADRs; use the headers to express the relationship
- Add reciprocal links when possible (e.g., `K4x-ADR-008` lists `supersededBy: K4x-ADR-010`; `K4x-ADR-010` lists `supersedes: K4x-ADR-008`)
- Use full ids in the headers (`K4x-ADR-###`) for clarity and grep-ability

## Authoring guidelines

- Focus on the why more than the how
- Capture alternatives considered and trade-offs succinctly
- Call out constraints: security, compliance, provider limitations, backwards-compatibility
- Provide links to related specs, tasks, and code

## Template

```markdown
---
id: K4x-ADR-<###>
title: <Short decision title>
status: proposed | accepted | rejected | deprecated
date: YYYY-MM-DD
supersedes: <K4x-ADR-### | [K4x-ADR-###, K4x-ADR-###]>
supersededBy: <K4x-ADR-### | [K4x-ADR-###, K4x-ADR-###]>
---

## Context

- ...

## Decision

- ...

## Alternatives Considered

- ...

## Consequences

- Pros: ...
- Cons/Constraints: ...

## Rollout

- ... (phased plan if applicable)

## References

- ... (links to tasks/specs/PRs)
```

## Index

- Keep [README.md](./README.md) up to date. ADRs are listed with their ID, title, updated date, status, and link.

## References

- [GUIDE.ja.md]
- [README.md]

[GUIDE.ja.md]: ./GUIDE.ja.md
[README.md]: ./README.md
