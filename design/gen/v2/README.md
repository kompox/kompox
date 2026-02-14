# Design Docs (v2) Index

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

Design docs (v2 - future) are the next iteration of comprehensive specifications that build upon the v1 docs. They aim to provide clearer structure, better maintainability, and more actionable guidance for implementation. Use the YAML front matter fields to ensure consistent indexing.

Status legend:
- draft: No implementation yet or still under discussion
- synced: Implementation exists and document reflects it correctly
- out-of-sync: Implementation exists but document needs updates
- archived: Kept as historical reference; no longer maintained
