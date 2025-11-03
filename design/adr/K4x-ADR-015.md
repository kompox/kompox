---
id: K4x-ADR-015
title: Introduce Kompox CLI Env with Git-like discovery
status: accepted
date: 2025-11-03
language: en
supersedes: []
supersededBy: []
---
# K4x-ADR-015: Introduce Kompox CLI Env with Git-like discovery

## Context

- The current "baseRoot" and `.kompoxroot` rules are confusing and hard to reason about for users and for tooling.
- We want a Git-like mental model: a repository-local hidden directory that anchors discovery and configuration, and a clear separation between working directory changes and repository configuration.
- We need an unambiguous contract for where KOM files are read from, how paths are resolved, and how defaults are provided without relying on implicit heuristics.
- Prior ADRs define KOM mode architecture and Defaults pseudo-resource. [K4x-ADR-009] [K4x-ADR-012]
- Backward compatibility is not required for this change set; we may repurpose flags and semantics (e.g., reclaim `-C`, remove `baseRoot`/`.kompoxroot`, rename variables) without providing shims.

## Decision

Introduce **Kompox CLI Env**, a Git-inspired project environment system centered around the `.kompox/` directory.

### Core Concepts

- **Project Directory (`KOMPOX_ROOT`)**: The root of a Kompox project, analogous to a Git repository root. Discovered by searching upward for a parent directory containing `.kompox/`. Can be explicitly set via `--kompox-root` flag or `KOMPOX_ROOT` environment variable.
- **Kompox Directory (`KOMPOX_DIR`)**: The directory containing Kompox CLI configuration. Defaults to `$KOMPOX_ROOT/.kompox`. Can be overridden via `--kompox-dir` flag or `KOMPOX_DIR` environment variable. This directory will also serve as the standard location for logs, cache, and other CLI-managed data in future versions.
- **Repository Configuration (`.kompox/config.yml`)**: YAML file for project-level CLI settings (store backend, default KOM paths, etc.).

### Discovery Rules

1. Apply `-C, --chdir` flag: Change working directory before any other processing (like `git -C`)
2. Resolve `KOMPOX_ROOT`: Priority is `--kompox-root` > `KOMPOX_ROOT` env var > upward search for `.kompox/`
3. Resolve `KOMPOX_DIR`: Priority is `--kompox-dir` > `KOMPOX_DIR` env var > `$KOMPOX_ROOT/.kompox`
4. Export to environment: Set `KOMPOX_ROOT` and `KOMPOX_DIR` as environment variables

### KOM Input Precedence

KOM files are read from **only the first available source** in this order:
1. `--kom-path` flag (repeatable; file or directory)
2. `KOMPOX_KOM_PATH` environment variable (OS-specific path separator)
3. `Defaults.spec.komPath` in `kompoxapp.yml` (with boundary check: must be within `$KOMPOX_ROOT` or `$KOMPOX_DIR`)
4. `komPath` in `.kompox/config.yml`
5. Default: `$KOMPOX_DIR/kom` directory

Single source selection principle: no merging across precedence levels.

### Variable Expansion

The strings `$KOMPOX_ROOT` and `$KOMPOX_DIR` are expanded in CLI flag values, environment variables, `.kompox/config.yml` fields, and `Defaults.spec.komPath` values.

### Project Initialization

New `kompoxops init` command bootstraps the Kompox CLI Env:
- Creates `.kompox/` directory structure
- Generates `.kompox/config.yml` with default configuration:
  ```yaml
  version: 1
  store:
    type: local
  komPath:
    - kom
  ```
- Creates `.kompox/kom/` as the default KOM storage location
- Supports `-C, --chdir <DIR>` to initialize in specified directory (creates parents if needed)
- Supports `-f, --force` to overwrite existing configuration

**Future extensions**: The `.kompox/` directory is designed to accommodate additional CLI-managed subdirectories such as:
- `.kompox/logs/` for standard log output
- `.kompox/cache/` for temporary artifacts and caches
- `.kompox/tmp/` for transient working files
These extensions will be added as needed while maintaining backward compatibility with the core structure.

### Breaking Changes

- Remove `baseRoot` and `.kompoxroot` concepts
- Reclaim `-C` flag from `--cluster-id` (use full `--cluster-id` flag instead)
- Remove implicit `.git` fallback for discovery
- Enforce single-source KOM input (no merging behavior)

Implementation details (path constraints, directory excludes, safety limits) are documented in [Kompox-CLI.ja.md]. Implementation tracking in [2025-11-03-kompox-cli-env.ja.md].

## Alternatives Considered

- **Keep `baseRoot` and `.kompoxroot`**: Rejected due to ambiguity and poor UX compared to Git analogy.
- **Use `KOMPOX_PATH` instead of `KOMPOX_DIR`**: Rejected because `PATH` conventionally implies a list; project root is a single directory. Git analogy favors `DIR`.
- **Name config file `.kompox/defaults.yml`**: Rejected to avoid confusion with `Defaults` pseudo-resource kind. `config.yml` matches Git's `config`.
- **Support multiple project roots**: Deferred for complexity. Current design covers most use cases with one root per invocation.
- **Merge KOM sources across precedence**: Rejected for unpredictability. Single-source selection is simpler and explicit.

## Consequences

### Pros
- Clear, Git-like UX with simpler mental model for discovery and configuration
- Stronger safety by scoping KOM inputs and enforcing boundaries
- Explicit precedence order enables reproducible runs and easier debugging
- Easy initialization with `kompoxops init` provides zero-friction setup
- Composable commands with `-C` enable directory changes without affecting project discovery
- Better tooling support due to explicit discovery rules
- Extensible structure: `.kompox/` can accommodate future additions (logs, cache, etc.) without breaking changes

### Cons
- Migration effort required for existing projects (add `.kompox/`, adjust flag usage)
- Breaking changes to CLI flags and semantics (no backward compatibility)
- Tools and tests that assumed `baseRoot` must be updated

## Migration

### For New Projects
```bash
# Initialize in current directory
kompoxops init

# Initialize in specific directory
kompoxops init -C /path/to/new-project
```

### For Existing Projects
1. Add `.kompox/` directory: `kompoxops init`
2. Update scripts: Change `-C` usage (now means "change directory"), remove `baseRoot` references
3. Configure KOM paths in `.kompox/config.yml` if needed

### Example
**Before** (using baseRoot):
```bash
cd /project/app1
export KOMPOX_BASE_ROOT=/project
kompoxops app deploy
```

**After** (using Kompox CLI Env):
```bash
kompoxops init -C /project  # One-time setup
cd /project/app1
kompoxops app deploy
# Or: kompoxops -C /project/app1 app deploy
```

## References

- [K4x-ADR-009]: KOM Mode and Repository Architecture
- [K4x-ADR-011]: Kompox Object Model (KOM) Validation
- [K4x-ADR-012]: Defaults Pseudo-Resource and App.RefBase
- [Kompox-CLI.ja.md]: Comprehensive CLI specification
- [Kompox-KOM.ja.md]: KOM file format and resolution
- [2025-11-03-kompox-cli-env.ja.md]: Implementation task tracking

[K4x-ADR-009]: ./K4x-ADR-009.md
[K4x-ADR-011]: ./K4x-ADR-011.md
[K4x-ADR-012]: ./K4x-ADR-012.md
[Kompox-CLI.ja.md]: ../v1/Kompox-CLI.ja.md
[Kompox-KOM.ja.md]: ../v1/Kompox-KOM.ja.md
[2025-11-03-kompox-cli-env.ja.md]: ../../_dev/tasks/2025-11-03-kompox-cli-env.ja.md
