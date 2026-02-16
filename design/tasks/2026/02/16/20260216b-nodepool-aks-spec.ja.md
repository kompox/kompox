---
id: 20260216b-nodepool-aks-spec
title: NodePool 対応に向けた AKS ProviderDriver 仕様更新 (Phase 2)
status: active
updated: 2026-02-16T15:59:31Z
language: ja
owner: yaegashi
adrs:
  - K4x-ADR-019
plans:
  - 2026ab-k8s-node-pool-support
---
# タスク: NodePool 対応に向けた AKS ProviderDriver 仕様更新 (Phase 2)

本タスクは、Plan [2026ab-k8s-node-pool-support] の Phase 2 を具体化する作業項目です。

## 目的

- [Kompox-ProviderDriver-AKS] に、NodePool 抽象を AKS Agent Pool API へマッピングする仕様を追加する。
- 断片的な追記を継ぎ足すのではなく、現実装を正として文書全体をリライトし、章立てと記述粒度を再整理する。
- 本タスクでは設計文書更新のみを行い、AKS driver 実装コードの変更は行わない。

## スコープ / 非スコープ

- 対象:
  - [Kompox-ProviderDriver-AKS] に NodePool `List/Create/Update/Delete` の AKS 対応方針を追記
  - 必須項目/可変項目/非対応項目の境界と、`not implemented` / validation error の使い分けを明記
  - `kompox.dev/node-pool` / `kompox.dev/node-zone` ラベル整合と zone 正規化責務を明記
- 非対象:
  - `adapters/drivers/provider/aks/**` の実装変更
  - E2E/Unit test の実装変更
  - CLI/Converter 仕様変更

## 仕様サマリ

- Kompox の `NodePool` 抽象を AKS の Agent Pool へマッピングする。
- `NodePoolGet` は導入せず、`List + name filter` を前提とする。
- `NodePoolUpdate` は mutable 項目のみ適用し、immutable 項目は validation error とする。
- 未対応機能は `not implemented` を返す capability 境界として明記する。
- 仕様記述は「実コードで確認できる事実」を優先し、未実装事項は TODO として境界を明示する。

## 計画 (チェックリスト)

- [ ] [Kompox-ProviderDriver-AKS] の既存セクション構成を確認する。
- [ ] 現実装(`adapters/drivers/provider/aks/**`)を一次情報として、章構成をリライト方針に沿って再編する。
- [ ] NodePool と AKS Agent Pool の用語・フィールド対応表を追記する。
- [ ] `List/Create/Update/Delete` の API 対応方針と入力制約を追記する。
- [ ] mutable/immutable 項目、`not implemented` / validation error の境界を追記する。
- [ ] ラベル(`kompox.dev/node-pool`, `kompox.dev/node-zone`)と zone 正規化責務を追記する。
- [ ] `make gen-index` を実行してインデックスを更新する。

## テスト

- ユニット: なし (docs-only)
- スモーク:
  - `make gen-index` が成功する。
  - `design/index.json` と `design/tasks/index.json` に task が反映される。

## 受け入れ条件

- [Kompox-ProviderDriver-AKS] に NodePool ↔ Agent Pool のマッピング方針が記載されている。
- [K4x-ADR-019] および [Kompox-ProviderDriver] と矛盾しない。
- 本タスクの範囲で実装コードが変更されていない。

## 備考

- リスク:
  - AKS API 仕様を実装詳細まで書きすぎると、設計文書と実装の責務境界が曖昧になる。
  - 現実装と乖離した記述を残すと、以後の実装判断とレビュー基準が不安定になる。
- フォローアップ:
  - 次タスクで AKS driver 実装(Phase 4)とテスト(Phase 6)を進める。

## 進捗

- 2026-02-16T15:48:34Z タスクファイルを作成
- 2026-02-16T15:52:40Z 現実装を正として AKS ProviderDriver 仕様をリライトする方針を追加

## 参照

- [2026ab-k8s-node-pool-support]
- [K4x-ADR-019]
- [Kompox-ProviderDriver]
- [Kompox-ProviderDriver-AKS]
- [design/tasks/README.md]

[2026ab-k8s-node-pool-support]: ../../../plans/2026/2026ab-k8s-node-pool-support.ja.md
[K4x-ADR-019]: ../../../adr/K4x-ADR-019.md
[Kompox-ProviderDriver]: ../../../v1/Kompox-ProviderDriver.ja.md
[Kompox-ProviderDriver-AKS]: ../../../v1/Kompox-ProviderDriver-AKS.ja.md
[design/tasks/README.md]: ../../README.md
