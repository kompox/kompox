# Tasks Index

Updated: {{ .Updated }}

This index lists short, action-oriented developer tasks found in this folder.

| ID | Title | Updated | Status |
| --- | --- | --- | --- |
{{- if .Docs }}
{{- range .Docs }}
| [{{ .ID }}]({{ .RelPath }}) | {{ .Title }} | {{ .Updated }} | {{ .Status }} |
{{- end }}
{{- else }}
| - | No documents | - | - |
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
