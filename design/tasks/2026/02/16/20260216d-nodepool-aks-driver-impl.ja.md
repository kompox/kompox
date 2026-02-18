---
id: 20260216d-nodepool-aks-driver-impl
title: AKS Driver の NodePool 実装 (Phase 4)
status: done
updated: 2026-02-18T01:30:40Z
language: ja
owner: yaegashi
adrs:
  - K4x-ADR-019
plans:
  - 2026ab-k8s-node-pool-support
---
# タスク: AKS Driver の NodePool 実装 (Phase 4)

本タスクは、Plan [2026ab-k8s-node-pool-support] の Phase 4 を具体化する作業項目です。

## 目的

- [Kompox-ProviderDriver] の NodePool 契約 (`NodePoolList/Create/Update/Delete`) を実装コードへ反映する。
- [Kompox-ProviderDriver-AKS] の設計に従い、AKS Agent Pool API と Kompox NodePool 抽象のマッピングを実装する。
- mutable/immutable 境界を `NodePoolUpdate` に反映し、未対応項目のエラー方針を明確化する。

## スコープ / 非スコープ

- 対象:
  - `adapters/drivers/provider/registry.go` の Driver 契約へ NodePool メソッドを追加
  - `adapters/drivers/provider/aks` に `NodePoolList/Create/Update/Delete` 実装を追加
  - AKS Agent Pool API 呼び出し層と DTO マッピングの実装
  - immutable 項目変更時の validation error / 未対応項目の `not implemented` 返却方針の実装
- 非対象:
  - 他クラウドドライバ (k3s/oke/gke/eks) の実装
  - KubeConverter の追加仕様変更 (`deployment.selectors` の実装など)
  - CLI コマンド実装 (`kompoxops cluster nodepool ...`) の追加
  - E2E シナリオ拡張 (別タスクで実施)

## 仕様サマリ

- NodePool 共通契約は [Kompox-ProviderDriver] を正とし、AKS 固有差異は [Kompox-ProviderDriver-AKS] のマッピング規則に従って吸収する。
- `NodePoolList` は一覧を返し、必要に応じて name filter で `Get` 相当を実現する。
- `NodePoolUpdate` は mutable フィールドのみ適用し、immutable フィールド更新要求は validation error とする。
- 未対応フィールドは `not implemented` を返し、利用側で判別可能にする。

## 計画 (チェックリスト)

- [x] Driver 契約に NodePool 4メソッドを追加し、既存 driver 実装との整合を取る。
- [x] AKS driver に NodePool API クライアント初期化を追加する。
- [x] `NodePoolList` 実装を追加する (Agent Pool 一覧→NodePool DTO 変換)。
- [x] `NodePoolCreate` 実装を追加する (必須項目バリデーションを含む)。
- [x] `NodePoolUpdate` 実装を追加する (mutable/immutable 境界を適用)。
- [x] `NodePoolDelete` 実装を追加する (`NotFound` は成功扱い)。
- [x] エラーモデル (`validation error` / `not implemented`) を統一する。
- [x] `make build` / `make test` で回帰確認する。

## テスト

- ユニット:
  - AKS driver の NodePool 変換ロジック
  - Update の mutable/immutable 判定
  - NotFound の冪等 delete 挙動
- スモーク:
  - `make build` が成功する。
  - `make test` が成功する。

## 受け入れ条件

- `registry.go` の Driver 契約に NodePool 4メソッドが追加されている。
- AKS driver に NodePool `List/Create/Update/Delete` 実装が追加されている。
- `NodePoolUpdate` の immutable 項目変更が validation error で扱われる。
- 未対応項目が `not implemented` として判別可能に返却される。

## 備考

- リスク:
  - AKS API で更新可能な項目の制約が厳しく、共通 DTO との対応を誤ると破壊的更新になる可能性がある。
- フォローアップ:
  - Phase 5 (ラベル/ゾーン整合) および Phase 6 (テスト拡張) へ接続する。

## 進捗

- 2026-02-16T19:13:57Z タスクファイルを作成
- 2026-02-17T01:49:46Z cloud agent 実装コミット (`197f66a7983b33cc349ec5e50e4a6b57a12d61d6`) を確認。`domain/model/nodepool_port.go` 追加、`registry.go` 契約拡張、`adapters/drivers/provider/aks/nodepool.go` の List/Create/Update/Delete 実装、`k3s` の未対応スタブ、関連テスト追加が反映されていることを確認
- 2026-02-17T01:49:46Z 検証として `make build` / `make test` を実行し成功。受け入れ条件を満たしたため本タスクを done 化
- 2026-02-17T01:53:54Z スコープ境界を明確化し、CLI コマンド実装 (`kompoxops cluster nodepool ...`) は本タスクの out of scope として明記

## 参照

- [2026ab-k8s-node-pool-support]
- [K4x-ADR-019]
- [Kompox-ProviderDriver]
- [Kompox-ProviderDriver-AKS]
- [design/tasks/README.md]

[2026ab-k8s-node-pool-support]: ../../../../plans/2026/2026ab-k8s-node-pool-support.ja.md
[K4x-ADR-019]: ../../../../adr/K4x-ADR-019.md
[Kompox-ProviderDriver]: ../../../../v1/Kompox-ProviderDriver.ja.md
[Kompox-ProviderDriver-AKS]: ../../../../v1/Kompox-ProviderDriver-AKS.ja.md
[design/tasks/README.md]: ../../../README.md
