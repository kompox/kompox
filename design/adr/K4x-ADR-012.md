---
id: K4x-ADR-012
title: Introduce App.RefBase for external references
status: accepted
updated: 2025-10-18
language: en
supersedes: []
supersededBy: []
---
# K4x-ADR-012: Introduce App.RefBase for external references

## Context

- Whether an App may reference the local filesystem depends on where the App was loaded from. Apps defined directly in the Kompox app file may use local files; Apps coming from external KOM sources must not.
- Today, file resolution is split across boundaries:
  - The CRD layer (v1alpha1) can expand `file:compose.yml` into inline content.
  - The Kubernetes adapter resolves Compose using compose-go and also reads other local references (env_file, configs/secrets).
  - An early string-scan check (`HasLocalFSReference`) is brittle and over-restrictive.
- This scattering makes policy enforcement inconsistent and hard to test. We need a single place to resolve and validate external references based on origin.

## Decision

- Introduce a new field on the domain model `App`: `RefBase` (string), the base location for resolving external references.
  - Semantics:
    - `""` (empty): external references are not allowed for this App.
    - `file:///abs/dir/`: allow and resolve local file references relative to this directory.
    - `http(s)://host/path/`: reserved for future URL-based KOM origins.
- Validation timing and user-facing behavior:
  - Loading KOM documents (KOM mode initialization) succeeds even if an App would later violate external reference policies. No validation error is emitted at load time.
  - Validation is performed during conversion/execution paths (e.g., kube Converter invoked by `kompoxops app ...` commands). At this point, origin-based policy (via `App.RefBase`) is enforced and violations are reported to the user as command errors.
  - Rationale: keep loaders side-effect-free and centralized enforcement at conversion time for consistent, testable behavior.
- Stop resolving `file:compose.yml` in v1alpha1. Instead, v1alpha1 only sets `App.RefBase` based on origin:
  - App in Kompox app file → `RefBase = file://<that file's directory>/`.
  - App from external KOM (e.g., Defaults.komPath) → `RefBase = ""` (disallowed).
  - Future: if KOM is loaded from a URL, set `RefBase` to that URL's directory.
- Centralize all reference resolution and policy checks in kube Converter:
  - Build the Compose project from inline text, `file:` path, or `http(s)` URL using `RefBase`.
  - Resolve env_file, configs/secrets (file sources) relative to `RefBase` when it is `file://`; otherwise reject.
  - Reject host bind mounts when `RefBase` is empty or `http(s)` (named volumes remain allowed).
- Deprecate the string-scan helper `HasLocalFSReference` in favor of structured validation after Compose is parsed.

## Alternatives Considered

- Keep resolving `file:` in v1alpha1 and only validate in kube:
  - Pros: smaller change in kube.
  - Cons: responsibility split remains; difficult to enforce origin-based rules consistently.
- Use a boolean flag like `AllowLocalFS` instead of a base location:
  - Pros: simple gate.
  - Cons: cannot support URL-based origins or relative path/URL joining; less future-proof.
- Other names for the field (e.g., `RefBaseURL`, `LocalFSBase`, `FileRefBaseDir`):
  - Chosen `RefBase` for clarity and neutrality across schemes (file and http[s]).

## Consequences

- Single policy switch (`RefBase`) governs all external reference behavior, improving clarity and testability.
- v1alpha1 becomes simpler: no file IO for Compose; it only sets origin metadata.
- kube Converter becomes the single integration point for resolving and validating Compose, env_file, configs/secrets, and volume policies.
- kompoxopscfg converter must also stop expanding `file:compose.yml` and set `App.RefBase` appropriately when loading from kompoxops config sources to keep behavior consistent across loaders.
- Backwards compatibility: Not required for this change.
  - The domain model gains a new field (`RefBase`). Persistence adapters may be updated as needed.
  - Apps from external KOM will now fail fast when using local references; this is an intentional tightening.
- Security posture improves by preventing unintended local file access outside allowed origins.

## Rollout

- Step 1: Add `App.RefBase` to the domain model. [domain model App]
- Step 2: Update v1alpha1 sink to stop expanding `file:compose.yml` and set `RefBase` per origin (Kompox app → file:// dir, external KOM → empty). [v1alpha1 loader and sink]
- Step 3: Update kompoxopscfg converter to stop expanding `file:compose.yml` and set `RefBase` per origin as well. [kompoxopscfg converter]
- Step 4: In kube, introduce helpers to load Compose and related artifacts using `RefBase`; validate volumes/env_file/configs/secrets accordingly. [kube compose loader] [kube converter]
- Step 5: Deprecate `HasLocalFSReference` and remove its call sites in CLI loader; rely on structured validation. [v1alpha1 validation] [CLI loader]
- Step 6: Extend and adjust tests (Compose parsing, env_file, configs/secrets, volumes) to cover both allowed and disallowed origins. [tests]
- Step 7: Update specs/tasks to reflect the boundary shift and the new field. [Kompox-CLI.ja.md] [2025-10-17-defaults.ja.md]

## References

- [Kompox-CLI.ja.md]
- [2025-10-17-defaults.ja.md]
- [2025-10-18-refbase.ja.md]
- [v1alpha1 loader and sink]
- [kube compose loader]
- [kube converter]
- [domain model App]
- [kompoxopscfg converter]
- [v1alpha1 validation]
- [CLI loader]
- [tests]

[Kompox-CLI.ja.md]: ../v1/Kompox-CLI.ja.md
[2025-10-17-defaults.ja.md]: ../../_dev/tasks/2025-10-17-defaults.ja.md
[2025-10-18-refbase.ja.md]: ../../_dev/tasks/2025-10-18-refbase.ja.md
[v1alpha1 loader and sink]: ../../config/crd/ops/v1alpha1/sink_tomodels.go
[kube compose loader]: ../../adapters/kube/compose.go
[kube converter]: ../../adapters/kube/converter.go
[domain model App]: ../../domain/model/app.go
[kompoxopscfg converter]: ../../config/kompoxopscfg/converter.go
[v1alpha1 validation]: ../../config/crd/ops/v1alpha1/app_validation.go
[CLI loader]: ../../cmd/kompoxops/kom_loader.go
[tests]: ../../tests/