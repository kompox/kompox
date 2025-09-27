---
id: README
title: Kompox Design Document Index
updated: {{ .Updated }}
language: {{ .Language }}
---

# Kompox Design Document Index

This directory holds the canonical design and planning documents for Kompox. v1 is the current CLI implementation; v2 is the future PaaS/Operator design.

{{- /* Centralized helpers: group title and status definitions by group key */}}
{{- /* Order definition (comma separated keys): adjust here to change section order */}}
{{- define "groupOrder" -}}v1, v2, adr, pub, other{{- end }}
{{- define "groupName" -}}
	{{- $k := . -}}
	{{- if eq $k "v1" -}}v1 (Current CLI)
	{{- else if eq $k "v2" -}}v2 (Future PaaS/Operator)
	{{- else if eq $k "adr" -}}ADR
	{{- else if eq $k "pub" -}}Public materials (reference)
	{{- else -}}Others
	{{- end -}}
{{- end }}

{{- define "groupStatusDefs" -}}
	{{- $k := . -}}
	{{- if eq $k "adr" -}}
- proposed: Under discussion and not yet accepted
- accepted: Decided and in effect
- rejected: Decided not to implement
- deprecated: No longer recommended; kept for historical reference
	{{- else if eq $k "pub" -}}
- draft: Planned or under preparation; not yet scheduled
- scheduled: Confirmed for a future date
- delivered: Completed and delivered (slides/article published)
- archived: Historical reference only
	{{- else -}}
- draft: No implementation yet or still under discussion
- synced: Implementation exists and document reflects it correctly
- out-of-sync: Implementation exists but document needs updates
- archived: Kept as historical reference; no longer maintained
	{{- end -}}
{{- end }}

{{- range .Groups }}

## {{ template "groupName" .Key }}

| ID | Title | Language | Version | Status | Last updated |
|---|---|---|---|---|---|
{{- range .Docs }}
| [{{ .ID }}]({{ .RelPath }}) | {{ .Title }} | {{ .Language }} | {{ .Version }} | {{ .Status }} | {{ .Updated }} |
{{- end }}

**Status definitions:**

{{ template "groupStatusDefs" .Key }}

{{- end }}

