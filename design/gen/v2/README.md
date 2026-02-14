# V2 Specs Index

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

- draft: No implementation yet or still under discussion
- synced: Implementation exists and document reflects it correctly
- out-of-sync: Implementation exists but document needs updates
- archived: Kept as historical reference; no longer maintained
