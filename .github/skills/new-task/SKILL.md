---
name: new-task
description: Create a new task file
---
## Purpose

- Create one new task document under [design/tasks].

## Steps

1) Decide file path
  - Standard paths:
    - `design/tasks/YYYY/MM/DD/YYYYMMDDa-short-slug.md` (English)
    - `design/tasks/YYYY/MM/DD/YYYYMMDDa-short-slug.ja.md` (Japanese)
  - Default language is Japanese.
  - Follow the user’s instructions if they specify path, date, language, or slug.
2) Draft the document
  - Use [GUIDE.md] for English tasks or [GUIDE.ja.md] for Japanese tasks.
  - Follow the schema and template defined in the applicable guide.
  - Refer to existing task files for style and granularity.
3) Apply front-matter rules
  - Set `id` as the task doc-id and keep it unique across the entire repository.
  - Keep `id` aligned with the filename stem.
4) Apply references rules
  - Keep a markdown reference list at the end of the file.
  - For links to markdown documents that define front-matter `id:`, use that doc-id as the reference label (no `.md` / `.ja.md` in labels).
  - Use label-style references in the body (for example, `[K4x-ADR-014]`).
5) Finalize
  - Run `make gen-index` after creating the file to refresh task indexes.

[design/tasks]: ../../../design/tasks
[GUIDE.md]: ../../../design/tasks/GUIDE.md
[GUIDE.ja.md]: ../../../design/tasks/GUIDE.ja.md
