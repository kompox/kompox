---
id: K4x-ADR-002
title: Unify snapshot restore into disk create
status: accepted
updated: 2025-09-27
language: en
---
# K4x-ADR-002: Unify snapshot restore into disk create

## Context

- Disk creation flows were split across empty-disk creation, snapshot restore, and (potentially) external imports.
- We want a single, consistent UX and a clear import path from provider-native resources.

## Decision

- Remove the standalone `snapshot restore` command and integrate snapshot-based restore into `disk create` via `-S/--source`.
- Auto-detect the source string:
  - empty or not provided → create an empty disk
  - starts with `/subscriptions/` → treat as provider-native Resource ID (e.g., Azure Snapshot/Disk)
  - otherwise → treat as a Kompox-managed snapshot name and resolve to a provider Resource ID in the driver
- Implement in provider drivers by handling `Source` inside `VolumeDiskCreate` (Copy with `SourceResourceID` or Empty).

## Scope

- In scope: CLI UX change to unify creation flows, UseCase/Model option to pass `Source`, provider driver logic to detect and copy from snapshot/disk, documentation updates
- Out of scope: introducing a separate `disk import` command, maintaining backward compatibility for `snapshot restore` (no existing users), provider-specific enhancements beyond the common abstraction

## Alternatives Considered

- Keep the separate `snapshot restore` command (higher learning cost, duplicates functionality)
- Introduce a new `disk import` subcommand (adds redundancy, splits UX)

## Consequences

- Pros: simpler CLI, clear import route, easier future extensions (other clouds/archives)
- Cons/Constraints: driver IF cleanup (remove `SnapshotRestore`), provider constraints (region match, RBAC), potential ambiguity in `Source`

## Risks and Mitigations

- Region/zone compatibility (e.g., Azure requires same region): validate early and document constraints; surface actionable errors
- RBAC for external Resource IDs: require appropriate roles; document minimum permissions
- Ambiguity in `Source` string: start with auto-detection; consider adding an explicit `--source-kind` in the future to disambiguate

## Rollout

1) Extend `disk create` to support `-S` and verify behavior
2) Remove `snapshot restore` implementation, driver IF, CLI, and docs

## References

- _dev/tasks/2025-09-27-disk-snapshot-unify.ja.md
- design/v1/Kompox-CLI.ja.md
