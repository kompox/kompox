---
id: 2026ab-k8s-node-pool-support
title: K8s プラットフォームドライバへの NodePool 対応追加
version: v1
status: draft
updated: 2026-02-17T01:57:00Z
language: ja
adrs:
  - K4x-ADR-019
tasks:
  - 20260216a-nodepool-providerdriver-spec
  - 20260216b-nodepool-aks-spec
  - 20260216c-nodepool-kubeconverter-spec
  - 20260216d-nodepool-aks-driver-impl
---

# Plan: K8s プラットフォームドライバへの NodePool 対応追加

## 目的

- Provider Driver の公開契約に NodePool 管理を追加し、クラスタ作成後の Day2 運用でノードプールを動的に追加・更新・削除できるようにする。
- AKS で先行して実装可能な最小スコープ(List/Create/Update/Delete)を定義し、他ベンダ(OKE/GKE/EKS)へ展開しやすい抽象を確立する。
- App 側の `spec.deployment.pool/zone/pools/zones` を初期実装対象として整備し、KubeConverter のラベル契約(`kompox.dev/node-pool`, `kompox.dev/node-zone`)と整合する。

## 非目的

- 本 plan では全クラウドの同時実装は行わない。
- K3s のようにクラウド管理プレーンの NodePool リソースがない環境への完全対応は対象外とする。
- App/KOM の大幅なスキーマ再設計は行わない(既存 `deployment.pool/zone` を前提に最小差分で進める)。
- `deployment.selectors` の汎用選択式は将来拡張として予約し、本 plan の初期実装スコープでは実装しない。

## 背景

- 現状 AKS はクラスタ作成時のテンプレート入力で system/user pool 構成が実質固定化されるため、ゾーン可用性やキャパシティ変動への追従が難しい。
- AKS は公式 API で Agent Pool の CRUD を提供しており、Driver 抽象を追加すれば Day2 運用に対応できる。
- 用語はベンダごとに差異があるが、Kompox の公開契約では `NodePool` を共通語として採用し、実装で AKS Agent Pool / EKS Node Group へマッピングする。

## 対象となる design docs

- In scope:
  - [Kompox-ProviderDriver]: Driver インターフェースに NodePool 管理メソッド分類を追加。
  - [Kompox-ProviderDriver-AKS]: AKS 実装で NodePool 抽象を Agent Pool API にマッピングする仕様を追加。
  - [Kompox-KubeConverter]: `deployment.pool/zone` と `kompox.dev/node-pool` / `kompox.dev/node-zone` の契約を NodePool 対応方針と整合。
- Out of scope:
  - [Kompox-CLI] の詳細コマンド仕様の全面改訂。
  - 各ベンダ実装コードの一斉追加。

## 差分サマリ

- Provider Driver 契約に NodePool 管理カテゴリを追加する。
  - 最小: `NodePoolList`, `NodePoolCreate`, `NodePoolUpdate`, `NodePoolDelete`
  - `Get` は当面 `List` + 名前解決で吸収し、将来拡張余地として扱う。
- `要求事項(横断)` を MVP 必須項目と将来検討項目に分割し、各要求項目を簡潔な 1 行解説で維持する。
- ベンダ差異の吸収方針を明記する。
  - 共通契約名は `NodePool`。
  - AKS 実装名は `Agent Pool` としてマッピング。
  - EKS は `Node Group` にマッピング。
- 設定モデルは「共通項目を型付き」「ベンダ拡張は限定的 escape hatch」で整理する。
  - `map[string]string` 単独の主契約化は避ける。
- KubeConverter 契約は `pool/zone` の互換維持に加えて `pools/zones` を初期実装へ取り込み、`kompox.dev/node-pool` を中心に運用する。
  - zone 値のベンダ差異は driver 側の正規化責務として整理する。
  - `deployment.selectors` は将来拡張として予約し、現時点では未サポート(バリデーションエラー)とする。
