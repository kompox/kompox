---
id: K4x-ADR-003
title: Unify Disk/Snapshot CLI flags and adopt opaque Source contract
status: accepted
date: 2025-09-28
language: en
supersedes: []
supersededBy: []
---
# K4x-ADR-003: Unify Disk/Snapshot CLI flags and adopt opaque Source contract

## Context

- Disk/Snapshot commands used inconsistent flags and semantics (`-D/--disk-name`, `--snapshot-name`, and `-S` sometimes meaning target snapshot name).
- Provider drivers already own source resolution for creation flows; parsing or validating the `Source` at the CLI/UseCase layers couples us to provider details and duplicates logic.
- We want a consistent UX across disk/snapshot operations and clear defaults that reduce destructive mistakes.

## Decision

- Unify common flags across disk/snapshot:
  - `-A|--app-name` (app), `-V|--vol-name` (volume)
  - `-N|--name` (target/creation name)
- Subcommand behaviors:
  - `disk list`   → `-A -V`
  - `disk create` → `-A -V [-N name] [-S source]` (omit `-S` to create an empty disk)
  - `disk assign` → `-A -V -N`
  - `disk delete` → `-A -V -N`
  - `snapshot list`   → `-A -V`
  - `snapshot create` → `-A -V [-N name] [-S source]` (omit `-S` to use the currently Assigned disk)
  - `snapshot delete` → `-A -V -N`
- Naming flag aliases:
  - disk: `-N | --name | --disk-name`
  - snapshot: `-N | --name | --snap-name`
- Naming constraints (Kompox-wide baseline):
  - Volume name: DNS-1123 label, length 1..16
  - Disk name: DNS-1123 label, length 1..24
  - Snapshot name: DNS-1123 label, length 1..24
  - DNS-1123 label means lowercase alphanumeric and hyphen (-) only, must start and end with an alphanumeric character.
  - Note: Provider drivers may enforce stricter limits due to underlying platform constraints; such driver-specific validation occurs in addition to the baseline above.
- Source contract (`-S|--source`, create-only):
  - Format: `-S [<type>:]<name>`
    - Common vocabulary: `disk:<name>` references a Kompox-managed disk; `snapshot:<name>` references a Kompox-managed snapshot.
    - If the user omits the type: `disk create` → use `snapshot:<name>`, `snapshot create` → use `disk:<name>`.
    - If the user omits `-S` entirely: `disk create` → create an empty disk; `snapshot create` → resolve Assigned disk and use `disk:<name>`.
  - CLI/UseCase do not parse/validate/normalize, do treat it as opaque; drivers interpret the value.

## Rationale

- Separates “what to operate on” (`-N`) from “where to create from” (`-S`), reducing cognitive load and collisions.
- Aligns with K4x-ADR-002 where snapshot restore is integrated into `disk create -S`.
- Keeps provider-specific logic where it belongs (drivers), minimizing cross-layer coupling.

## Scope

- In scope: CLI flag set and help text, UseCase parameter pass-through, documentation updates (CLI spec), tests for pass-through and defaults.
- Out of scope: maintaining backward compatibility for legacy flags, provider driver heuristics beyond the reserved vocabulary.

## Alternatives Considered

- Keep mixed legacy flags (`-D`, `--snapshot-name`): rejected due to inconsistency and `-S` collision.
- Parse or normalize `Source` in CLI/UseCase: rejected to avoid provider coupling and duplicated validation.

## Consequences

- Breaking change: legacy flag forms are not preserved by design.
- CLI help/docs/tests must be updated to reflect `-N` and `-S` semantics and defaults.
- CLI/UseCase do not interpret `Source` at all; it is passed verbatim to drivers even when omitted (empty string).
- Provider drivers remain the source of truth for `Source` resolution and validation.

## Risks and Mitigations

- Ambiguity in source strings: keep CLI opaque; drivers document accepted formats and validation. Reserved prefixes reduce ambiguity.
- UX safety: defaults minimize destructive outcomes (empty disk by default; snapshot creation from the currently assigned disk by default).

## Rollout

1) Update CLI flags and semantics for disk/snapshot commands.
2) Update CLI spec docs and examples.
3) Add/adjust tests to cover pass-through and default behaviors.

## References

- K4x-ADR-002 (Unify snapshot restore into disk create)
- _dev/tasks/2025-09-28-disk-snapshot-cli-flags.ja.md
- design/v1/Kompox-CLI.ja.md