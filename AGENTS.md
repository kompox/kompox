# Guidelines for Agents

## Makefile targets (recommended)

If available, prefer these targets over running raw `go` commands (no user intervention needed):

```bash
make build
make test
make tidy
make bicep
make git-diff-cached
make git-commit-with-editor
make git-show
make gen-index
```

## Docs for developers and maintainers

All documents are maintained in markdown files under the `design` directory, organized by type.

Each document type has an index (README.md). Some types also have guides (GUIDE.md and GUIDE.ja.md) with instructions and templates.

|Type|Index|Guide|Description|
|-|-|-|-|
|ADR|[README.md](./design/adr/README.md)|[GUIDE.md](./design/adr/GUIDE.md)|Architectural Decision Records (ADRs) for design decisions|
|Design Docs (v1)|[README.md](./design/v1/README.md)|[GUIDE.md](./design/v1/GUIDE.md)|Design documents for features and components|
|Design Docs (v2)|[README.md](./design/v2/README.md)|-|Future design documents and roadmaps|
|Plans|[README.md](./design/plans/README.md)|[GUIDE.md](./design/plans/GUIDE.md)|Higher-level plans and roadmaps|
|Tasks|[README.md](./design/tasks/README.md)|[GUIDE.md](./design/tasks/GUIDE.md)|Short-term tasks with implementation details|
|Public Docs|[README.md](./design/pub/README.md)|-|Documents intended to be shared outside the organization|

Each document begins with YAML front matter.

Common fields (see each type's GUIDE.md for the full schema):

- id: Unique identifier for the document, equal to the filename without the extension
  - ADRs: `K4x-ADR-NNN` (use sequential numbers for NNN)
  - Design docs: `Kompox-<short-slug>`
  - Plan files: `YYYYaa-<short-slug>` (use `aa`, `ab`, ... to disambiguate multiple plans in the same year)
  - Task files: `YYYYMMDDa-<short-slug>` (use `a`, `b`, ... to disambiguate multiple tasks on the same date)
  - User may refer to plans by `YYYYaa` and tasks by `YYYYMMDDa` when the short slug is not important.
- title: Title of the document
- status: Document status (see GUIDE.md)
- updated: Last updated timestamp (UTC, ISO 8601: YYYY-MM-DDTHH:MM:SSZ)
  - bash: `date -u +"%Y-%m-%dT%H:%M:%SZ"`
  - pwsh: `(Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ss'Z'")`
- language: `en | ja`

Each document should have a "References" section at the end with the following format:

- Use reference-style links for cross-document references.
- The References section should list link reference IDs (descriptions are optional).
- The actual link definitions should follow the list in the References section: `[ref-id]: ../relative/path/to/doc.md`
- Use `[ref-id]` in the document body to refer to linked documents without restating their content.

For example:

```markdown
## Overview

This document describes the design for Feature X, which is based on the decision made in [K4x-ADR-001] and is related to the implementation details in [Kompox-DesignDoc-FeatureY].

## References

- [K4x-ADR-001] - ADR for choosing Go as the implementation language
- [Kompox-DesignDoc-FeatureY] - Design doc for Feature Y

[K4x-ADR-001]: ../adr/K4x-ADR-001.md
[Kompox-DesignDoc-FeatureY]: ../v1/Kompox-DesignDoc-FeatureY.md
```

## Agent Skills (recommended)

Some agent skills are defined in `.github/skills` and you should use them when applicable. For example:

- `git-commit` - The user will ask you to run this skill after staging changes to commit. You should review changes with `make git-diff-cached`, write commit message candidates to the suggested `_tmp/git-commit/NNNN.txt`, then run `make git-commit-with-editor` so the user can pick/edit the final message before committing.
- `new-adr` - Use this skill to create a new Architectural Decision Record (ADR).
- `new-task` - Use this skill to create a new task file.

## Language and Communication Guidelines

This repository prefers English for natural-language communication. Japanese is used where the document itself is Japanese.

- Use English by default for comments, PR descriptions, commit messages, and conversational output.
- For design documents, use the filename suffix and `language` front matter to indicate Japanese (`.ja.md`, `language: ja`).

## Go Language Programming Guidelines

The following is a list of Go language idioms and best practices to follow:

- Use `any` instead of `interface{}`.
- Refer to `design/v1/Kompox-Arch-Implementation.ja.md` for architecture guidance including package structure, module boundaries, design patterns, and naming conventions.

## Generic Source Code Comment Guidelines

Write comments that are timeless, useful, and focused on developers or future readers.
Do not include historical, temporary, or meta information.

DO NOT:
- Do NOT use time-relative phrases like:
  - "recently", "as of now", "temporary", "after refactor", "new spec", etc.
- Do NOT include:
  - Old values, new places ("A is moved to B"), change histories.  Use Git commit messages instead.
  - Mentions what's told in prompts or discussions with users
- Do NOT restate what the code or logging/diagnostic message already says.

DO:
- Explain **why** the code exists, not just what it does.
- Document domain rules, invariants, and non-obvious constraints.
- Use TODO only with:
  - concrete action
  - owner (if known)
  - condition or trigger
