---
id: 20260217a-nodepool-cli-impl
title: NodePool CLI 実装 (Phase 5)
status: active
updated: 2026-02-17T02:00:29Z
language: ja
owner: yaegashi
adrs:
  - K4x-ADR-019
plans:
  - 2026ab-k8s-node-pool-support
---
# タスク: NodePool CLI 実装 (Phase 5)

本タスクは、Plan [2026ab-k8s-node-pool-support] の Phase 5 を具体化する作業項目です。

## 目的

- `kompoxops cluster nodepool` 系コマンドを追加し、NodePool の日次運用を CLI から実行可能にする。
- [Kompox-CLI] のコマンド仕様と Provider Driver の NodePool 契約を接続する。
- 後続 Phase (ラベル/ゾーン整合、テスト拡張) の前提となる操作経路を確立する。

## スコープ / 非スコープ

- 対象:
  - `kompoxops cluster nodepool list --cluster-id <clusterID>` を追加
  - `kompoxops cluster nodepool create --cluster-id <clusterID> ...` を追加
  - `kompoxops cluster nodepool update --cluster-id <clusterID> ...` を追加
  - `kompoxops cluster nodepool delete --cluster-id <clusterID> --name <poolName>` を追加
  - コマンド入出力/エラーの最小仕様を [Kompox-CLI] と整合
- 非対象:
  - AKS driver の NodePool API 実装本体の追加変更 (Phase 4 で完了)
  - NodePool ラベル/ゾーン整合ロジックの実装 (Phase 6 で実施)
  - E2E シナリオ拡張 (Phase 7 で実施)

## 仕様サマリ

- CLI は [Kompox-ProviderDriver] の `NodePoolList/Create/Update/Delete` に対応する操作を公開する。
- `cluster-id` は既存 `kompoxops cluster` サブコマンド群と同じ解決規則を継承する。
- Provider/driver 未対応時は判別可能な `not implemented` エラーを利用者へ返す。

## 計画 (チェックリスト)

- [ ] `cluster nodepool` サブコマンドのルーティングを追加する。
- [ ] `list` コマンドを追加し、NodePool 一覧を表示する。
- [ ] `create` コマンドを追加し、必須オプションのバリデーションを実装する。
- [ ] `update` コマンドを追加し、immutable 項目更新要求時のエラーを透過する。
- [ ] `delete` コマンドを追加し、冪等 delete の挙動を維持する。
- [ ] [Kompox-CLI] の記載と実装オプション名/意味を一致させる。

## テスト

- ユニット:
  - 各サブコマンドの引数解析と入力バリデーション
  - driver 呼び出し結果の表示/エラー変換
- スモーク:
  - `make build` が成功する。
  - `make test` が成功する。

## 受け入れ条件

- `kompoxops cluster nodepool` の `list/create/update/delete` が呼び出せる。
- 各コマンドが `--cluster-id` を受け取り、NodePool driver API を実行する。
- 未対応 driver で `not implemented` を判別可能に返却する。
- [Kompox-CLI] 設計との不整合がない。

## 備考

- リスク:
  - CLI 引数スキーマと driver DTO の境界が曖昧だと後続フェーズの互換性影響が大きい。
- フォローアップ:
  - Phase 6 で NodePool ラベル/ゾーン整合を追加する。
  - Phase 7 で E2E を含む検証を拡張する。

## 進捗

- 2026-02-17T02:00:29Z タスクファイルを作成

## 参照

- [2026ab-k8s-node-pool-support]
- [K4x-ADR-019]
- [Kompox-CLI]
- [Kompox-ProviderDriver]
- [design/tasks/README.md]

[2026ab-k8s-node-pool-support]: ../../../plans/2026/2026ab-k8s-node-pool-support.ja.md
[K4x-ADR-019]: ../../../adr/K4x-ADR-019.md
[Kompox-CLI]: ../../../v1/Kompox-CLI.ja.md
[Kompox-ProviderDriver]: ../../../v1/Kompox-ProviderDriver.ja.md
[design/tasks/README.md]: ../../README.md
