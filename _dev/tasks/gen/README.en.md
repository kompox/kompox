---
id: README
title: Developer Tasks Index
updated: {{ .Updated }}
language: {{ .Language }}
---

# Developer Tasks Index

This index lists short, action-oriented developer tasks found in this folder. Tasks are grouped by year (based on the `updated` date or `id` prefix).

- Developer's guide: [GUIDE.en.md](./GUIDE.en.md)
- 日本語版: [README.ja.md](./README.ja.md)

{{- /* Section: Year groups */}}
{{- range .Groups }}

## {{ .Key }}

| ID | Title | Status | Category | Owner | Updated | Language |
|---|---|---|---|---|---|---|
{{- range .Docs }}
| [{{ .ID }}]({{ .RelPath }}) | {{ .Title }} | {{ .Status }} | {{ .Category }} | {{ .Owner }} | {{ .Updated }} | {{ .Language }} |
{{- end }}

{{- end }}

---

Status legend:
- active: Work in progress
- blocked: Waiting on dependency or decision
- done: Completed; kept for history
- canceled: Stopped intentionally
- superseded: Replaced by a newer task

Notes:
- Tasks are intentionally short and specific. Decisions should be captured in ADRs and specifications.
- Use the YAML front matter fields to ensure consistent indexing.
