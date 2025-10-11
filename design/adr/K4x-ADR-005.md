---
id: K4x-ADR-005
title: Support Compose configs/secrets and make bind volumes directory-only
status: accepted
date: 2025-10-10
supersedes: []
supersededBy: []
---

## Context

- We want Kompox to handle single-file mounts like `nginx.conf` or credentials in a portable, declarative way without relying on runtime filesystem heuristics.
- Docker Compose specification includes standard keys `configs` and `secrets` (originating from Swarm) to model non-image configuration and sensitive data as single files. These map naturally to Kubernetes `ConfigMap` and `Secret`.
- Today, Kompox only interprets `services.<svc>.volumes` and treats relative bind paths as PVC subPath mounts. This overloads volumes for both directories and single files, making it ambiguous and coupling behavior to local filesystem state.
- There is no installed user base yet; we can enact a clean rule change without backward-compatibility constraints.

## Decision

Adopt Compose standard `configs`/`secrets` as the only way to express single-file mounts. Limit `volumes` bind mounts to directories. Implement full support in `kube.Converter` as follows.

1) Compose support
- Parse and honor top-level `configs` and `secrets` and their service-level references:
  - Service reference shape: `{ source, target, mode? }` (uid/gid ignored)
  - Top-level definition: `{ file | external | name }` (we support `file` and `name`, external-as-name passthrough)
- Keep `volumes` semantics but restrict bind to directories only.

2) Kubernetes translation
- configs → Kubernetes `ConfigMap`
  - Constraints: file must be UTF-8 (no BOM) with no NUL bytes; size ≤ 1 MiB per ConfigMap
  - Key: `basename(file)` unless overridden by the service reference `target` filename (subPath)
  - Mount: single file via `volumes` + `volumeMounts` with `subPath=<key>`, `mountPath=<target>`, `readOnly: true`
  - Annotate each ConfigMap with `kompox.dev/compose-content-hash` (algorithm: `ComputeContentHash` — deterministic hash over sorted `KEY=VALUE` entries joined with NUL separators).
- secrets → Kubernetes `Secret` (type `Opaque`)
  - Content: any bytes allowed; if valid UTF-8 and no NUL → store in `data`, else `binaryData`
  - Annotate each Secret with `kompox.dev/compose-content-hash` (algorithm: `ComputeContentHash`).
  - Mount: single file via `volumes` + `volumeMounts` with `subPath=<key>`, `mountPath=<target>`, `readOnly: true`
- Resource naming
  - ConfigMap name: `<appName>-<componentName>--cfg-<configName>`
  - Secret name:    `<appName>-<componentName>--sec-<secretName>`
  - `configName` / `secretName` must be DNS-1123 label compliant (lowercase alphanumeric or `-`, 1..63 chars, start/end alphanumeric).
  - Maximum resource name length is constrained by Kubernetes (≤253). Given the fixed separator `--cfg-`/`--sec-` length is 6, the theoretical maximum for a full name is: `len(appName) + 1 + len(componentName) + 6 + len(configName)` ≤ 253. Practically, enforce `configName`/`secretName` ≤ 63 and recommend `appName`/`componentName` be chosen to keep the total under 253. A safe upper bound can be expressed as: `maxTotal = 253`, so `len(configName|secretName) ≤ 253 - 6 - 1 - len(appName) - len(componentName)`.
- Target defaults and shorthand ([Docker Configs], [Docker Secrets])
  - When a service reference omits `target`:
    - configs default `target`: `/<configName>`
    - secrets default `target`: `/run/secrets/<secretName>`
  - Shorthand lists are supported: e.g., `configs: [myconfig]`, `secrets: [mysecret]`.

3) Volumes policy (simplified and deterministic)
- `volumes` bind mounts are directory-only:
  - Relative bind `./sub/dir:/mount` → PVC subPath mount using `app.volumes[0]` with `subPath=sub/dir`
  - Absolute bind `/host:/mount` → error
  - If a bind refers to a single file (detected by comparing with declared configs/secrets targets), treat as a configuration error:
    - When the same `target` is present in `services.<svc>.configs` or `services.<svc>.secrets`, ignore the conflicting `volumes` entry and emit a warning (for docker compose compatibility).
    - Otherwise: error with remediation hint “use configs/secrets for single-file mounts”.
- Named volumes `name:/mount` and `name/sub/path:/mount` keep existing PVC rules.

- Note: As a guardrail only, when the relative source path actually exists on disk and is a regular file, the converter rejects the bind as a single-file bind (with the same remediation hint). If the path does not exist, we treat it as a directory (in line with docker compose behavior). This is the only runtime filesystem check introduced for bind validation.

