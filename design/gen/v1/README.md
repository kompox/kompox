# Design Docs (v1) Index

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

Design docs (v1 - current) are comprehensive specifications that describe the design and implementation details of features and components. They are intended to be living documents that evolve with the project. Use the YAML front matter fields to ensure consistent indexing.

Status legend:
- draft: No implementation yet or still under discussion
- synced: Implementation exists and document reflects it correctly
- out-of-sync: Implementation exists but document needs updates
- archived: Kept as historical reference; no longer maintained
