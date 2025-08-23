# Repository Copilot Instructions

## Language and Communication Guidelines

This repository prefers English for natural-language communication, with Japanese allowed where noted in the repository instructions.

- Use English by default for comments, PR descriptions, commit messages, and conversational output.
- Japanese may be used when appropriate for files or documentation that are specifically Japanese; see the language guidance files in `.github/instructions` for exact rules and file-scope mappings.
- Repository-level language/instruction rules are defined in `.github/instructions/en.instructions.md` and `.github/instructions/ja.instructions.md`.

## Important Documents and Resources

Always refer to the following documents for guidance:

- `docs/Kompox-Spec-Draft.ja.md` for the project overview and goals. That document is the canonical source for the intended behavior and high-level requirements.
- `docs/Kompox-Arch-v1.ja.md` for the architecture guidance when proposing design changes, new packages, or infra patterns.

## Go Language Programming Guidelines

The following is a list of Go language idioms and best practices to follow:

- Use `any` instead of `interface{}`.
- Refer to `docs/Kompox-Arch-v1.ja.md` for architecture guidance including package structure, module boundaries, design patterns, and naming conventions.

## Generic Source Code Comment Guidelines

Write comments that are timeless, useful, and focused on maintainers or future readers.
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

## Git and Commit Message Guidelines

- Do not commit unless the user asks you to.
- Follow Commit Message Guideline in `.github/prompts/commit-messages.prompt.md`

## Tools guideline

Use `make` for regular tasks because it requires no user intervention.

```bash
# Run full tests (go test ./...)
make test

# Run full build check (go build ./...)
make build

# Build kompoxops CLI executable (go build ./cmd/kompoxops)
make cmd

# Run go mod tidy
make tidy

# Build adapters/drivers/provider/aks/main.json to embed in AKS driver
# You need it when you make changes in infra/aks
make bicep
```