- CLI 契約は別フェーズとして `kompoxops cluster nodepool` 系コマンド (`list/create/update/delete`) を追加し、driver NodePool API への操作経路を提供する。

## 計画 (チェックリスト)

- [x] Phase 1: Provider Driver 契約に NodePool 管理カテゴリを追加する。
  - [x] Task: [20260216a-nodepool-providerdriver-spec]
  - [x] [Kompox-ProviderDriver] に `NodePoolList/Create/Update/Delete` を追加する。
  - [x] `Get` は当面 `List` + 名前解決で吸収する方針を明記する。
  - [x] `要求事項(横断)` を MVP 必須項目/将来検討項目に分割し、各項目を簡潔化する。
- [x] Phase 2: AKS 実装方針を Agent Pool API マッピングとして定義する。
  - [x] Task: [20260216b-nodepool-aks-spec]
  - [x] [Kompox-ProviderDriver-AKS] に AKS Agent Pool CRUD の対応方針を追加する。
  - [x] 必須項目/可変項目/非対応項目、冪等性、`not implemented` 境界を明記する。
  - [x] [Kompox-ProviderDriver] に AKS 実装から抽出した全ドライバ共通原則を反映し、境界を整理する。
  - [x] [Kompox-ProviderDriver-AKS] に NodePool メソッド実装 (`NodePoolList/Create/Update/Delete`) の実装準拠記載を追加する。
- [x] Phase 3: KubeConverter 契約の責務分離を明確化する。
  - [x] Task: [20260216c-nodepool-kubeconverter-spec]
  - [x] [Kompox-KubeConverter] に `deployment.pool/zone/pools/zones` と NodePool 抽象の関係を追記する。
  - [x] `deployment.selectors` を将来拡張として予約し、現時点の未サポート方針を明記する。
  - [x] `kompox.dev/node-pool` / `kompox.dev/node-zone` を維持し、zone 正規化責務を driver 側に置くことを明記する。
- [x] Phase 4: AKS driver の NodePool 実装を追加する。
  - [x] Task: [20260216d-nodepool-aks-driver-impl]
  - [x] `adapters/drivers/provider/registry.go` に追加する NodePool メソッド契約に合わせて `adapters/drivers/provider/aks` の `driver` 実装を更新する。
  - [x] AKS の Agent Pool API を呼び出す実装(`List/Create/Update/Delete`)を追加し、`NodePool` 抽象へマッピングする。
  - [x] `NodePoolUpdate` の更新可能項目を明示し、未対応項目は `not implemented` / validation error として扱う。
- [ ] Phase 5: CLI の NodePool 操作コマンドを実装する。
  - [ ] `kompoxops cluster nodepool list --cluster-id <clusterID>` を追加する。
  - [ ] `kompoxops cluster nodepool create --cluster-id <clusterID> ...` を追加する。
  - [ ] `kompoxops cluster nodepool update --cluster-id <clusterID> ...` を追加する。
  - [ ] `kompoxops cluster nodepool delete --cluster-id <clusterID> --name <poolName>` を追加する。
  - [ ] [Kompox-CLI] 設計との整合を確認し、必要に応じて task 化して追跡する。
- [ ] Phase 6: AKS の NodePool ラベル/ゾーン整合を実装する。
  - [ ] 追加・更新される Agent Pool に `kompox.dev/node-pool` / `kompox.dev/node-zone` ラベルを一貫して設定する。
  - [ ] `deployment.pool/zone` のスケジューリング指定と、AKS 側 NodePool 設定の整合チェックを実装する。
- [ ] Phase 7: テストと検証を追加する。
  - [ ] AKS driver の NodePool API 呼び出しに対する unit test を追加する。
  - [ ] 既存 AKS E2E シナリオに NodePool の追加/更新/削除ケースを追加する。
  - [ ] 既存機能(ClusterProvision/Install、Volume 系)の回帰がないことを確認する。
