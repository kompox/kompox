---
id: 20260217d-nodepool-doc-sync
title: NodePool 設計ドキュメント同期
status: done
updated: 2026-02-18T01:22:50Z
language: ja
owner: yaegashi
adrs:
  - K4x-ADR-019
plans:
  - 2026ab-k8s-node-pool-support
---
# タスク: NodePool 設計ドキュメント同期

本タスクは Plan [2026ab-k8s-node-pool-support] の文書同期タスクとして、NodePool 実装・検証の現状を設計文書へ同期し、次フェーズのテスト実施および ADR 判定の前提を整える。

## 目的

- [Kompox-ProviderDriver] に NodePool 契約・実装進捗・テスト状況を最新化する。
- [Kompox-ProviderDriver-AKS] に AKS 固有の NodePool 運用・検証手順を最新化する。
- [Kompox-CLI] に `cluster nodepool` コマンド群の現行仕様・検証観点を反映する。
- [Kompox-KubeConverter] に `deployment.pool/zone/pools/zones` と NodePool 契約の接続点を最新化する。

## スコープ / 非スコープ

- In:
  - `design/v1/Kompox-ProviderDriver.ja.md` の NodePool 関連記述を現行実装と整合させる。
  - `design/v1/Kompox-ProviderDriver-AKS.ja.md` の Agent Pool/NodePool 運用手順・制約を現行実装に合わせて更新する。
  - `design/v1/Kompox-CLI.ja.md` の `kompoxops cluster nodepool` 系コマンド仕様・引数・検証観点を現行挙動に合わせて更新する。
  - `design/v1/Kompox-KubeConverter.ja.md` の `deployment.pool/zone/pools/zones` と NodePool 契約接続の記述を現行状態へ更新する。
  - 参照リンク・References・index 生成物の整合をとる。
- Out:
  - NodePool 実装コードの機能追加/変更。
  - Phase 7 のテスト実装そのもの。
  - ADR [K4x-ADR-019] の `accepted` への変更。

## 仕様サマリ

- 文書更新は「実装済みの事実」と「未完了項目」を明確に分離して記載する。
- `not implemented` 境界、互換方針、既知の制約は曖昧にせず明示する。
- CLI 文書には `list/create/update/delete` の入力方式 (`--file` 前提) と検証観点を明記する。

## 計画 (チェックリスト)

- [x] **ProviderDriver 文書更新**
  - [x] [Kompox-ProviderDriver] の NodePool 管理 API 節を現行実装準拠へ更新する。
  - [x] 全ドライバ共通の `not implemented` 境界と互換方針を確認・明記する。
- [x] **AKS ProviderDriver 文書更新**
  - [x] [Kompox-ProviderDriver-AKS] の Agent Pool 対応範囲と更新可能項目を現行挙動に合わせる。
  - [x] NodePool 運用/検証手順(想定コマンドや確認観点)を同期する。
- [x] **CLI 文書更新**
  - [x] [Kompox-CLI] の `kompoxops cluster nodepool` 仕様を現行実装に同期する。
  - [x] `--file` 入力方式とエラー/検証観点を明記する。
- [x] **KubeConverter 文書更新**
  - [x] [Kompox-KubeConverter] の NodePool スケジューリング契約記述を Phase 8 方針に合わせて同期する。
- [x] **生成物同期**
  - [x] `make gen-index` を実行して `design` index を更新する。

## テスト

- ドキュメント整合確認:
  - 更新した 4 文書で NodePool 用語と責務境界の不整合がないことを確認。
  - コマンド仕様・制約・未実装項目が実装状況と矛盾しないことを確認。
- 生成確認:
  - `make gen-index` 実行後に `design/index.json` と `design/v1/index.json` の更新内容を確認。

## 受け入れ条件

- [Kompox-ProviderDriver] / [Kompox-ProviderDriver-AKS] / [Kompox-CLI] / [Kompox-KubeConverter] の 4 文書で NodePool 関連記述が現行実装と一致する。
- 未完了項目 (Phase 7 残タスク) と既知制約が明示されている。
- `make gen-index` 後の index が更新され、参照リンクが解決できる。

## メモ

- リスク:
  - 文書の用語差 (`NodePool` / `Agent Pool`) を混在させると運用手順の誤解を招く。
- フォローアップ:
  - 本タスク完了後、Phase 7 の残テスト実装タスクを順次着手する。

## 進捗

- 2026-02-17T23:07:43Z タスクファイルを作成
- 2026-02-17T23:10:22Z Phase 8 対象へ [Kompox-KubeConverter] を追加し、スコープ/チェックリスト/受け入れ条件を更新
- 2026-02-17T23:18:51Z [Kompox-ProviderDriver] / [Kompox-ProviderDriver-AKS] / [Kompox-CLI] / [Kompox-KubeConverter] の NodePool 関連記述を同期し、本タスクを `done` に更新
- 2026-02-18T01:09:35Z 各対象文書の更新状況を再確認し、本タスクに確認結果を追記
- 2026-02-18T01:20:06Z [Kompox-CLI] について、NodePool 範囲に限定せず CLI 全体にわたってセクション構成と文章表現のリライトが行われたことを追記

## 参照

- [2026ab-k8s-node-pool-support]
- [K4x-ADR-019]
- [Kompox-ProviderDriver]
- [Kompox-ProviderDriver-AKS]
- [Kompox-CLI]
- [Kompox-KubeConverter]
- [design/tasks/README.md]

[2026ab-k8s-node-pool-support]: ../../../../plans/2026/2026ab-k8s-node-pool-support.ja.md
[K4x-ADR-019]: ../../../../adr/K4x-ADR-019.md
[Kompox-ProviderDriver]: ../../../../v1/Kompox-ProviderDriver.ja.md
[Kompox-ProviderDriver-AKS]: ../../../../v1/Kompox-ProviderDriver-AKS.ja.md
[Kompox-CLI]: ../../../../v1/Kompox-CLI.ja.md
[Kompox-KubeConverter]: ../../../../v1/Kompox-KubeConverter.ja.md
[design/tasks/README.md]: ../../../README.md
