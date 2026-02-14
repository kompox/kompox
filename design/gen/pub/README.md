# Public Docs Index

Updated: {{ .Updated }}

| ID | Title | Updated | Status |
| --- | --- | --- | --- |
{{- if .Docs }}
{{- range .Docs }}
| [{{ .ID }}]({{ .RelPath }}) | {{ .Title }} | {{ .Updated }} | {{ .Status }} |
{{- end }}
{{- else }}
| - | No documents | - | - |
{{- end }}

**Status definitions:**

- draft: Planned or under preparation; not yet scheduled
- scheduled: Confirmed for a future date
- delivered: Completed and delivered (slides/article published)
- rejected: Proposal was reviewed and not adopted
- archived: Historical reference only
