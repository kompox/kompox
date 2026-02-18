---
id: 20260217c-kom-app-deployment-impl
title: KOM App Deployment 指定と KubeConverter 更新
status: done
updated: 2026-02-18T01:30:40Z
language: ja
owner: yaegashi
adrs:
  - K4x-ADR-019
plans:
  - 2026ab-k8s-node-pool-support
---
# タスク: KOM App Deployment 指定と KubeConverter 更新

本タスクは Plan [2026ab-k8s-node-pool-support] の Phase 6 において、KOM 定義と KubeConverter の解釈を更新し、`deployment.pool/zone/pools/zones` のスケジューリング契約を明確化する作業項目です。

## 目的

- `App.spec.deployment.pool/zone` と `pools/zones` の解釈ルールを KOM 定義として明文化する。
- KubeConverter の NodeSelector/ラベル付与ロジックを上記契約に一致させる。
- Bicep/NodePool/Manifest の 3 層で `kompox.dev/node-pool` / `kompox.dev/node-zone` の意味を一致させるための基盤を整える。

## スコープ / 非スコープ

- In:
  - `AppDeploymentSpec` に `pools` / `zones` / `selectors` フィールドを追加し、CRD から model への取り込み経路を更新する。
  - `deployment.pool/zone/pools/zones` の優先順位・排他・互換ルールを実装レベルで定義する。
  - KubeConverter が deployment 設定から `nodeSelector` / `nodeAffinity` を出力する仕様と実装を一致させる。
  - `Kompox-KOM` / `Kompox-KubeConverter` の Phase 6 対応差分を設計文書に反映する。
- Out:
  - AKS driver の NodePool ラベル実装変更そのもの (別タスクで実施)。
  - `deployment.selectors` の本実装 (本タスクでは予約/未サポート方針維持)。

## 仕様サマリ

- `AppDeploymentSpec` は `pool` / `zone` (単数) と `pools` / `zones` (複数) を持ち、同種の単数・複数同時指定は validation error とする。
- `deployment.selectors` は予約フィールドとして受理するが、値が 1 件でも設定された場合は未サポートとして validation error とする。
- KubeConverter は次の写像規則で出力する。
  - `pool` / `zone` は `Deployment.spec.template.spec.nodeSelector` の `kompox.dev/node-pool` / `kompox.dev/node-zone` に写像。
  - `pools` / `zones` は `Deployment.spec.template.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution` の `In` 条件へ写像。
- 互換優先として既存 `pool/zone` の既定挙動(既定 pool=`user`)を維持しつつ、`pools/zones` を追加する。
- `deployment.selectors` は将来拡張として予約し、本タスクでは実装しない。

## 計画 (チェックリスト)

- [x] **モデル/CRD 更新** (`config/crd/ops/v1alpha1`)
  - [x] `types.go` の `AppDeploymentSpec` に `Pools []string`、`Zones []string`、`Selectors map[string]string` (または同等の予約表現) を追加する。
  - [x] 単数/複数同時指定を禁止する validation を追加する(同時指定時は error)。
  - [x] `deployment.selectors` に値が設定された場合は未サポートとして validation error を返す。
  - [x] `sink_tomodels.go` で `pool/zone/pools/zones` を model 側へ取り込む。
  - [x] `sink_tomodels_test.go` に単数/複数/同時指定エラーに加えて、`selectors` 指定時エラーのケースを追加する。
- [x] **KubeConverter 更新** (`adapters/kube`)
  - [x] `converter.go` で `pool/zone` を `nodeSelector` に、`pools/zones` を `nodeAffinity` に写像する。
  - [x] 既定 `pool=user` 挙動と zone 未指定時の互換挙動を維持する。
  - [x] `converter_test.go` に `nodeSelector` / `nodeAffinity` 出力のケースを追加する。
- [x] **設計文書同期**
  - [x] [Kompox-KubeConverter] に `nodeSelector` / `nodeAffinity` 出力規則を実装準拠で追記する。
  - [x] [2026ab-k8s-node-pool-support] の Phase 6 進捗へ本タスクの反映を追記する。
- [x] **非スコープ確認**
  - [x] `deployment.selectors` は予約のみで未実装であり、設定時はエラーになることを明記する。

## テスト

- ユニット:
  - `config/crd/ops/v1alpha1/sink_tomodels_test.go` に `pools/zones` 取り込みと同時指定エラー検証を追加。
  - `config/crd/ops/v1alpha1/sink_tomodels_test.go` に `selectors` 設定時エラー検証を追加。
  - `adapters/kube/converter_test.go` に `nodeSelector` / `nodeAffinity` 出力検証を追加。
