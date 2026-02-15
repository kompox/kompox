---
id: 20260215b-networkpolicy-empty-peer
title: "NetworkPolicy の from: {} 出力修正"
status: draft
updated: 2026-02-15T09:34:13Z
language: ja
owner: yaegashi
adrs: []
plans: []
---
# Task: NetworkPolicy の from: {} 出力修正

## 目的

- `kompoxops app validate --out-manifest -` で生成される NetworkPolicy に `from: - {}` が出力される不具合を修正する。
- 期待する許可範囲 (same-namespace + 既定 namespace 許可) を、曖昧な zero value 表現に依存せず明示的に表現する。

## スコープ/非スコープ

- In:
  - `adapters/kube/converter.go` の NetworkPolicy peer 生成ロジックを修正する。
  - `from: - {}` が出力されないことをテストで固定する。
- Out:
  - Box/Compose のライフサイクル境界見直し。
  - `2026aa-kompox-box-update` のフェーズ計画更新。

## 仕様サマリ

- `from: - {}` は許可範囲が広く読めるため、same-namespace 許可を表す手段として不適切。
- same-namespace 許可は `namespaceSelector` により明示し、生成 YAML が意図と一致する状態にする。

## 計画 (チェックリスト)

- [ ] `from: - {}` が出る再現ケースをテスト化する。
- [ ] same-namespace 許可の表現を zero value 非依存へ変更する。
- [ ] 既存の `kube-system` / ingress namespace 許可との整合を確認する。
- [ ] 回帰テストを追加し、生成 YAML に `from: - {}` が出ないことを確認する。

## テスト

- ユニット:
  - `go test ./adapters/kube -run NetworkPolicy`
- スモーク:
  - `go test ./...`

## 問題の再現方法

- フィクスチャ配置: `tests/fixtures/20260215b-networkpolicy-empty-peer`
- 再現コマンド:
  - `kompoxops -C ./tests/fixtures/20260215b-networkpolicy-empty-peer app validate --out-manifest -`
- 確認ポイント:
  - 出力される `NetworkPolicy.spec.ingress[].from` に `- {}` が含まれる。

## 受け入れ条件

- 生成される NetworkPolicy に `from: - {}` が含まれない。
- same-namespace 許可と既定 namespace 許可が、意図どおりの peer として出力される。
- 既存テストに加え、当該不具合を検知できる回帰テストが追加される。

## メモ

- リスク:
  - NetworkPolicy peer の意味を変更すると通信到達性に影響する可能性がある。
- フォローアップ:
  - 必要なら old task の network-policy 系文書へ関連リンクを追加する。

## 進捗

- 2026-02-15T08:22:48Z タスク作成
- 2026-02-15T09:26:59Z 再現フィクスチャを `tests/fixtures/app-validate/networkpolicy-empty-peer` へ移動し、再現方法を追記
- 2026-02-15T09:34:13Z フィクスチャ配置を doc-id ベースの `tests/fixtures/20260215b-networkpolicy-empty-peer` へ変更

## 参照

- [adapters/kube/converter.go]
- [2026-02-12-network-policy]

[adapters/kube/converter.go]: ../../../../../adapters/kube/converter.go
[2026-02-12-network-policy]: ../../../../../_dev/tasks/2026-02-12-network-policy.ja.md
