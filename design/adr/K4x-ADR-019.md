---
id: K4x-ADR-019
title: Introduce NodePool abstraction for multi-provider cluster scaling and scheduling
status: accepted
updated: 2026-02-20T00:15:00Z
language: en
supersedes: []
supersededBy: []
plans: [2026ab-k8s-node-pool-support]
---
# K4x-ADR-019: Introduce NodePool abstraction for multi-provider cluster scaling and scheduling

## Context

Current cluster provisioning in AKS relies on mostly static pool definitions at provision time. This limits Day2 operations when zone availability and capacity conditions change.

Kompox needs a provider-neutral contract that supports dynamic node pool lifecycle operations after cluster creation, while keeping App-side scheduling input stable (`deployment.pool` / `deployment.zone`).

Provider terminology differs:
- AKS: Agent Pool
- GKE: Node Pool
- OKE: Node Pool
- EKS: Node Group

Despite naming differences, these resources represent the same operational concept for Kompox: a schedulable and scalable worker pool.

## Decision

- Introduce `NodePool` as the canonical cross-provider term in Kompox public contracts.
- Map provider-native terms in driver implementations:
  - AKS Agent Pool ↔ Kompox NodePool
  - EKS Node Group ↔ Kompox NodePool
  - GKE/OKE Node Pool ↔ Kompox NodePool
- Extend provider driver contract with NodePool lifecycle methods (overview level):
  - `NodePoolList(...)`
  - `NodePoolCreate(...)`
  - `NodePoolUpdate(...)`
  - `NodePoolDelete(...)`
- Do not add `NodePoolGet(...)` initially; resolve single pool lookup by filtering `NodePoolList(...)` results.
- Define capability behavior for unsupported providers:
  - Drivers that do not support NodePool lifecycle operations MUST return a deterministic `not implemented` error.
  - Callers MUST treat this as a capability boundary, not as a transient failure.
- Keep pod scheduling labels provider-neutral and explicit:
  - `kompox.dev/node-pool` as the primary pool selector label
  - `kompox.dev/node-zone` as the Kompox-level zone selector label
- Keep `deployment.pool` / `deployment.zone` as App-level inputs and keep normalization/mapping responsibility in each provider driver.
- Preserve backward compatibility for existing App inputs:
  - Existing `deployment.pool` / `deployment.zone` semantics remain valid.
  - If no explicit pool is specified, current default behavior remains unchanged.
- Define update error model at contract level:
  - Unsupported update fields are treated as validation errors when the field is known but not mutable.
  - Unsupported features at provider level are treated as `not implemented`.
- Clarify responsibility boundaries:
  - Converter is responsible for emitting scheduling labels from App intent.
  - Provider drivers are responsible for validating/mapping label intent to provider-native pool and zone configuration.

## Alternatives Considered

- Use provider-specific terms in the public contract (`AgentPool`, `NodeGroup`, etc.)
  - Rejected: leaks provider semantics into shared interfaces and increases cross-provider complexity.
- Use only Kubernetes standard topology labels for scheduling (`topology.kubernetes.io/zone`)
  - Rejected: value conventions vary by provider and region; Kompox needs a stable app-facing contract.
- Add only free-form `map[string]string` options for pool control
  - Rejected: weak typing and poor validation for long-term public API stability.

## Consequences

- Pros
  - A stable and portable contract for pool lifecycle operations across providers.
  - Better Day2 operability for capacity/zone changes without re-provisioning clusters.
  - Clear separation between App scheduling intent and provider-specific implementation details.
- Cons/Constraints
  - Drivers must implement mapping and validation boundaries explicitly.
  - `NodePoolGet` absence may require list-and-filter at call sites until proven insufficient.
  - `kompox.dev/node-zone` requires consistent label management by each driver.
  - A consistent distinction between validation error and `not implemented` must be enforced across drivers.

## Rollout

- Implementation sequencing is tracked by [2026ab-k8s-node-pool-support].
- This ADR defines the contract-level decision; code-level phasing (AKS-first implementation and tests) is managed in the plan/tasks.

## References

- [2026ab-k8s-node-pool-support]
- [Kompox-ProviderDriver]
- [Kompox-ProviderDriver-AKS]
- [Kompox-KubeConverter]

[2026ab-k8s-node-pool-support]: ../plans/2026/2026ab-k8s-node-pool-support.ja.md
[Kompox-ProviderDriver]: ../v1/Kompox-ProviderDriver.ja.md
[Kompox-ProviderDriver-AKS]: ../v1/Kompox-ProviderDriver-AKS.ja.md
[Kompox-KubeConverter]: ../v1/Kompox-KubeConverter.ja.md
