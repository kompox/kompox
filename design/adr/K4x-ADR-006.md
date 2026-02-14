---
id: K4x-ADR-006
title: Rename domain model Service to Workspace
status: accepted
updated: 2025-10-12
language: en
supersedes: []
supersededBy: []
---
# K4x-ADR-006: Rename domain model Service to Workspace

## Context

- In the Kompox domain model, Service represents the top-level logical grouping for Providers, Clusters, and Apps. It functions as the user’s operational unit for DevOps across multiple clusters and applications.
- The term “Service” collides with Kubernetes Service (core/v1) and also with application-level “service,” causing confusion in docs, CLI help, and conversations.
- We are introducing directory-scoped YAML loading and CRD-style configs to improve multi-app UX. Aligning terminology now reduces cognitive load as we evolve toward a v2 Operator.
- Backward compatibility is not required for this rename. Prioritize the fastest path to build/test green.
- Store (RDB/in-memory) migration concerns are out of scope for this change: there are no external users yet, so schema compatibility is not required.

## Decision

- Rename the domain concept Service to Workspace.
  - domain/model: `Service` → `Workspace` (source-level type rename)
  - domain interfaces: `ServiceRepository` → `WorkspaceRepository`
  - adapters/store: in-memory and RDB implementations renamed accordingly
- No backward compatibility shims are required at the API surface. We will only use a temporary type alias to get the codebase compiling quickly during the transition.
- Temporary alias to enable the fastest green build/tests while refactoring across packages:
  - Introduce `type Service = Workspace` immediately after creating `Workspace`.
  - Remove the alias once all compile errors and failing tests are resolved under the new names.
- External inputs/docs:
  - We will update docs after code compiles and tests pass. We will not rewrite past ADRs or `_dev/tasks`.
  - Do not rename Kubernetes Service–related identifiers or ServiceAccount (they refer to k8s primitives).
 - CLI changes:
   - Flags: `--service` is renamed to `--workspace`.
   - Command group: `admin service` is renamed to `admin workspace`.
   - No backward-compatible aliases will be provided.

## Alternatives Considered

- Keep “Service” as-is: Minimizes churn but keeps ambiguity with Kubernetes Service and app-level “service.”
- Rename to Tenant: Strong multi-tenant connotation (RBAC/quotas/billing) that doesn’t match the intended operational scope.
- Rename to Project: Common term but may conflict with Git/CI/CD project semantics; less precise for our grouping semantics.
- Rename to Realm/Domain/Space/Stack: Either overloaded (Domain/Realm) or too vague/context-specific (Space/Stack).

## Consequences

- Pros:
  - Reduces ambiguity with Kubernetes Service and app “services.”
  - Matches user mental model: a Workspace as the operational unit grouping Providers, Clusters, and Apps.
  - Clears the path for v2 Operator docs where “Service” would otherwise be confusing.
- Cons/Constraints:
  - Breaking change across the codebase; consumers must adapt to new names.
  - Requires coordinated rename across domain interfaces and adapters.
  - Potential code churn in tests and logs.

## Rollout

- Implementation plan and checklists are maintained in [2025-10-12-workspace].

### Source code comment policy

- Do not add historical rename comments such as “formerly Service” in source code.
- Only add explanatory comments when a `Service` identifier or literal must remain for legitimate reasons (for example, persisted schema or external wire formats).
- Follow repository comment guidelines: write timeless, useful comments that explain why code exists, not its change history.

## References

- Design docs
  - [Kompox-Arch-Implementation-ja]
  - [Kompox-Spec-Draft-ja]
- Tasks
  - [2025-10-12-workspace]

[Kompox-Arch-Implementation-ja]: ../v1/Kompox-Arch-Implementation.ja.md
[Kompox-Spec-Draft-ja]: ../v1/Kompox-Spec-Draft.ja.md
[2025-10-12-workspace]: ../../_dev/tasks/2025-10-12-workspace.ja.md