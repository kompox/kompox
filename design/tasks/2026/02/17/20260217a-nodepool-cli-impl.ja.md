---
id: 20260217a-nodepool-cli-impl
title: NodePool CLI 実装 (Phase 5)
status: active
updated: 2026-02-17T02:34:28Z
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
- 実装レイヤは既存の `disk` / `snapshot` パターンに合わせ、`cmd/kompoxops` から直接 driver を叩かず `usecase/nodepool` を経由する。
- usecase 構成は `usecase/volume` と同様に `types.go` (Repos/UseCase) + 操作別ファイル(`list.go`,`create.go`,`update.go`,`delete.go`) を基本形とする。
- NodePool の設定項目が多いため、`create` / `update` は YAML/JSON ファイルから設定を読み込める入力方式を提供する。
- 設定ファイル形式は YAML を正 (基準) とし、JSON は YAML のサブセットとして互換入力として扱う。

## 計画 (チェックリスト)

- [ ] **usecase 追加 (先行実施)**: `usecase/nodepool` パッケージを追加する。
  - [ ] `types.go` に `Repos` / `UseCase` を定義し、`model.NodePoolPort` を注入可能にする。
  - [ ] `list.go` / `create.go` / `update.go` / `delete.go` を作成し、`Cluster` 解決と `NodePoolPort` 呼び出しを実装する。
  - [ ] 入力 DTO は `cluster_id` と操作固有項目を持ち、nil/必須チェックを usecase 側で統一する。
- [ ] **builder 接続**: `cmd/kompoxops/repos_builder.go` と `cmd/kompoxops/usecase_builder.go` に NodePool 用ビルダーを追加する。
  - [ ] `buildNodePoolRepos(cmd)` を追加し、`Workspace/Provider/Cluster` repository を束ねる。
  - [ ] `buildNodePoolUseCase(cmd)` を追加し、provider driver adapter から `NodePoolPort` を注入する。
  - [ ] 必要に応じて provider adapter 側に `GetNodePoolPort(...)` を追加する。
- [ ] **CLI 追加**: `cmd/kompoxops/cmd_cluster_nodepool.go` (新規) を追加し、`cluster` 配下に `nodepool` サブコマンドを登録する。
  - [ ] `newCmdClusterNodePool()` を実装し、`list/create/update/delete` を配下に持たせる。
  - [ ] `newCmdCluster()` (`cmd_cluster.go`) の `AddCommand(...)` に `newCmdClusterNodePool()` を追加する。
  - [ ] `--cluster-id` は `cluster` 親の persistent flag と既存 `resolveClusterID(...)` を流用する。
- [ ] **各サブコマンド実装**: `list/create/update/delete` から `usecase/nodepool` を呼ぶ。
  - [ ] `list`: NodePool 一覧を JSON で出力する。
  - [ ] `create`: 必須入力を検証し、作成結果を JSON で出力する。
  - [ ] `update`: mutable 項目のみ受け付け、immutable 更新エラーをそのまま返す。
  - [ ] `delete`: `not found` の冪等挙動を維持する。
- [ ] **YAML/JSON ファイル入力対応**: NodePool の多項目設定をファイル入力で扱えるようにする。
  - [ ] `create/update` に `--file <path>` (または同等) を追加し、YAML/JSON を DTO にマップする。
  - [ ] 入力フォーマットは YAML を正とし、JSON も受理することを CLI ヘルプと仕様に明記する。
  - [ ] CLI フラグ直接指定とファイル入力の優先順位・排他規則を定義する。
  - [ ] YAML/JSON の必須項目バリデーションとエラーメッセージを整備する。
- [ ] **オプション整合**: [Kompox-CLI] の記載に合わせてフラグ名・意味・ヘルプを確定する。
  - [ ] `kompoxops cluster nodepool --help` および各サブコマンド `--help` の文言を確認する。
  - [ ] 既存サブコマンド(`disk/snapshot`)と同様に `SilenceUsage/SilenceErrors/DisableSuggestions` 方針を適用する。
- [ ] **テスト追加**:
  - [ ] `usecase/nodepool` のユニットテスト (`list/create/update/delete`) を追加する。
  - [ ] `cmd/kompoxops` のコマンド層テストを必要最小限追加する (引数バリデーション/呼び出し経路)。

## テスト

- ユニット:
  - `usecase/nodepool`: 入力バリデーション、Cluster 解決、`NodePoolPort` 呼び出し、エラー透過
  - `cmd/kompoxops`: `cluster nodepool` の引数解析とエラー条件
- スモーク:
  - `kompoxops cluster nodepool --help` が表示できる。
  - `kompoxops cluster nodepool list --help` が表示できる。
  - `make build` が成功する。
  - `make test` が成功する。

## 受け入れ条件

- `kompoxops cluster nodepool` の `list/create/update/delete` が呼び出せる。
- 各コマンドが `--cluster-id` を受け取り、`usecase/nodepool` 経由で NodePool driver API を実行する。
- 未対応 driver で `not implemented` を判別可能に返却する。
- [Kompox-CLI] 設計との不整合がない。
- `disk/snapshot` と同等のレイヤ分離 (`cmd -> usecase -> port`) が保たれている。
- `create/update` で YAML/JSON ファイル入力が利用でき、YAML を正とした仕様・ヘルプが整備され、入力エラー時に原因を判別可能に返却する。

## 備考

- リスク:
  - CLI 引数スキーマと driver DTO の境界が曖昧だと後続フェーズの互換性影響が大きい。
- フォローアップ:
  - Phase 6 で NodePool ラベル/ゾーン整合を追加する。
  - Phase 7 で E2E を含む検証を拡張する。

## 進捗

- 2026-02-17T02:00:29Z タスクファイルを作成
- 2026-02-17T02:09:58Z `disk`/`snapshot` 実装パターンを基に、Phase 5 の計画・チェックリストを具体化。先行ステップとして `usecase/nodepool` 追加を明記
- 2026-02-17T02:32:42Z NodePool 多項目設定に対応するため、`create/update` の JSON ファイル入力対応を仕様・チェックリスト・受け入れ条件に追記
- 2026-02-17T02:34:28Z ファイル入力要件を YAML/JSON 対応へ拡張し、YAML を正 (基準) とする方針を追記

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
