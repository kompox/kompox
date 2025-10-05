---
id: 2025-09-30-cluster-dns
title: Cluster DNS 適用 API の追加(Driver/Domain)
status: done
updated: 2025-10-06
language: ja
supersedes: []
---
# Task: Cluster DNS 適用 API の追加(Driver/Domain)

## 目的

- クラスターの Ingress エンドポイントを指す FQDN の DNS レコードをプロバイダ管理の DNS ゾーンへ自動設定できる仕組みを導入する。
- 任意のレコードタイプ/値を扱える、汎用的な「レコードセット適用」API を提供する。
- 既定はベストエフォートかつ冪等(Ensure ではなく Apply の意味合い)。Strict オプションで失敗をエラー化可能にする。

## スコープ / 非スコープ

- In:
  - Domain に DNS モデル型(`DNSRecordType`/`DNSRecordSet`)を追加。
  - ClusterPort に `DNSApply` を追加。DNS オプション(`ClusterDNSApplyOptions`)とそのヘルパーは `cluster_port.go` に集約。
  - Provider Driver インターフェースへ `ClusterDNSApply` を追加(ベストエフォートでドライバに委譲)。
  - AKS/K3s ドライバにスタブ実装を追加(現状は no-op/ログのみ)。
  - Provider ClusterPort アダプタからドライバ `ClusterDNSApply` へ委譲実装を追加。
- Out:
  - 各クラウド DNS への実処理実装(例: Azure DNS への Upsert)。
  - CLI のコマンド/フラグ追加。
  - 複数レコードセットの一括適用 API(現状は 1 呼び出しにつき単一 rset)。

## 仕様サマリ

- Driver メソッド(新規)
  - `ClusterDNSApply(ctx context.Context, cluster *model.Cluster, rset model.DNSRecordSet, opts ...model.ClusterDNSApplyOption) error`
  - 役割: レコードセット単位(FQDN × Type)の作成/更新/削除をベストエフォートで適用。
  - 返り値: 基本は `error` のみ。引数不正や `ctx` キャンセルなどは `error` を返す。権限不足などの書き込み失敗は既定ではエラーにしない(Strict 時のみエラー)。

- Domain 型
  - `DNSRecordType` ("A"/"AAAA"/"CNAME"/"TXT"/"MX"/"NS"/"SRV"/"CAA").
  - `DNSRecordSet{ FQDN string, Type DNSRecordType, TTL uint32, RData []string }`
    - `TTL == 0` はプロバイダ既定 TTL を意味する。
    - `RData` が空(`len==0`)の場合は「レコードセット削除」の意図と解釈可能。

- Domain オプション(ClusterPort 用)
  - `ClusterDNSApplyOptions{ ZoneHint string, Strict bool, DryRun bool }`
  - ヘルパー: `WithClusterDNSApplyZoneHint(string)`, `WithClusterDNSApplyStrict()`, `WithClusterDNSApplyDryRun()`
  - Zone の自動選択に迷う場合は `ZoneHint` を優先。Strict 時は書き込み失敗を `error` で返す。DryRun は変更を行わない。

- ClusterPort(Domain ポート)
  - `DNSApply(ctx context.Context, cluster *Cluster, rset DNSRecordSet, opts ...ClusterDNSApplyOption) error`
  - Provider Driver へ委譲。

- 期待されるドライバ実装の挙動(ガイドライン)
  - 冪等な Upsert/Delete。A/AAAA/CNAME 等の RDATA は `RData` に文字列のまま受ける。
  - 対象ゾーンが不明な場合は `ZoneHint` を利用し、未指定時は最長一致等で推定。
  - ベストエフォート(既定): 書き込み失敗はワーニング扱いで `error` は返さない。
  - Strict: 書き込み失敗を `error` とする。

## 計画 (チェックリスト)

- [x] Domain: DNS 型を追加
  - [x] `domain/model/dns.go`: `DNSRecordType`/`DNSRecordSet` を定義
- [x] Domain: ClusterPort を拡張
  - [x] `domain/model/cluster_port.go`: `DNSApply` `ClusterDNSApplyOptions` 追加
- [x] Provider Driver インターフェース更新
  - [x] `adapters/drivers/provider/registry.go`: `ClusterDNSApply` を追加(コメントでベストエフォートを明記)
- [x] Provider 実装 (スタブ)
  - [x] AKS: `adapters/drivers/provider/aks/cluster.go` に no-op 実装(デバッグログ出力)
  - [x] K3s: `adapters/drivers/provider/k3s/k3s.go` に no-op 実装
- [x] Port アダプタの委譲実装
  - [x] `adapters/drivers/provider/cluster_port.go`: `DNSApply` でドライバへ委譲
- [x] ビルド確認
  - [x] `make build` が成功すること

## テスト

- ユニット/スモーク
  - 署名の整合とビルド成功 (`make build`).
  - 既存機能への影響なし (コンパイルエラー無し).
- E2E (将来)
  - AKS ドライバで Ingress の外部 IP を検出し、`A/AAAA` を Upsert できること。
  - `ZoneHint` 指定時/未指定時のゾーン解決が期待通りであること。
  - `Strict`/`DryRun` の動作切り替え確認。

## 受け入れ条件

- Domain に `DNSRecordType`/`DNSRecordSet` が存在する。
- `ClusterPort` に `DNSApply` が追加され、Provider への委譲実装がある。
- Provider Driver インターフェースに `ClusterDNSApply` が追加され、AKS/K3s でメソッドが定義されている (現状スタブ).
- ビルドが成功する (`make build`).
- ベストエフォート挙動 (権限不足等は既定でエラーにしない) がドライバ契約に明記されている。

## メモ

- 今回は API/モデルの導入が目的。実 DNS 実装は別タスクで対応 (Azure DNS 等).
- `Ensure` ではなく `Apply` を採用: 達成不能時に即エラーへ寄らない方針に適合。
- レコードセット削除は `Rdata` を空にする設計で表現可能。

## 進捗

- 2025-09-30: API/モデル/スタブの実装とビルド確認を完了。

## 参考

- ADR
  - [K4x-ADR-004]

[K4x-ADR-004]: ../../design/adr/K4x-ADR-004.md