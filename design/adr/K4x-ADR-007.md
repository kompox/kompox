---
id: K4x-ADR-007
title: Introduce CRD-style configuration
status: accepted
updated: 2025-10-13
language: en
supersedes: []
supersededBy: [K4x-ADR-009]
---
# K4x-ADR-007: Introduce CRD-style configuration

## Context

- We are improving Kompox v1 CLI UX for multi-cluster/multi-app DevOps by splitting a monolithic `kompoxops.yml` into multiple YAML documents.
- We also want forward compatibility with a potential v2 Operator while keeping v1 CLI file-scan mode first-class.
- Users author app configs (compose + env) per app folder; multiple boxes (Deployment-like units) may exist under the same App.
- Prior discussions established: API group `ops.kompox.dev`, version `v1alpha1`, kinds `Workspace`, `Provider`, `Cluster`, `App`, and `Box` (`Box` is a new kind introduced in [K4x-ADR-008]).

## Decision

- Define CRD-style configuration schema for v1 CLI under Group/Version/Kind:
  - group: `ops.kompox.dev`, version: `v1alpha1`, kinds: `Workspace|Provider|Cluster|App|Box`
  - Package path for Go types: `config/crd/ops/v1alpha1` (package name: `v1alpha1`)
- Import loader (CLI):
  - Accepts directory input (recursive), supports multi-document YAML
  - Only one shorthand for location is supported in import files:
    - `metadata.annotations["ops.kompox.dev/path"] = "<ws>/<prv>/<cls>[/<app>[/<box>]]"`
  - Reads all documents first, then validates in order: Workspace → Provider → Cluster → App → Box
  - On any error (missing parent, path mismatch, duplicate names), aborts without modifying the database
- IDs and storage (inmem/RDB):
  - Use FQN as the canonical identifier (primary key) per kind:
    - Workspace: `ws`
    - Provider: `ws/prv`
    - Cluster: `ws/prv/cls`
    - App: `ws/prv/cls/app`
    - Box: `ws/prv/cls/app/box`
  - Repositories must honor pre-set IDs on Create and only auto-generate when empty; Get retrieves by ID(=FQN). Duplicate IDs should raise errors.
  - Optionally migrate later to UUID PK + FQN UNIQUE when scale/rename requirements arise
- Kubernetes internal representation (for potential cluster storage/operator):
  - Prefer labels over annotations for selection/indexing
  - Common labels by scope (only attach relevant levels):
    - `app.kubernetes.io/managed-by=kompox`
    - `ops.kompox.dev/workspace`, `/provider`, `/cluster`, `/app`, `/box`
    - and their `-hash` counterparts for selector-friendly short values
  - Namespace naming (own-kind hash is used):
    - `k4x-ws-<wsHash>-<wsName>`, `k4x-prv-<prvHash>-<prvName>`, `k4x-cls-<clsHash>-<clsName>`, `k4x-app-<appHash>-<appName>`
  - For `Workspace/Provider/Cluster/App`, add `status.opsNamespace`
- CLI ingestion (v1):
  - Flags and env vars:
    - `--crd-path <PATH>` (repeatable) to import CRD-style YAML from files/directories recursively; supports multi-document YAML; environment override `KOMPOX_CRD_PATH=path1,path2` (comma-separated). Missing path is an error. Flags take precedence over env vars.
    - `--crd-app <PATH>` (default: `./kompoxapp.yml`) to help infer default app name; environment override `KOMPOX_CRD_APP`. If the path does not exist, it is ignored (not an error).
  - Activation & precedence:
    - When any CRD inputs (from `--crd-path` and existing `--crd-app`) are successfully loaded and validated, the CLI enters CRD mode and ignores `--db-url`/`KOMPOX_DB_URL` for that session.
    - If no CRD inputs are present/loaded, fall back to `--db-url` with the default `file:kompoxops.yml`.
  - Default ID inference:
    - If exactly one `App` (Kind: App) exists within the `--crd-app` input scope and `--app-id` is not provided, use its FQN(`ws/prv/cls/app`) as the default `--app-id`. If the referenced cluster is uniquely determined, set `--cluster-id` to its FQN as the default as well.
  - Resource selection flags:
    - Add `--app-id` and `--cluster-id` to accept resource IDs(FQN) directly and resolve via UseCase `Get(ID)`.
    - Keep `--app-name` and `--cluster-name` for backward compatibility, but error out immediately when multiple matches exist. Do not attempt ambiguous resolution.
  - Validation & errors:
    - Validation is all-or-nothing after collecting all documents. On error, do not modify the database and include the source file path and the 1-based document index in error messages.
