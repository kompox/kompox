# Public Docs Index

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

Public docs are materials that have been shared outside the organization, such as conference talks, blog posts, or articles. They serve as a record of our public communication and can be useful for reference and learning. Use the YAML front matter fields to ensure consistent indexing.

Status legend:
- draft: Planned or under preparation; not yet scheduled
- scheduled: Confirmed for a future date
- delivered: Completed and delivered (slides/article published)
- rejected: Proposal was reviewed and not adopted
- archived: Historical reference only
