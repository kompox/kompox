# Contribution and Language Policy

- Code, comments, commit messages, PR titles/descriptions: English only.
- Documentation under `docs/`: Japanese is allowed (existing policy); keep markdown there in Japanese unless specified otherwise.
- User-facing CLI messages: English by default. Japanese may appear only in `docs/`.
- Variable, function, and package names: English.

## Rationale
Consistency improves readability and reduces ambiguity. Japanese is reserved for end-user documents in `docs/` to serve the intended audience. Everything else remains in English for maintainability.

## Examples
- Good: `// Validate RFC1123 label`
- Avoid: `// RFC1123 ラベルを検証`

## Notes for AI assistants (Copilot, etc.)
- Generate code and comments in English.
- If you need to cite or summarize content from `docs/`, keep the code/comments in English while referencing the Japanese text as needed.
- When adding new documentation pages, prefer English unless it's explicitly under `docs/` where Japanese is acceptable.
