---
id: K4x-ADR-009
title: Kompox Ops Manifest Schema
status: proposed
updated: 2025-10-15
language: en
supersedes: [K4x-ADR-007]
supersededBy: []
---
# K4x-ADR-009: Kompox Ops Manifest Schema

## Context

- Kompox v1 CLI ingests CRD-style YAMLs (Group: `ops.kompox.dev`, Version: `v1alpha1`, Kinds: `Workspace|Provider|Cluster|App|Box`).
- [K4x-ADR-007] used an FQN-like path and the annotation `ops.kompox.dev/path` as a shorthand to derive identity, with Workspace being a special case.
- We want a uniform, self-describing, and future-proof identifier that:
  - Works for all kinds (including `Workspace`) without special casing
  - Is stable and suitable as the canonical primary key across loaders and stores
  - Is extensible to new kinds (e.g., `Ingress` under `Cluster`)

## Decision

- Introduce Kompox Ops Manifest (KOM) schema and a single, canonical identifier: `metadata.annotations["ops.kompox.dev/id"]` (a.k.a. Resource ID, alias: FQN).
- Format: typed, absolute, slash-prefixed path with kind shortnames.
  - Grammar: `/ <kind> \/ <name> ( \/ <kind> \/ <name> )*`
  - Kind shortnames: `ws|prv|cls|app|box|...` (extensible; e.g., `ing` for Ingress)
  - Name constraints: DNS-1123 label (lowercase alnum and `-`, 1..63 chars)
- Required for all kinds, including Workspace.
  - Examples:

```yaml
# Workspace
apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: ws1
  annotations:
    ops.kompox.dev/id: /ws/ws1

# Provider
apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: prv1
  annotations:
    ops.kompox.dev/id: /ws/ws1/prv/prv1

# Cluster
apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  name: cls1
  annotations:
    ops.kompox.dev/id: /ws/ws1/prv/prv1/cls/cls1

# App
apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  name: app1
  annotations:
    ops.kompox.dev/id: /ws/ws1/prv/prv1/cls/cls1/app/app1

# Box
apiVersion: ops.kompox.dev/v1alpha1
kind: Box
metadata:
  name: api
  annotations:
    ops.kompox.dev/id: /ws/ws1/prv/prv1/cls/cls1/app/app1/box/api
```

- Use this Resource ID as the canonical primary key in memory and RDB stores; derive any K8s-label-safe hashes from it (avoid putting the full ID into labels).
- Validation rules:
  - `metadata.kind` must equal the last `<kind>` segment (e.g., `Box` ⇔ `.../box/<name>`)
  - `metadata.name` must equal the last `<name>` segment
  - All `<name>` segments are DNS-1123 labels; all `<kind>` segments are recognized shortnames
  - Parent chain must be structurally valid (e.g., `Box` requires `.../app/<appName>`)

## Terminology

- Kompox Ops Manifest (KOM):
  - The CRD-style portable YAML schema for Kompox defined by this ADR.

- Resource ID (ID) — alias: FQN:
  - Format: `/ <kind> / <name> ( / <kind> / <name> )*`
  - Example: `/ws/ws1/prv/prv1/cls/cls1/app/app1/box/api`
  - Meaning: Canonical, unique, and stable identifier for each resource, stored in `metadata.annotations["ops.kompox.dev/id"]`.
  - Constraints: Each `<name>` is a DNS-1123 label; leading slash required; the tail represents the resource itself `<kind>/<name>`.
  - Usage: Primary key, references, logs/errors, input to short-hash derivation.

- Kind shortnames:
  - Defined: `ws` (Workspace), `prv` (Provider), `cls` (Cluster), `app` (App), `box` (Box)
  - Future examples: `ing` (Ingress) can be added with the same rule
  - Mapping: `metadata.kind` equals the last `<kind>` segment (e.g., `.../box/<name>` ⇔ `kind: Box`)

## Alternatives Considered

- Keep `ops.kompox.dev/path` (untyped or parent-only path):
  - Ambiguous intent (parent scope vs self ID), requires additional resolution, harder diagnostics.
- Use `ops.kompox.dev/scope`:
  - Similar ambiguity to `path`; still needs a separate self ID.
- Exempt Workspace from annotation:
  - Introduces special casing and branching in loader/validator.

## Consequences

- Pros:
  - Uniform identity across all kinds; simpler loader/validator and clearer error messages
  - Future-proof via typed segments; documents can be understood in isolation
  - Single source of truth for hashing, selection, and cross-references
- Cons/Constraints:
  - Backwards incompatible with [K4x-ADR-007] (`ops.kompox.dev/path` and untyped FQN).
  - Requires coordinated updates in loaders, stores, CLI flags/docs, and tests.

## Rollout

- Types & Loader:
  - Update `config/crd/ops/v1alpha1` types and the CLI loader to require and parse `ops.kompox.dev/id`.
  - Implement strict validation rules (kind/name alignment, DNS-1123, parent structure).
- Storage:
  - Use Resource ID as the primary key (inmem/RDB). Keep existing unique indexes aligned if any.
  - Derive label-safe short hashes from the Resource ID for Kubernetes resource names/labels.
- CLI:
  - Accept `--*-id` flags where applicable; surface IDs in error and help messages.
  - Keep `kompoxops app` as default Box handler (`componentName=app`), consistent with [K4x-ADR-008].
- Documentation:
  - Update `design/v1/Kompox-CRD.ja.md` to the new ID scheme and examples.
  - Update [K4x-ADR-008] wording where it references FQN/path; no supersession needed for [K4x-ADR-008].
  - Regenerate indexes via `make gen-index`.
- Tests:
  - Update end-to-end and fixtures to the new scheme, including `tests/aks-e2e-crd/`.
  - Migrate sample CRD YAMLs to use `ops.kompox.dev/id` and typed Resource IDs.
  - Adjust assertions and selectors that relied on `ops.kompox.dev/path` or untyped FQN.

## References

- [K4x-ADR-007]
- [K4x-ADR-008]
- [Kompox-CRD.ja.md]
- [_dev/tasks/2025-10-13-crd-p1.ja.md]
- [_dev/tasks/2025-10-13-crd-p2.ja.md]

[K4x-ADR-007]: ./K4x-ADR-007.md
[K4x-ADR-008]: ./K4x-ADR-008.md
[Kompox-CRD.ja.md]: ../v1/Kompox-CRD.ja.md
[_dev/tasks/2025-10-13-crd-p1.ja.md]: ../../../_dev/tasks/2025-10-13-crd-p1.ja.md
[_dev/tasks/2025-10-13-crd-p2.ja.md]: ../../../_dev/tasks/2025-10-13-crd-p2.ja.md
