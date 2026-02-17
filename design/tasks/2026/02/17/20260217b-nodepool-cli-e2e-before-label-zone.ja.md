---
id: 20260217b-nodepool-cli-e2e-before-label-zone
title: NodePool CLI E2E 先行追加 (Phase 6 着手前)
status: draft
updated: 2026-02-17T07:05:46Z
language: ja
owner: yaegashi
adrs:
  - K4x-ADR-019
plans:
  - 2026ab-k8s-node-pool-support
---
# タスク: NodePool CLI E2E 先行追加 (Phase 6 着手前)

本タスクは Plan [2026ab-k8s-node-pool-support] に対して、Phase 6 (Deployment/Node ラベル調整) の前に `kompoxops cluster nodepool` の E2E 検証を先行追加するための作業項目です。

## 目的

- NodePool CLI (`list/create/update/delete`) の実運用経路を E2E で固定化し、後続のラベル/ゾーン整合実装の回帰を早期検出できる状態にする。
- 「CLI 経路は正常」というベースラインを先に確立し、Phase 6 の不具合切り分けを容易にする。

## スコープ / 非スコープ

- 対象:
  - 既存 AKS E2E シナリオに NodePool CLI 経路の追加/更新/削除/一覧を追加する。
  - 失敗時に原因を特定しやすいログ出力(入力 spec / 実行コマンド / 主要 API エラー)を整える。
  - `--file` 入力(YAML 正、JSON 互換)の最低 1 ケースを E2E で検証する。
- 非対象:
  - `kompox.dev/node-pool` / `kompox.dev/node-zone` ラベル整合ロジックの追加 (Phase 6 で実施)。
  - driver 契約や DTO の再設計。

## 仕様サマリ

- E2E は CLI 観点を優先し、`cmd -> usecase -> driver` の実経路を通す。
- NodePool 名衝突を避けるため、テストごとにユニーク接尾辞を付与する。
- 後始末を必須化し、失敗時も `delete` を実行して次シナリオへ影響を残さない。

## 計画 (チェックリスト)

- [ ] AKS E2E の実行基盤から NodePool CLI 呼び出しヘルパーを利用可能にする。
- [ ] `create --file` (YAML) → `list` で反映確認のケースを追加する。
- [ ] `update --file` (YAML または JSON) → `list` で差分確認のケースを追加する。
- [ ] `delete` 後に `list` から消えることを確認する。
- [ ] 失敗時のデバッグ情報(対象 cluster id / pool 名 / 入力ファイル)を記録する。

## テスト

- E2E:
  - 既存 `tests/aks-e2e-*` 系の NodePool 実行可能シナリオを追加
- スモーク:
  - `kompoxops cluster nodepool --help`
  - `kompoxops cluster nodepool create --help`

## 受け入れ条件

- Phase 6 着手前に、NodePool CLI の主要ユースケース(create/update/delete/list)が AKS E2E で通る。
- 失敗時のログから、CLI 引数不備・driver エラー・環境要因を切り分け可能である。
- 既存 AKS E2E シナリオの回帰がない。

## 備考

- リスク:
  - E2E 実行時間が増える可能性があるため、NodePool ケースは最小本数で始める。
- フォローアップ:
  - Phase 6 でラベル/ゾーン整合ロジックを追加後、本タスクで追加した E2E を回帰判定の基準として再利用する。

## 進捗

- 2026-02-17T07:05:46Z タスクファイルを作成

## 参照

- [2026ab-k8s-node-pool-support]
- [K4x-ADR-019]
- [20260217a-nodepool-cli-impl]
- [design/tasks/README.md]

[2026ab-k8s-node-pool-support]: ../../../plans/2026/2026ab-k8s-node-pool-support.ja.md
[K4x-ADR-019]: ../../../adr/K4x-ADR-019.md
[20260217a-nodepool-cli-impl]: ./20260217a-nodepool-cli-impl.ja.md
[design/tasks/README.md]: ../../README.md
