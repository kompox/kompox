---
id: K4x-ADR-015
title: Adopt project KOMPOX_DIR, KOMPOX_CFG_DIR (.kompox), and Git-like discovery for KOM inputs
status: proposed
date: 2025-11-03
language: en
supersedes: []
supersededBy: []
---
# K4x-ADR-015: Adopt project KOMPOX_DIR, KOMPOX_CFG_DIR (.kompox), and Git-like discovery for KOM inputs

## Context

- The current "baseRoot" and `.kompoxroot` rules are confusing and hard to reason about for users and for tooling.
- We want a Git-like mental model: a repository-local hidden directory that anchors discovery and configuration, and a clear separation between working directory changes and repository configuration.
- We also want an unambiguous contract for where KOM files are read from, how paths are resolved, and how defaults are provided without relying on implicit heuristics.
 - Backward compatibility is not required for this change set; we may repurpose flags and semantics (e.g., reclaim `-C`, remove `baseRoot`/`.kompoxroot`, rename variables) without providing shims.

## Decision

- Introduce the notion of a project directory (project root) and a configuration directory:
  - Project directory: environment variable `KOMPOX_DIR`; CLI flag `--kompox-dir`.
  - Configuration directory: `.kompox/` at the project directory by default; environment variable `KOMPOX_CFG_DIR`; CLI flag `--kompox-cfg-dir`.
  - Repository config file: `.kompox/config.yml` (YAML) for CLI/runtime defaults.
    - Rationale: keep repository configuration clearly separated from KOM content while allowing a concise default location.
- Reserve `-C` exclusively for changing the current working directory before execution (like `git -C`). The intended use is to point to the directory that contains `kompoxapp.yml`.
- Define Git-like discovery anchored by `.kompox/`:
  - If `--kompox-dir`/`KOMPOX_DIR` is set, use it as the project directory; otherwise search upward from the working directory for a parent containing `.kompox/` and use that parent as `KOMPOX_DIR`.
  - If `--kompox-cfg-dir`/`KOMPOX_CFG_DIR` is set, use it as the configuration directory; otherwise default to `$KOMPOX_DIR/.kompox`.
  - If neither is resolvable, error with guidance.
- KOM inputs are read only from the first available source in the following precedence:
  1) `--kom-path` (repeatable; file or directory)
  2) `KOMPOX_KOM_PATH` (OS PathListSeparator)
  3) `kompoxapp.yml` → `Defaults.spec.komPath`
  4) `.kompox/config.yml` → `komPath`
  5) `$KOMPOX_CFG_DIR/kom` (default KOM directory)
- Support `$KOMPOX_DIR` and `$KOMPOX_CFG_DIR` expansion across flags, environment variables, `.kompox/config.yml`, and `Defaults` values.
- Deprecate and remove `baseRoot` and `.kompoxroot`. Reclaim the `-C` short option from `--cluster-id` (do not reuse it for cluster-id).
  
Implementation specifics (discovery steps, resolution rules, path constraints, excludes, limits) are documented in [Kompox-CLI.ja.md].

## Migration
- Add an empty `.kompox/` directory at the project directory; optionally add `.kompox/config.yml` with `store.type: local` and `komPath`.
- Update CLI invocations and scripts:
  - Prefer `-C` to point to the directory containing `kompoxapp.yml` and rely on upward discovery.
  - Use `--kompox-dir` to pin the project directory when discovery is ambiguous; use `--kompox-cfg-dir` only for advanced setups.
  - Replace any prior usage of `baseRoot`/`.kompoxroot`.

## Consequences

- Pros
  - Clear, Git-like UX and simpler mental model for discovery and configuration.
  - Stronger safety by scoping KOM inputs under a single store root.
  - Explicit, composable precedence order enables reproducible runs and easier debugging.
- Cons
  - Minor migration work to add `.kompox/` and adjust flags in scripts.
  - Tools and tests that assumed `baseRoot` must be updated to use `KOMPOX_DIR` and store root constraints.

## Alternatives considered

- Keep `baseRoot` and `.kompoxroot`:
  - Rejected: ambiguous and hard to document/validate; worse UX than Git analogy.
- Use `--kompox-path`/`KOMPOX_PATH` instead of `--kompox-dir`/`KOMPOX_DIR`:
  - Rejected: `PATH` conventionally implies a list; the root concept is a single directory. Git analogy favors `DIR`.
- Name `.kompox/defaults.yml` for repository settings:
  - Rejected: confusable with the `Defaults` pseudo-resource kind; `config.yml` is clearer and matches Git.

## References

- [K4x-ADR-009]
- [K4x-ADR-011]
- [K4x-ADR-012]
- [Kompox-KOM.ja.md]
- [Kompox-CLI.ja.md]
- [_dev/tasks/2025-10-17-defaults.ja.md]

[K4x-ADR-009]: ./K4x-ADR-009.md
[K4x-ADR-011]: ./K4x-ADR-011.md
[K4x-ADR-012]: ./K4x-ADR-012.md
[Kompox-KOM.ja.md]: ../v1/Kompox-KOM.ja.md
[Kompox-CLI.ja.md]: ../v1/Kompox-CLI.ja.md
[_dev/tasks/2025-10-17-defaults.ja.md]: ../../_dev/tasks/2025-10-17-defaults.ja.md
