---
id: 20260218a-nodepool-tests
title: NodePool テスト実装
status: active
updated: 2026-02-18T02:12:58Z
language: ja
owner: yaegashi
adrs:
  - K4x-ADR-019
plans:
  - 2026ab-k8s-node-pool-support
---
# タスク: NodePool テスト実装

本タスクは Plan [2026ab-k8s-node-pool-support] の Phase 7 として、NodePool 機能のテスト不足を解消し、ADR 判定に必要な検証根拠を揃える。

## 目的

- AKS driver の NodePool API 呼び出しに対する unit test を追加し、主要分岐の破壊的変更を早期検出できる状態にする。
- `kompoxops cluster nodepool` のコマンド層テストを追加し、引数バリデーションと usecase 呼び出し経路を固定化する。
- 既存 AKS E2E に NodePool の追加/更新/削除ケースを組み込み、回帰確認を継続運用できる形にする。
- 既存主要機能 (ClusterProvision/Install、Volume 系) の回帰がないことを確認し、Phase 9 の ADR ステータス判定へ接続する。

## スコープ / 非スコープ

- In:
  - `adapters/drivers/provider/aks` の NodePool 関連 unit test を拡張する。
  - `cmd/kompoxops` の `cluster nodepool` コマンド層テストを追加する。
  - `tests/aks-e2e-nodepool` および既存 AKS E2E に NodePool update/delete を含むケースを追加する。
  - NodePool 追加後に既存フロー (ClusterProvision/Install、Volume 系) が通ることを検証する。
- Out:
  - NodePool 機能自体の仕様変更・機能追加。
  - [K4x-ADR-019] の `accepted` 反映 (本タスク完了後、Phase 9 で判断)。

## 仕様サマリ

- テスト追加は既存実装を前提とし、契約変更を伴わない。
- Unit では AKS Agent Pool API 呼び出しパラメータ、エラー分類 (`validation error` / `not implemented`)、冪等 delete を主要観点とする。
- CLI コマンド層では `--cluster-id` / `--file` の必須チェック、入力ファイル不正時のエラー、usecase 呼び出しへの引き渡しを観点とする。
- E2E は NodePool create/list/update/delete の連続操作に加え、既存シナリオへの副作用がないことを確認する。

## 計画 (チェックリスト)

- [ ] **AKS driver unit test 追加**
  - [ ] `NodePoolList/Create/Update/Delete` の主要成功系を追加する。
  - [ ] immutable 更新要求の validation error ケースを追加する。
  - [ ] delete の NotFound 冪等ケースを追加する。
- [ ] **CLI コマンド層テスト追加**
  - [ ] `cluster nodepool list/create/update/delete` の引数バリデーションを追加する。
  - [ ] `--file` 入力の正常/異常ケースを追加する。
  - [ ] usecase 呼び出し経路 (入力 DTO マッピング) を検証する。
- [ ] **E2E 拡張**
  - [ ] NodePool update/delete を既存 `tests/aks-e2e-nodepool` シナリオに追加する。
  - [ ] 既存 AKS E2E スイートへ NodePool 操作を組み込んだ回帰ケースを追加する。
- [ ] **回帰検証**
  - [ ] ClusterProvision/Install の主要経路に回帰がないことを確認する。
  - [ ] Volume 系 E2E に回帰がないことを確認する。
- [ ] **生成物同期**
  - [ ] `make gen-index` を実行して task index を更新する。

## テスト

- ユニット:
  - `adapters/drivers/provider/aks` NodePool 関連テスト
  - `cmd/kompoxops` NodePool コマンド層テスト
- E2E:
  - `tests/aks-e2e-nodepool` (create/list/update/delete)
  - 既存 AKS E2E スイート (ClusterProvision/Install, Volume 系)
- スモーク:
  - `make build`
  - `make test`

## 受け入れ条件

- AKS driver NodePool API の主要分岐が unit test でカバーされている。
- `kompoxops cluster nodepool` コマンド層で引数バリデーションと DTO 引き渡しがテストで固定化されている。
- NodePool 追加/更新/削除を含む AKS E2E が安定実行できる。
- 既存主要機能 (ClusterProvision/Install、Volume 系) に回帰がないことを確認できる。

## メモ

- リスク:
  - E2E 実行時間増加により CI 負荷が高くなる可能性がある。
- フォローアップ:
  - 本タスク完了後、[2026ab-k8s-node-pool-support] の Phase 9 で [K4x-ADR-019] ステータス判定へ進む。

## 進捗

- 2026-02-18T02:11:40Z タスクファイルを作成

## 参照

- [2026ab-k8s-node-pool-support]
- [K4x-ADR-019]
- [20260217a-nodepool-cli-impl]
- [20260217b-nodepool-cli-e2e-before-label-zone]
- [20260216d-nodepool-aks-driver-impl]
- [design/tasks/README.md]

[2026ab-k8s-node-pool-support]: ../../../../plans/2026/2026ab-k8s-node-pool-support.ja.md
[K4x-ADR-019]: ../../../../adr/K4x-ADR-019.md
[20260217a-nodepool-cli-impl]: ../17/20260217a-nodepool-cli-impl.ja.md
[20260217b-nodepool-cli-e2e-before-label-zone]: ../17/20260217b-nodepool-cli-e2e-before-label-zone.ja.md
[20260216d-nodepool-aks-driver-impl]: ../16/20260216d-nodepool-aks-driver-impl.ja.md
[design/tasks/README.md]: ../../../README.md
