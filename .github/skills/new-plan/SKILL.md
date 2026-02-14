---
name: new-plan
description: Create a new plan file
---
## Purpose

- Create one new plan document under [design/plans].

## Steps

1) Decide file path
  - Standard paths:
    - `design/plans/YYYY/YYYYaa-short-slug.md` (English)
    - `design/plans/YYYY/YYYYaa-short-slug.ja.md` (Japanese)
  - Default language is Japanese.
  - Follow the user’s instructions if they specify path, date, language, or slug.
2) Draft the document
  - Use [GUIDE.md] for English plans or [GUIDE.ja.md] for Japanese plans.
  - Follow the schema and template defined in the applicable guide.
  - Refer to existing plan files for structure and writing style.
3) Apply front-matter rules
  - Set `id` as the plan doc-id and keep it unique across the entire repository.
  - Keep `id` aligned with the filename stem.
  - Set `adrs` as a string array of related ADR doc-ids (use `[]` when none yet).
  - Keep `tasks` as a string array of related task doc-ids when applicable.
4) Apply references rules
  - Keep a markdown reference list at the end of the file.
  - For links to markdown documents that define front-matter `id:`, use that doc-id as the reference label (no `.md` / `.ja.md` in labels).
5) Finalize
  - Run `make gen-index` after creating the file to refresh plan indexes.

[design/plans]: ../../../design/plans
[GUIDE.md]: ../../../design/plans/GUIDE.md
[GUIDE.ja.md]: ../../../design/plans/GUIDE.ja.md
