---
title: Kompox 設計ドキュメント目次
version: meta
status: draft
updated: {{ .Updated }}
language: {{ .Language }}
labels:
  groups:
    v1: "v1（現行 CLI 実装）"
    v2: "v2（将来 PaaS/Operator 設計）"
    pub: "公開資料（参考）"
    other: "その他"
---

# Kompox 設計ドキュメント目次

本ディレクトリは Kompox の設計・計画ドキュメントの正本です。v1 は現行 CLI 実装、v2 は将来の PaaS/Operator 設計です。

{{- range .Groups }}

## {{ .Name }}

| タイトル | 言語 | バージョン | ステータス | 最終更新日 |
|---|---|---|---|---|
{{- range .Docs }}
| [{{ .Title }}]({{ .RelPath }}) | {{ .Language }} | {{ .Version }} | {{ .Status }} | {{ .Updated }} |
{{- end }}

{{- end }}

## ステータスの意味

- draft: 実装が存在しない、もしくは検討段階のドラフト
- synced: 実装が存在し、文書がその実装内容を正しく反映
- out-of-sync: 実装は存在するが、文書が追随しておらず更新が必要
- archived: 古い参考資料として保管し、今後は更新しない

