---
id: K4x-ADR-020
title: Migrate AKS driver provisioning from ARM template deployment to ARM REST ensure workflow
status: proposed
updated: 2026-02-24T10:31:42Z
language: en
supersedes: []
supersededBy: []
plans: [2026ac-aks-arm-rest-migration]
---
# K4x-ADR-020: Migrate AKS driver provisioning from ARM template deployment to ARM REST ensure workflow

## Context

The current AKS provider driver provisions infrastructure primarily through subscription-scope ARM template deployment and relies on deployment outputs as a central metadata source.

This approach creates operational coupling between lifecycle operations and deployment records. When deployment output retrieval is unavailable or inconsistent, follow-up operations can fail even if Azure resources still exist.

Kompox needs a convergent and idempotent provisioning model that can recreate or rediscover required resources directly from Azure resource state. At the same time, this ADR should avoid implementation-level detail and defer concrete method design and rollout sequencing to [2026ac-aks-arm-rest-migration].

## Decision

- Adopt ARM REST API based provisioning and deprovisioning workflow in the AKS provider driver.
- Replace single deployment-centric orchestration with resource-level ensure semantics inside the AKS adapter.
- Keep idempotency as a hard requirement: repeated execution with the same desired state must converge safely.
- Keep implementation boundaries stable: cmd/usecase/domain contracts remain unchanged; migration is contained in provider adapter internals.
- Treat Azure Storage backed diagnostic export as in-scope and maintained for cost-aware retention strategy.
- Exclude Key Vault and ACR lifecycle management from this migration scope.
- Delegate implementation details, exact resource order, method list, and phased rollout to [2026ac-aks-arm-rest-migration].

## Alternatives Considered

- Keep ARM template deployment as the primary provisioning mechanism.
  - Rejected: preserves deployment-output dependency and limits direct state-driven recovery.
- Hybrid model with deployment outputs as long-term primary source and REST as fallback.
  - Rejected for now: adds dual-path complexity without addressing the architectural center of gravity.
- Full architecture redesign across all provider operations in one step.
  - Rejected: too broad for the current migration objective and risk profile.

## Consequences

- Pros
  - Stronger idempotency and recovery by operating on concrete resource state.
  - Reduced coupling to deployment artifacts for cluster lifecycle operations.
  - Clear migration boundary that fits existing architecture layers.
- Cons/Constraints
  - Driver must own ARM REST operation handling, API versioning policy, and LRO behavior.
  - Error normalization and retry rules become explicit implementation responsibilities.
  - Additional verification is required to ensure parity with existing operational behavior.

## Rollout

- Rollout scope and sequencing are defined in [2026ac-aks-arm-rest-migration].
- This ADR is accepted when that plan's design and implementation checkpoints are completed and validated.

## References

- [2026ac-aks-arm-rest-migration]
- [Kompox-ProviderDriver]
- [Kompox-ProviderDriver-AKS]

[2026ac-aks-arm-rest-migration]: ../plans/2026/2026ac-aks-arm-rest-migration.ja.md
[Kompox-ProviderDriver]: ../v1/Kompox-ProviderDriver.ja.md
[Kompox-ProviderDriver-AKS]: ../v1/Kompox-ProviderDriver-AKS.ja.md
