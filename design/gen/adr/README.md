# ADR Index

Updated: {{ .Updated }}

Guides:
- [GUIDE.md](./GUIDE.md)
- [GUIDE.ja.md](./GUIDE.ja.md)

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

- proposed: Under discussion and not yet accepted
- accepted: Decided and in effect
- rejected: Decided not to implement
- deprecated: No longer recommended; kept for historical reference