- E2E (フルセットの一部):
  - fixture `tests/fixtures/20260217c-kom-app-deployment-impl` を使い、`--kom-app ./app-valid-*.yml` / `--kom-app ./app-invalid-*.yml` の切替で成功系・失敗系(同時指定エラー、`selectors` 予約エラー)を確認。
  - `deployment.pool/zone` の既存入力で出力が変わらないことを確認。
  - `deployment.pools/zones` 入力で `nodeAffinity.required...In` が出力されることを確認。

## 受け入れ条件

- `AppDeploymentSpec` で `pool/zone/pools/zones` が表現でき、同時指定制約が検証される。
- `AppDeploymentSpec` で `selectors` が予約フィールドとして定義され、値が設定された場合に未サポートエラーになる。
- KubeConverter が仕様どおり `nodeSelector` / `nodeAffinity` を出力する。
- `deployment.pool/zone` 既存ケースの後方互換が維持される。
- `deployment.pool/zone/pools/zones` の解釈ルールが KOM と KubeConverter で矛盾なく定義されている。
- `kompox.dev/node-pool` / `kompox.dev/node-zone` の用途が Bicep/NodePool/Manifest の接続点として説明されている。
- Phase 6 の後続実装タスクが本タスクの仕様を参照して着手できる。

## メモ

- リスク:
  - 優先順位定義が曖昧だと、既存 manifest のスケジューリング結果が意図せず変わる。
- フォローアップ:
  - 本タスク完了後、AKS driver 側ラベル実装調整タスクを追加する。

## 進捗

- 2026-02-17T09:07:40Z タスクファイルを作成
- 2026-02-17T09:13:59Z `AppDeploymentSpec` フィールド追加と KubeConverter の `nodeSelector` / `nodeAffinity` 出力要件に合わせて、実装対象ファイルとテスト観点を含む具体的チェックリストへ更新
- 2026-02-17T10:58:14Z `domain/model`・`config/crd`・`config/kompoxopscfg`・`adapters/kube` に `pool/zone/pools/zones/selectors` 対応を実装し、単数/複数同時指定エラーと `selectors` 予約エラーを追加。関連ユニットテストを更新し `make test` 全通過を確認
- 2026-02-17T10:58:14Z `tests/fixtures/20260217c-kom-app-deployment-impl` を追加し、`.kompox/kom/` に共通の Workspace/Provider/Cluster 定義、`app-valid-*` / `app-invalid-*` に App パターンを分離して `--kom-app` 切替検証手順を整備
- 2026-02-17T11:34:21Z doc-id を `20260217c-kom-app-deployment-impl` に統一し、task/plan/fixture の参照リンクとディレクトリ名を整合。タイトルから `Phase 6` 表現を削除
- 2026-02-17T11:38:25Z [Kompox-KOM] に `App.spec.deployment` 入力契約 (単数/複数排他・`selectors` 予約・互換ルール) を追記し、[Kompox-KubeConverter] に nodeSelector/nodeAffinity の実装準拠出力規則を追記。設計文書同期チェックを完了
- 2026-02-17T11:44:26Z 指摘により [Kompox-KOM] の `App.spec.deployment` 追記を取り下げて原状へ復元。設計文書同期の [Kompox-KOM] 項目を未完了へ戻した
- 2026-02-17T11:55:47Z 方針変更に合わせてチェックリストから [Kompox-KOM] 項目を削除し、設計文書同期は [Kompox-KubeConverter] と plan 反映を完了条件として更新
- 2026-02-17T12:44:29Z KOM/NodePool/Bicep のスケジューリングラベル方針整理が完了。`Kompox-KubeConverter` へ運用方針と fixture 手順を反映し、本タスクを `done` へ更新

## 参照

- [2026ab-k8s-node-pool-support]
- [K4x-ADR-019]
- [Kompox-KOM]
- [Kompox-KubeConverter]
- [design/tasks/README.md]

[2026ab-k8s-node-pool-support]: ../../../../plans/2026/2026ab-k8s-node-pool-support.ja.md
[K4x-ADR-019]: ../../../../adr/K4x-ADR-019.md
[Kompox-KOM]: ../../../../v1/Kompox-KOM.ja.md
[Kompox-KubeConverter]: ../../../../v1/Kompox-KubeConverter.ja.md
[design/tasks/README.md]: ../../../README.md
