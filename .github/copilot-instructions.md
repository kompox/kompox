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

## Programming Guidelines

The following is a list of Go language idioms and best practices to follow:

- Use `any` instead of `interface{}`.
- Refer to `docs/Kompox-Arch-v1.ja.md` for architecture guidance including package structure, module boundaries, design patterns, and naming conventions.

## Git Guidelines

- Do not commit unless the user asks you to.
- Use descriptive commit messages that accurately reflect the changes made.
