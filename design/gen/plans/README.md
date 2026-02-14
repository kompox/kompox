# Plans Index

| ID | Title | Updated | Status |
| --- | --- | --- | --- |
{{- if .Docs }}
{{- range .Docs }}
| [{{ .ID }}]({{ .RelPath }}) | {{ .Title }} | {{ .Updated }} | {{ .Status }} |
{{- end }}
{{- else }}
| - | No documents | - | - |
{{- end }}

Updated: {{ .Updated }}

---

Plans are higher-level documents that outline the roadmap and strategy for features and components. They are more comprehensive than tasks and may include multiple milestones or phases. Use the YAML front matter fields to ensure consistent indexing.

Status legend:
- draft: Not started yet
- active: Work in progress
- done: Completed; kept for history
- superseded: Replaced by a newer plan (kept for history)
- canceled: Stopped intentionally (kept for history)

Guides:
- [GUIDE.md] (English)
- [GUIDE.ja.md] (Japanese)

[GUIDE.md]: ./GUIDE.md
[GUIDE.ja.md]: ./GUIDE.ja.md
