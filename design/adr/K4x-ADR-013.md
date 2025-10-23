---
id: K4x-ADR-013
title: Resource protection policy for Cluster operations
status: accepted
date: 2025-10-23
supersedes: []
supersededBy: []
---

## Context

- Operators need guardrails to prevent accidental `kompoxops cluster deprovision` and `kompoxops cluster uninstall` from deleting or mutating live environments.
- We want a provider-neutral policy that is enforced consistently in Kompox (UseCase and CLI), while optionally mapping to provider-native locks (e.g., Azure Management Locks) for defense-in-depth.
- Vocabulary needs to be unambiguous across two operation scopes:
  - Cloud/infrastructure lifecycle (provision/deprovision)
  - In-cluster lifecycle (install/uninstall/updates)
- Prior ADRs define style/structure and external-contract discipline for Kompox configuration and models. [K4x-ADR-012]

## Decision

- Add a protection policy to Cluster via `spec.protection` (noun form) with two explicit scopes:
  - `provisioning`: controls cloud/infrastructure lifecycle operations (provision/deprovision and related destructive/mutating actions outside the cluster)
  - `installation`: controls in-cluster lifecycle operations (install/uninstall and mutating changes inside the cluster)
- Enumerated values for both scopes are unified:
  - `none`: no restriction
  - `cannotDelete`: block destructive operations (e.g., deprovision/uninstall)
  - `readOnly`: block destructive and mutating operations (treat as immutable)
- Enforcement model (defense-in-depth):
  - UseCase: hard block regardless of CLI flags (e.g., ignore `--force`), returning a clear error message indicating the scope/value and how to unlock (set to `none`).
  - CLI: early guard to prevent obvious mistakes and provide fast feedback; messages mirror UseCase wording.
  - Finalizer: when either scope is `cannotDelete` or `readOnly`, do not remove the finalizer so that CR deletion is effectively blocked at the API server level.
  - Providers: drivers may synchronize the policy with native protections. For Azure AKS: map `cannotDelete` → `CanNotDelete` lock, `readOnly` → `ReadOnly` lock; `none` removes Kompox-managed locks. This mapping is optional but recommended.
- Creation semantics:
  - First-time creation is not blocked by `readOnly`. Protection values govern post-creation behavior.
  - Provisioning scope: if Kompox determines that no managed infrastructure exists yet (e.g., the target resource ID does not exist), initial `provision` proceeds; after success, the chosen protection (and provider locks if enabled) is enforced for subsequent operations.
  - Installation scope: if Kompox determines there has been no prior installation, initial `install` proceeds; after success, subsequent installs/upgrades/uninstalls are governed by the protection value.
  - Implementation may detect “first-time” via Kompox status fields and/or provider existence checks; drivers should reconcile locks immediately after successful creation.
- Defaults and UX:
  - Default is `none` for both scopes (backward compatible).
  - To perform destructive or mutating operations when protected, users must explicitly edit the Cluster to set the relevant scope to `none` before re-running the command.

## Alternatives Considered

- Field name
  - `spec.protected` (adjective) vs `spec.protection` (noun). Chosen: `spec.protection` to represent a policy bucket, aligning with common API style.
- Scope naming
  - Considered `infrastructure|incluster`, `cloud|platform`, and `deployment|cluster`. Rejected to avoid ambiguity with Kubernetes resources (e.g., Deployment) and to keep symmetry with common terms: `provisioning`/`installation`.
- Value set
  - `none|noDelete|immutable` (clear severity levels, widely understood)
  - `none|deleteLocked|updateLocked` (intuitive but more colloquial)
  - Boolean flags (`deleteProtected`, `updateProtected`)—rejected due to combinatorial ambiguity and less expressive UX.
- Enforcement-only approaches
  - Rely solely on finalizers/admission policy—harder to give good UX and provider integration.
  - Rely solely on provider locks—does not catch CLI/UseCase paths consistently and is provider-specific.

## Consequences

- Pros
  - Safer day-2 operations by requiring explicit unlock for destructive/mutating actions.
  - Consistent vocabulary across scopes and commands; clear user error messages and remediation.
  - Optional, idempotent mapping to provider-native locks for extra protection (e.g., Azure Management Locks).
- Cons/Constraints
  - Slight operational friction: users must edit the Cluster to unlock (intentional).
  - `readOnly` is strict; operational playbooks must plan temporary unlock flows for valid maintenance.
  - Providers without native lock features rely on Kompox enforcement only.

## Rollout

- Step 1: Extend CRD and domain model
  - Add `spec.protection.provisioning|installation` with enum values `none|cannotDelete|readOnly`, default `none`.
  - Map to the domain model and ensure serialization/validation.
- Step 2: Enforce in UseCases
  - Deprovision path consults `provisioning`; uninstall/installation/update paths consult `installation`.
  - Standardize error messages and ignore `--force` when protected.
- Step 3: Add CLI early guards
  - Mirror UseCase logic for fast feedback and consistent wording.
- Step 4: Finalizer behavior
  - Keep finalizer when either scope is `cannotDelete` or `readOnly` to prevent CR deletion.
- Step 5 (optional, provider-specific): Azure mapping
  - Reconcile Azure Management Locks: `cannotDelete` ↔ `CanNotDelete`, `readOnly` ↔ `ReadOnly`, `none` ↔ remove Kompox-managed lock.
  - Ensure idempotent lock creation/update/removal.
- Step 6: Documentation, tests, and index
  - Update CRD/spec docs and CLI help; add unit/E2E tests for guarded operations.
  - Regenerate documentation indices (e.g., `make gen-index`).

## References

- [K4x-ADR-012]
- [Kompox-Spec-Draft.ja.md]
- [Kompox-CRD.ja.md]
- [Kompox-Arch-Implementation.ja.md]
- [2025-10-23-aks-cr.ja.md]
- [Azure-Management-Locks]

[K4x-ADR-012]: ./K4x-ADR-012.md
[Kompox-Spec-Draft.ja.md]: ../v1/Kompox-Spec-Draft.ja.md
[Kompox-CRD.ja.md]: ../v1/Kompox-CRD.ja.md
[Kompox-Arch-Implementation.ja.md]: ../v1/Kompox-Arch-Implementation.ja.md
[2025-10-23-aks-cr.ja.md]: ../../_dev/tasks/2025-10-23-aks-cr.ja.md
[Azure-Management-Locks]: https://learn.microsoft.com/azure/azure-resource-manager/management/lock-resources