- [ ] Phase 8: 実装タスクへ分割する。
  - [ ] 契約変更、AKS 実装、テスト更新を task file として分割する。
- [ ] Phase 9: ADR ステータス判定を行う。
  - [ ] 現時点では [K4x-ADR-019] は `proposed` を維持する。
  - [ ] 次の条件を満たした時点で [K4x-ADR-019] を `accepted` に変更する。
    - [ ] Phase 1〜3 の design docs 更新が完了している。
    - [ ] AKS driver の NodePool 実装とテスト(Phase 4〜7)が完了している。
    - [ ] 未対応ドライバでの `not implemented` 挙動と互換性方針が確認されている。

## リスク/未解決点

- `NodePoolUpdate` の更新可能項目はベンダ差が大きく、共通契約の境界を明確にしないと互換性問題が起きる。
- zone の表現(論理値 `1/2/3` vs ベンダ固有値)をどこで正規化するかを明確化する必要がある。
- `List` のみで `Get` を吸収する方針は API 単純化に有利だが、大規模時の効率要件が将来顕在化する可能性がある。

## Migration notes

- 既存の App `spec.deployment.pool/zone` は互換維持し、`spec.deployment.pools/zones` を初期実装で追加する。
- `spec.deployment.selectors` は将来拡張として予約し、現時点で指定された場合はバリデーションエラーとする。
- 既存 AKS クラスタは、NodePool API 導入後も現行設定(`AZURE_AKS_SYSTEM_*`, `AZURE_AKS_USER_*`)を初期値として扱い、段階的に Day2 管理へ移行する。
- NodePool 未対応ドライバは明示的 `not implemented` を返し、機能可否を利用側で判定可能にする。

## 進捗

- 2026-02-17T01:49:46Z Phase 4 実装完了を確認。`197f66a7983b33cc349ec5e50e4a6b57a12d61d6` にて NodePool 契約拡張、AKS driver 実装(List/Create/Update/Delete)、k3s 未対応スタブ、関連ユニットテストが追加された
- 2026-02-17T01:49:46Z 検証として `make build` / `make test` が成功し、Phase 4 を完了に更新
- 2026-02-17T01:57:00Z CLI 実装がテスト前提であることを反映し、`kompoxops cluster nodepool` コマンド実装を Phase 5 に前倒し。後続フェーズを再採番

## 参照

- [design/plans/README.md]
- [Kompox-ProviderDriver]
- [Kompox-ProviderDriver-AKS]
- [Kompox-KubeConverter]
- [Kompox-CLI]
- [K4x-ADR-019]
- [20260216a-nodepool-providerdriver-spec]
- [20260216b-nodepool-aks-spec]
- [20260216c-nodepool-kubeconverter-spec]
- [20260216d-nodepool-aks-driver-impl]

[design/plans/README.md]: ../README.md
[Kompox-ProviderDriver]: ../../v1/Kompox-ProviderDriver.ja.md
[Kompox-ProviderDriver-AKS]: ../../v1/Kompox-ProviderDriver-AKS.ja.md
[Kompox-KubeConverter]: ../../v1/Kompox-KubeConverter.ja.md
[Kompox-CLI]: ../../v1/Kompox-CLI.ja.md
[K4x-ADR-019]: ../../adr/K4x-ADR-019.md
[20260216a-nodepool-providerdriver-spec]: ../../tasks/2026/02/16/20260216a-nodepool-providerdriver-spec.ja.md
[20260216b-nodepool-aks-spec]: ../../tasks/2026/02/16/20260216b-nodepool-aks-spec.ja.md
[20260216c-nodepool-kubeconverter-spec]: ../../tasks/2026/02/16/20260216c-nodepool-kubeconverter-spec.ja.md
[20260216d-nodepool-aks-driver-impl]: ../../tasks/2026/02/16/20260216d-nodepool-aks-driver-impl.ja.md
