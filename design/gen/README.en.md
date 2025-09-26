---
title: Kompox Design Document Index
version: meta
status: draft
updated: {{ .Updated }}
language: {{ .Language }}
labels:
  groups:
    v1: "v1 (Current CLI)"
    v2: "v2 (Future PaaS/Operator)"
    pub: "Public materials (reference)"
    other: "Others"
---

# Kompox Design Document Index

This directory holds the canonical design and planning documents for Kompox. v1 is the current CLI implementation; v2 is the future PaaS/Operator design.

{{- range .Groups }}

## {{ .Name }}

| Title | Language | Version | Status | Last Updated |
|---|---|---|---|---|
{{- range .Docs }}
| [{{ .Title }}]({{ .RelPath }}) | {{ .Language }} | {{ .Version }} | {{ .Status }} | {{ .Updated }} |
{{- end }}

{{- end }}

## Status definitions

- draft: No implementation yet or still under discussion
- synced: Implementation exists and document reflects it correctly
- out-of-sync: Implementation exists but document needs updates
- archived: Kept as historical reference; no longer maintained

