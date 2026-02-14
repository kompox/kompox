# ADR Index

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

ADRs (Architecture Decision Records) are used to document significant architectural decisions and their rationale. Each ADR should be concise and focused on a single decision. Use the YAML front matter fields to ensure consistent indexing.

Status legend:
- proposed: Under discussion and not yet accepted
- accepted: Decided and in effect
- rejected: Decided not to implement
- deprecated: No longer recommended; kept for historical reference

Guides:
- [GUIDE.md] (English)
- [GUIDE.ja.md] (Japanese)

[GUIDE.md]: ./GUIDE.md
[GUIDE.ja.md]: ./GUIDE.ja.md