- CLI UX split:
  - `kompoxops app ...` operates the default box (componentName=app) using kompoxapp.yml (and optional kompoxbox.yml overlay)
  - `kompoxops box ...` operates an arbitrary box; flags can override/compose without requiring files
- Naming constraints:
  - Each path segment (ws/prv/cls/app/box) must be a DNS-1123 label, 1..63 chars, lowercase alnum + `-`
  - FQN length is unrestricted internally; do not place full FQN into labels; use `-hash` labels and truncated+hash names for K8s resources

## Alternatives Considered

- Use `ops.kompox.dev/location` instead of `.../path`: rejected due to common use of "location" as region/placement terminology (e.g., Azure)
- Derive scope from directory layout or CLI flags: rejected for import files to keep behavior explicit and deterministic
- Allow multiple shorthand keys (path/location/fqn): rejected to avoid complexity and ambiguity
- ClusterScoped CRs with encoded names instead of namespaces: rejected; hurts RBAC and cleanup ergonomics

## Consequences

- Pros:
  - Clear, deterministic import behavior; single shorthand key keeps the loader simple
  - FQN IDs align CLI, UseCase, and stores with a single source of truth
  - K8s alignment (GVK + labels + namespacing) eases future Operator adoption without breaking v1
- Cons/Constraints:
  - Import files must include `ops.kompox.dev/path`; no directory-based inference
  - Renaming a segment changes the FQN; treated as delete + create unless we later adopt surrogate IDs
  - Additional name-length handling required when generating K8s resource names
  - Long string PKs have index/storage overhead on some RDBs; acceptable for now, with an escape hatch to UUID PK + FQN UNIQUE when needed

## Rollout

- Phase 1: Core types and loader
  - Add `config/crd/ops/v1alpha1` types with path-only shorthand support
  - Implement stateless Loader (directory recursion, multi-document YAML)
  - Implement Validator (topological sort, parent validation, duplicate detection)
  - Implement immutable Sink (read-only index with FQN keys)
  - Document tracking: `Document.Path` (source file) and `Document.Index` (position in multi-document YAML)
  - Enhanced error reporting: validation errors include source file path and document index
- Phase 2: UseCase and CLI integration
  - Integrate loader/validator/sink into CLI commands
  - Add `--crd-path` (repeatable) and `--crd-app` flags with env var support and precedence rules
  - Activate CRD mode on successful import/validation and ignore `--db-url` in that session
  - Implement `app`/`box` CLI split with validations
  - Implement plan/deploy flow
- Phase 3: Persistent storage (optional)
  - Add RDB sink with identical FQN keys
  - Consider UUID PK + FQN UNIQUE when scale/rename needs justify it
- Phase 4 (optional): K8s cluster storage
  - K8s cluster storage mode using the same schema + labels
  - Operator may watch `ops.kompox.dev/*` resources

## References

- [K4x-ADR-008]
- [Kompox-CLI.ja.md]
- [Kompox-CRD.ja.md]
- [Kompox-KubeConverter.ja.md]
- [2025-10-12-workspace.ja.md]
- [2025-10-13-crd-p1.ja.md]
- [2025-10-13-crd-p2.ja.md]
- [config/crd/ops/v1alpha1/README.md]

[K4x-ADR-008]: ./K4x-ADR-008.md
[Kompox-CLI.ja.md]: ../v1/Kompox-CLI.ja.md
[Kompox-CRD.ja.md]: ../v1/Kompox-CRD.ja.md
[Kompox-KubeConverter.ja.md]: ../v1/Kompox-KubeConverter.ja.md
[2025-10-12-workspace.ja.md]: ../../../_dev/tasks/2025-10-12-workspace.ja.md
[2025-10-13-crd-p1.ja.md]: ../../../_dev/tasks/2025-10-13-crd-p1.ja.md
[2025-10-13-crd-p2.ja.md]: ../../../_dev/tasks/2025-10-13-crd-p2.ja.md
[config/crd/ops/v1alpha1/README.md]: ../../../config/crd/ops/v1alpha1/README.md
