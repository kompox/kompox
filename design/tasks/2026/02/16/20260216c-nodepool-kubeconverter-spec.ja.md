---
id: 20260216c-nodepool-kubeconverter-spec
title: NodePool 対応に向けた KubeConverter 契約更新 (Phase 3)
status: done
updated: 2026-02-16T19:06:55Z
language: ja
owner: yaegashi
adrs:
  - K4x-ADR-019
plans:
  - 2026ab-k8s-node-pool-support
---
# タスク: NodePool 対応に向けた KubeConverter 契約更新 (Phase 3)

本タスクは、Plan [2026ab-k8s-node-pool-support] の Phase 3 を具体化する作業項目です。

## 目的

- [Kompox-KubeConverter] に `deployment.pool/zone` と NodePool 抽象の関係を明記し、責務分離を明確化する。
- [Kompox-KubeConverter] に `deployment.pools/zones` の複数候補指定仕様を追加し、初期実装の対象範囲を明確化する。
- `deployment.selectors` を将来拡張として予約し、現時点では未サポートとする方針を文書化する。
- `kompox.dev/node-pool` / `kompox.dev/node-zone` ラベル契約を維持し、zone 正規化責務を provider driver 側に置く方針を文書化する。
- Provider Driver 側の NodePool 仕様 ([Kompox-ProviderDriver], [Kompox-ProviderDriver-AKS]) と矛盾しない KubeConverter 仕様へ整理する。

## スコープ / 非スコープ

- 対象:
  - [Kompox-KubeConverter] の NodeSelector/nodeAffinity ラベル契約に関する節を更新
  - `deployment.pool/zone/pools/zones` と `kompox.dev/node-pool` / `kompox.dev/node-zone` のマッピング責務を明記
  - `deployment.selectors` を将来拡張として予約し、現時点では未サポートであることを明記
  - zone の値変換は converter ではなく provider driver の責務であることを明記
- 非対象:
  - `adapters/kube/converter.go` の実装変更
  - Provider Driver 実装 (`adapters/drivers/provider/**`) の変更
  - E2E/Unit test の追加・変更

## 仕様サマリ

- KubeConverter は `deployment.pool/zone` を nodeSelector、`deployment.pools/zones` を nodeAffinity へ写像する契約を維持する。
- ラベルキーは `kompox.dev/node-pool` / `kompox.dev/node-zone` を継続利用する。
- `deployment.selectors` は将来拡張として予約し、現時点では未サポート(バリデーションエラー)とする。
- zone 値のベンダ差異吸収 (例: AKS の `"1"` 形式) は provider driver 側で正規化し、converter は与えられた論理値を扱う。

## 計画 (チェックリスト)

- [x] [Kompox-KubeConverter] の既存 NodeSelector/ラベル仕様記述を確認する。
- [x] `deployment.pool/zone` と NodePool 抽象の関係を仕様として追記する。
- [x] `deployment.pools/zones` の複数候補指定を仕様として追記する。
- [x] `deployment.selectors` を将来拡張として予約し、現時点で未サポートであることを明記する。
- [x] `kompox.dev/node-pool` / `kompox.dev/node-zone` ラベル契約の維持を明記する。
- [x] zone 正規化責務を provider driver 側に置く方針を明記する。
- [x] [Kompox-ProviderDriver] / [Kompox-ProviderDriver-AKS] / [K4x-ADR-019] との整合を確認する。
- [x] `make gen-index` を実行してインデックスを更新する。

## テスト

- ユニット: なし (docs-only)
- スモーク:
  - `make gen-index` が成功する。
  - `design/index.json` と `design/tasks/index.json` に task が反映される。

## 受け入れ条件

- [Kompox-KubeConverter] に `deployment.pool/zone/pools/zones` と NodePool 抽象の責務分離が記載されている。
- `deployment.selectors` が将来拡張として予約され、現時点で未サポートであることが記載されている。
- `kompox.dev/node-pool` / `kompox.dev/node-zone` のラベル契約が明示されている。
- zone 正規化責務が provider driver 側であることが明示され、[K4x-ADR-019] と矛盾しない。
- 本タスクの範囲で実装コードが変更されていない。

## 備考

- リスク:
  - converter と driver の責務境界が曖昧なまま記述すると、実装時に重複実装や責務衝突を引き起こす。
- フォローアップ:
  - 次フェーズで AKS driver 実装タスク (Phase 4 以降) に接続する。

## 進捗

- 2026-02-16T18:22:47Z タスクファイルを作成
- 2026-02-16T18:28:15Z [Kompox-KubeConverter] の Deployment 節に NodePool 抽象との責務分離を追記。`deployment.pool/zone` と `kompox.dev/node-pool` / `kompox.dev/node-zone` の関係、zone 正規化責務を provider driver 側とする方針、関連参照 ([K4x-ADR-019]/[Kompox-ProviderDriver]/[Kompox-ProviderDriver-AKS]) を反映しタスク完了
- 2026-02-16T18:33:19Z 追加仕様として `kompox.dev/node-pool` / `kompox.dev/node-zone` の値規約を明文化。推奨フォーマット (Kubernetes Label Value 互換)、共通語彙 (`system` / `user`、`<region>-<zoneIndex>` 推奨)、互換語彙 (AKS 数字ゾーン) を [Kompox-ProviderDriver] / [Kompox-KubeConverter] に追記
- 2026-02-16T19:06:55Z 追加仕様として `deployment.pools/zones` の初期実装サポートと `deployment.selectors` の将来拡張予約(現時点未サポート)を [Kompox-KubeConverter] に反映し、計画・受け入れ条件を更新

## 参照

- [2026ab-k8s-node-pool-support]
- [K4x-ADR-019]
- [Kompox-KubeConverter]
- [Kompox-ProviderDriver]
- [Kompox-ProviderDriver-AKS]
- [design/tasks/README.md]

[2026ab-k8s-node-pool-support]: ../../../plans/2026/2026ab-k8s-node-pool-support.ja.md
[K4x-ADR-019]: ../../../adr/K4x-ADR-019.md
[Kompox-KubeConverter]: ../../../v1/Kompox-KubeConverter.ja.md
[Kompox-ProviderDriver]: ../../../v1/Kompox-ProviderDriver.ja.md
[Kompox-ProviderDriver-AKS]: ../../../v1/Kompox-ProviderDriver-AKS.ja.md
[design/tasks/README.md]: ../../README.md
