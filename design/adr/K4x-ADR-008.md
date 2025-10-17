---
id: K4x-ADR-008
title: Introduce Box as a first-class ops kind
status: proposed
date: 2025-10-13
language: en
supersedes: []
supersededBy: []
---

## Context

- In Kompox PaaS, the tenant boundary is the App (1 App = 1 Namespace), but operationally we need multiple deployable units under an App (equivalent to K8s Deployments).
- In v1 CLI, `kompoxops app` is the primary operational command, yet we also need to manipulate individual components within an App for investigation/debug/development.
- For a future v2 Operator, we want a stable reconciliation granularity for deployable units under an App.

## Decision

- Introduce a new ops-plane kind `Box` (Group: `ops.kompox.dev`, Version: `v1alpha1`).
  - Add alongside existing kinds: `Workspace`, `Provider`, `Cluster`, `App`.
  - Box represents a Deployment-like unit under an App. Its canonical identity is the Resource ID stored in `metadata.annotations["ops.kompox.dev/id"]`.
  - Typed Resource ID (alias: FQN) format with kind shortnames: `/ws/<ws>/prv/<prv>/cls/<cls>/app/<app>/box/<component>`.
- CLI split of responsibilities:
  - `kompoxops app` handles the default Box (`componentName=app`).
  - `kompoxops box` manages a toolbox-like workload in the App namespace. Select the target App via `--app-id` (FQN). Component selection defaults to `app` and is exposed in other commands when needed (e.g., secret operations via `--component`).
- PaaS user files:
  - `kompoxapp.yml` (kind: App) is required; a single file is sufficient to deploy the default Box.
  - `kompoxbox.yml` (kind: Box) is optional; acts as an overlay applied on top of the App defaults.
- References and IDs:
  - Use the typed, absolute Resource ID (alias: FQN) as the canonical ID for all kinds; it must be present at `metadata.annotations["ops.kompox.dev/id"]`.
  - Examples: Workspace=`/ws/<ws>` / Provider=`/ws/<ws>/prv/<prv>` / Cluster=`/ws/<ws>/prv/<prv>/cls/<cls>` / App=`/ws/<ws>/prv/<prv>/cls/<cls>/app/<app>` / Box=`/ws/<ws>/prv/<prv>/cls/<cls>/app/<app>/box/<component>`.
  - `ops.kompox.dev/path` is deprecated; see [K4x-ADR-009] for the Kompox Ops Manifest (KOM) schema and migration notes.

## Alternatives Considered

- No Box (App = Deployment): lacks flexibility; difficult to maintain multiple execution units or per-component differences within an App.
- Box only as a cluster-side CR (e.g., KompoxApp): misaligned granularity between CLI and Operator, fragmenting UX.
- CLI-only service-name targeting under App: poor type-safety/validation/maintainability.

## Consequences

- Pros:
  - Keeps App as the tenant boundary while explicitly defining the operational unit (Deployment-like).
  - Clear split between production operation (`app`) and investigation/dev (`box`) flows.
  - Future Operator can reconcile on a 1:1 basis per Box (e.g., KompoxApp), easing migration.
- Cons/Constraints:
  - One additional schema/CLI surface (Box).
  - Additional mapping/validation against Compose services (subset selection, etc.).

## Rollout

- Phase 1: Core types (completed as part of [K4x-ADR-007] Phase 1)
  - Add `Box` type to `config/crd/ops/v1alpha1`
  - Extend loader for Box ingestion and Resource ID parsing/validation
  - Implement App→Box reference validation
- Phase 2: CLI integration
  - Implement `kompoxops box` subcommand and options
  - Align `kompoxops app` to handle the default Box
  - Implement Box spec (currently placeholder)
- Phase 3: Documentation
  - Document Box mapping/naming in `design/v1/Kompox-CRD.ja.md` and `Kompox-KubeConverter.ja.md`

## References

- [K4x-ADR-007]
- [K4x-ADR-009]
- [Kompox-CRD.ja.md]
- [_dev/tasks/2025-10-13-crd-p1.ja.md]
- [_dev/tasks/2025-10-13-crd-p2.ja.md]
- [config/crd/ops/v1alpha1/README.md]

[K4x-ADR-007]: ./K4x-ADR-007.md
[K4x-ADR-009]: ./K4x-ADR-009.md
[Kompox-CRD.ja.md]: ../v1/Kompox-CRD.ja.md
[_dev/tasks/2025-10-13-crd-p1.ja.md]: ../../../_dev/tasks/2025-10-13-crd-p1.ja.md
[_dev/tasks/2025-10-13-crd-p2.ja.md]: ../../../_dev/tasks/2025-10-13-crd-p2.ja.md
[config/crd/ops/v1alpha1/README.md]: ../../../config/crd/ops/v1alpha1/README.md
