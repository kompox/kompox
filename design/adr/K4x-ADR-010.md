---
id: K4x-ADR-010
title: Rename CRD to KOM
status: accepted
date: 2025-10-17
language: en
supersedes: []
supersededBy: []
---

## Context

- Our portable format is Kompox Ops Manifest (KOM), not Kubernetes CRDs themselves. The term "CRD mode" and flags like `--crd-path`/`--crd-app` can mislead users.
- Unifying naming around KOM improves conceptual clarity and aligns with documentation.
- Defaults pseudo-resource for ingestion has been accepted separately; terminology should follow the KOM naming.

## Decision

- Rename the concept "CRD mode" to "KOM mode" in CLI and documentation.
- Introduce new CLI flags and env vars only (no aliases):
  - `--kom-path` (repeatable), env: `KOMPOX_KOM_PATH`
  - `--kom-app` (single), env: `KOMPOX_KOM_APP`
- No backward compatibility:
  - Remove `--crd-path`/`--crd-app` flags and `KOMPOX_CRD_*` environment variables.
  - If legacy flags/envs are provided, the CLI fails fast with a clear error suggesting `--kom-*` / `KOMPOX_KOM_*`.
- Documentation:
  - Replace narrative uses of “CRD mode” with “KOM mode” and use `--kom-*` in all examples.

## Alternatives Considered

- Keep "CRD mode" naming: rejected due to conceptual mismatch with KOM and potential user confusion.
- Deprecation with aliases (`--crd-*` kept temporarily): rejected. It increases surface/complexity and prolongs confusion; a single, clean naming is preferred.

## Consequences

- Pros
  - Clearer terminology and stronger KOM branding.
  - Reduced confusion between Kubernetes CRDs and Kompox portable manifests.
- Cons
  - Temporary duplication of flags and environment variables during the migration period.

## Rollout

- Implement `--kom-*` flags and `KOMPOX_KOM_*` env vars; remove `--crd-*` and `KOMPOX_CRD_*`.
- Add validation: using legacy flags/envs results in a clear error message with migration hint.
- Update docs and examples to use “KOM mode” and `--kom-*` exclusively.
- Note the breaking change in release notes and changelog.

## References

- [Kompox-KOM.ja.md]
- [Kompox-CRD.ja.md]
- [_dev/tasks/2025-10-17-defaults.ja.md]

[Kompox-KOM.ja.md]: ../v1/Kompox-KOM.ja.md
[Kompox-CRD.ja.md]: ../v1/Kompox-CRD.ja.md
[_dev/tasks/2025-10-17-defaults.ja.md]: ../../_dev/tasks/2025-10-17-defaults.ja.md
