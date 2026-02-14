---
id: K4x-ADR-017
title: Define App/Box model, Compose mapping, and CLI selectors
status: proposed
updated: 2026-02-14
language: en
supersedes: [K4x-ADR-008]
supersededBy: []
---
# K4x-ADR-017: Define App/Box model, Compose mapping, and CLI selectors

## Context

[K4x-ADR-008] introduced `Box` as a first-class ops kind, but left the Box spec and its integration details as placeholders.

We need a concrete, stable v1 direction for:
- The App/Box resource contract (including Compose-derived boxes and standalone boxes)
- How Docker Compose services map to Kubernetes deployable units (components)
- A consistent CLI selection model for targeting pods/containers when multiple components exist under one App

## Decision

- Treat App as the tenant boundary (1 App = 1 Namespace) and Box as the deployable unit (component) under an App.
- Support two categories under the same kind `Box`:
  - Compose Box: splits a subset of Compose services into an independent component
  - Standalone Box: deploys a toolbox-like workload in the App namespace, independent of Compose topology
- Define a deterministic Compose services → Box mapping and validation rules.
- Standardize CLI selectors for “single target” operations around:
  - `--component` (default: `app`)
  - `--pod`
  - `--container`

Detailed schema, validation rules, NetworkPolicy defaults, ingress distribution rules, and examples are specified in [2026aa-kompox-box-update.ja.md]. This ADR only records the decision to adopt that model and to use that document as the normative design reference for implementation.

## Alternatives Considered

- Keep Box as a placeholder and implement per-feature ad-hoc behavior
  - Rejected: leads to drift across Converter/CLI and makes validation ambiguous.
- Introduce separate kinds for Compose and Standalone units
  - Rejected: fragments the component model and complicates CLI/policy application.

## Consequences

- Pros
  - A single, coherent “component” concept for operations, policy, and CLI selection.
  - Clear validation rules reduce surprising behavior and implementation complexity.
- Cons/Constraints
  - Requires phasing: CRD/spec expansion and validation must precede broader Converter/CLI changes.

## Rollout

- Phase 1: Treat [2026aa-kompox-box-update.ja.md] as the design source of truth for implementation.
- Phase 2: Expand BoxSpec from the current placeholder to support the minimal v1 fields needed by the draft.
- Phase 3: Implement loader-time validation and deterministic Compose services → component mapping.
- Phase 4: Update Kubernetes conversion so outputs (Deployment/Service/Ingress/NetworkPolicy) are produced per component, including default-deny ingress with baseline allowances.
- Phase 5: Renew CLI targeting for single-target operations using `--component/--pod/--container`, keeping backward-compatible defaults where possible.

Migration notes:
- Apps without Box resources continue to deploy as a single default component (`app`).
- Standalone Box operations remain available under `kompoxops box` during the transition; Compose-derived components are primarily targeted via `kompoxops app` selectors.

## References

- [K4x-ADR-008]
- [K4x-ADR-009]
- [2026aa-kompox-box-update.ja.md]

[K4x-ADR-008]: ./K4x-ADR-008.md
[K4x-ADR-009]: ./K4x-ADR-009.md
[2026aa-kompox-box-update.ja.md]: ../plans/2026/2026aa-kompox-box-update.ja.md
