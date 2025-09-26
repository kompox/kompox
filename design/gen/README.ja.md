---
id: README
title: Kompox 設計ドキュメント目次
updated: {{ .Updated }}
language: {{ .Language }}
---

# Kompox 設計ドキュメント目次

本ディレクトリは Kompox の設計・計画ドキュメントの正本です。v1 は現行 CLI 実装、v2 は将来の PaaS/Operator 設計です。

{{- /* 集中管理されたヘルパー: グループ名とステータス説明（グループ別） */}}
{{- /* 並び順定義（カンマ区切りキー）: セクション順を変えたい場合はここを編集 */}}
{{- define "groupOrder" -}}v1, v2, adr, pub, other{{- end }}
{{- define "groupName" -}}
	{{- $k := . -}}
	{{- if eq $k "v1" -}}v1（現行 CLI 実装）
	{{- else if eq $k "v2" -}}v2（将来 PaaS/Operator 設計）
	{{- else if eq $k "adr" -}}ADR
	{{- else if eq $k "pub" -}}公開資料（参考）
	{{- else -}}その他
	{{- end -}}
{{- end }}

{{- define "groupStatusDefs" -}}
	{{- $k := . -}}
	{{- if eq $k "adr" -}}
- proposed: 検討中で、まだ採択されていない
- accepted: 採択済みで有効
- rejected: 採択せず不採用
- deprecated: 推奨しない（歴史的経緯として残置）
- superseded: 新しい ADR によって置き換え
	{{- else if eq $k "pub" -}}
- draft: 企画・準備段階で未確定
- scheduled: 実施予定が確定
- delivered: 実施完了（登壇/公開済み）
- archived: 参考資料として保管
	{{- else -}}
- draft: 実装が存在しない、もしくは検討段階のドラフト
- synced: 実装が存在し、文書がその実装内容を正しく反映
- out-of-sync: 実装は存在するが、文書が追随しておらず更新が必要
- archived: 古い参考資料として保管し、今後は更新しない
	{{- end -}}
{{- end }}

{{- range .Groups }}

## {{ template "groupName" .Key }}

| ID | Title | Language | Version | Status | Last updated |
|---|---|---|---|---|---|
{{- range .Docs }}
| [{{ .ID }}]({{ .RelPath }}) | {{ .Title }} | {{ .Language }} | {{ .Version }} | {{ .Status }} | {{ .Updated }} |
{{- end }}

**ステータスの意味:**

{{ template "groupStatusDefs" .Key }}

{{- end }}

