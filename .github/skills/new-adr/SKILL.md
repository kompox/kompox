---
name: new-adr
description: Create a new ADR file
---
## Purpose

- Create one new ADR document under [design/adr].

## Steps

1) Decide file path and ADR number
  - Standard path: `design/adr/K4x-ADR-NNN.md`
  - Use the next unused sequential number for `NNN`.
2) Draft the document
  - Always write ADR content in English.
  - Follow structure, status rules, and authoring guidance in [GUIDE.md] and [README.md].
  - Refer to existing ADR files for tone and level of detail.
3) Apply front-matter rules
  - Use the fields defined in [GUIDE.md].
  - Set `id` as the ADR doc-id and keep it unique across the entire repository.
  - Keep `id` aligned with the filename stem.
  - Add `tasks`/`plans` only when stable related doc-ids are known; otherwise omit them.
4) Apply references rules
  - Keep a markdown reference list at the end of the file.
  - For links to markdown documents that define front-matter `id:`, use that doc-id as the reference label (no `.md` / `.ja.md` in labels).
  - Use label-style references in the body (for example, `[K4x-ADR-013]`).
5) Finalize
  - Run `make gen-index` after creating the file to refresh ADR indexes.

[design/adr]: ../../../design/adr
[GUIDE.md]: ../../../design/adr/GUIDE.md
[README.md]: ../../../design/adr/README.md