4) Conflict resolution (target path wins)
- If both `volumes` and `configs/secrets` attempt to write the same `target` path for a service:
  - Prefer `configs/secrets` and ignore the `volumes` entry with a warning.
  - Multiple configs/secrets mapping to the same `target` is an error.

5) Compose-first, minimal runtime probing
- The conversion outcome is decided by `compose.yml` declarations. We avoid runtime filesystem classification for deciding “file vs directory” except for the guardrail noted in (3) to prevent accidental single-file binds.
- Files referenced by `configs/secrets` are read to embed content and validate size/encoding constraints.

## Alternatives Considered

- Auto-detect from `volumes` bind whether the source is a file or directory and map files to ConfigMap/Secret automatically
  - Rejected: conversion would depend on local filesystem state, undermining determinism and portability. Also adds surprising magic.
- Introduce custom `x-kompox` hints to annotate `volumes` entries with desired mapping
  - Rejected: increases schema surface and cognitive load; standard `configs/secrets` already convey intent.
- Keep allowing single-file binds under `volumes`
  - Rejected: ambiguous semantics and poorer mapping to Kubernetes; single-file config belongs to ConfigMap/Secret.

## Consequences

- Pros
  - Clear, portable contract: directories via PVC, single files via ConfigMap/Secret.
  - Aligns with Compose and Kubernetes idioms; easier mental model and validation.
  - Deterministic conversion based on compose file only; no hidden runtime heuristics.
- Cons/Constraints
  - Compose files that relied on single-file binds under `volumes` must be updated to use `configs/secrets` (Kompox has no current users, so acceptable).
  - ConfigMap text constraints (UTF-8, no NUL, ≤1 MiB) must be documented; binary or large content must use `secrets` or PVC.

## Rollout

1) Converter implementation (`adapters/kube`)
- Parse `configs/secrets` with compose-go and generate corresponding ConfigMap/Secret manifests and mounts.
- Enforce directory-only for `volumes` bind; detect conflicts by `target` and prefer configs/secrets.
- Apply content-hash annotations (`kompox.dev/compose-content-hash`) to both ConfigMap and Secret using `ComputeContentHash`. Propagate file mode from service references (`mode` → item mode; mounts are readOnly), and generate the corresponding `volumes` entries with `items[{key,path,mode?}]`.

2) Documentation
- Update `design/v1/Kompox-KubeConverter.ja.md` to reflect the new rules: directory-only binds, single-file via configs/secrets, size/encoding limits, conflict resolution, names, and annotations.

3) Tests
- Add unit tests for: configs→ConfigMap, secrets→Secret (data vs binaryData), target conflict resolution, bind file error, directory bind PVC behavior.

4) Migration note
- The annotation name for Secrets is changed from `kompox.dev/compose-secret-hash` to `kompox.dev/compose-content-hash`.
- Deploy-time behavior: new Converter outputs only the new annotation. Existing clusters may still have Secrets annotated with the old key; these will not be updated until re-deploy.
- Operators should re-deploy affected apps to refresh annotations. No functional impact is expected beyond annotation name change; any tooling relying on the old key must be updated.

## Acceptance criteria (implemented)

- Compose `configs` become ConfigMaps and are mounted as single files with `subPath`, `readOnly`, and `mode` applied.
- Compose `secrets` become Opaque Secrets; valid UTF-8 without NUL goes to `data`, otherwise to `binaryData`.
- All generated ConfigMaps/Secrets are annotated with `kompox.dev/compose-content-hash` computed deterministically from content.
- Shorthand and default targets are supported: `/<configName>` for configs, `/run/secrets/<secretName>` for secrets.
- Duplicate `target` among configs/secrets of a service is an error.
- When `volumes` and configs/secrets collide on the same `target`, prefer configs/secrets; ignore the volume with a warning, and remove the conflicting mount from the pod spec.
- `volumes` are directory-only: absolute binds are errors; existing single-file relative binds are rejected with guidance; non-existing paths are treated as directories.
- Existing env_file and imagePullSecrets behaviors remain consistent with the converter guide.
- Unit tests cover conflicts, defaults, volumes policy, content-hash functions, and end-to-end happy paths; CI passes.


## References

- ADRs
  - [K4x-ADR-003] (naming and volume constraints)
- Design docs
  - [Kompox-KubeConverter.ja.md][KubeConverter-ja]
- Tasks
  - [2025-10-10-configs-secrets]
- Docker docs
  - [Docker Configs]
  - [Docker Secrets]

[K4x-ADR-003]: ./K4x-ADR-003.md
[KubeConverter-ja]: ../v1/Kompox-KubeConverter.ja.md
[2025-10-10-configs-secrets]: ../../_dev/tasks/2025-10-10-configs-secrets.md
[Docker Configs]: https://docs.docker.com/engine/swarm/configs/
[Docker Secrets]: https://docs.docker.com/engine/swarm/secrets/
