---
id: K4x-ADR-011
title: Introduce Defaults pseudo-resource for KOM ingestion
status: accepted
updated: 2025-10-17
language: en
supersedes: []
supersededBy: []
---
# K4x-ADR-011: Introduce Defaults pseudo-resource for KOM ingestion

## Context

- We need a clear entrypoint file (kompoxapp.yml) to declare both: (1) which local KOM documents to load, and (2) which App to treat as the default when CLI flags are omitted.
- Previous heuristics such as "use the only App found in kompoxapp.yml" become unreliable once composition is allowed, and relative-path resolution for composed files is ambiguous and unsafe.
- Security and reproducibility require strictly local-only inputs with no wildcards; document appearance order is not significant because we aggregate, topologically sort, and validate after loading.

## Decision

- Add a pseudo-kind Defaults that declares loading targets and default selections for KOM ingestion. Defaults is a loader/CLI concern and is not persisted.

  ```yaml
  apiVersion: ops.kompox.dev/v1alpha1
  kind: Defaults
  spec:
    komPath:
      - file.yml                    # same-directory relative file
      - ./path/to/dir               # local relative-path directory → recursive load (YAML only)
      - /path/to/absolute.yml       # local absolute-path file
    appId: /ws/myws/prv/az/cls/prod/app/web
  ```

- Semantics:
  - komPath: list of local filesystem locations to load. Rules:
    - Local-only; remote URLs are not supported.
    - No wildcards ("*", "?", "[").
    - Directories are scanned recursively (YAML files only).
    - Relative paths resolve against the directory of the kompoxapp.yml file (the one that contains Defaults).
  - appId: default Resource ID for the App when the CLI flag is omitted.
  - Order independence: the loader aggregates all documents then performs topological sort and validation; appearance order does not matter.
  - Local FS reference constraints for App documents:
    - Only App documents that are directly present in the kompoxapp.yml file (Document.Path == --kom-app) may use local filesystem references such as file:compose.yml or relative hostPath volumes (e.g., ./data:/data). All other Apps must not contain local filesystem references; violating documents cause validation errors.

- CLI precedence:
  - For load paths: `--kom-path` > `KOMPOX_KOM_PATH` > `Defaults.spec.komPath` > none.
  - For default App: `--app-id` > `Defaults.spec.appId` > if exactly one App is directly present in `--kom-app`, use it; otherwise, the App must be specified.

## Alternatives Considered

- Pseudo-kind Include with multi-include semantics (string/object list, per-item options): rejected.
  - Adds complexity (loop handling, base path ambiguity), weakens security expectations, and obscures the single entrypoint model.
  - Defaults with explicit komPath is simpler, more predictable, and aligns with the local-only policy.
- Annotation-driven defaults on App: less explicit and requires editing the App document; Defaults decouples entrypoint control from resource manifests.

## Consequences

- Pros
  - Single, explicit entrypoint with clear defaults and load targets.
  - Deterministic and secure: local-only, no wildcards, order-independent.
  - Keeps relative-path resolution simple (base = kompoxapp.yml directory) and prevents ambiguous references.

- Cons/Constraints
  - Users must enumerate files or directories explicitly (no wildcard convenience).
  - Apps not directly authored in kompoxapp.yml cannot use local filesystem references.

## Rollout

- Implement Defaults parsing in the CLI/loader front-end; treat it as non-persisted pseudo-kind.
- Use komPath to drive recursive local loading; maintain visited de-duplication and file size limits.
- Enforce validation rules for local filesystem references based on document provenance (directly in `--kom-app` vs loaded from komPath).
- Update docs and examples in Kompox design/specs.
- Add unit tests covering precedence, path resolution, directory recursion, prohibition of wildcards, and FS-reference validation.
- Regenerate design indices (`make gen-index`).

## References

- [K4x-ADR-007]
- [K4x-ADR-009]
- [Kompox-KOM.ja.md]
- [Kompox-KubeConverter.ja.md]
- [_dev/tasks/2025-10-17-defaults.ja.md]

[K4x-ADR-007]: ./K4x-ADR-007.md
[K4x-ADR-009]: ./K4x-ADR-009.md
[Kompox-KOM.ja.md]: ../v1/Kompox-KOM.ja.md
[Kompox-KubeConverter.ja.md]: ../v1/Kompox-KubeConverter.ja.md
[_dev/tasks/2025-10-17-defaults.ja.md]: ../../_dev/tasks/2025-10-17-defaults.ja.md
