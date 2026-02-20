---
id: 2026ab-k8s-node-pool-support
title: K8s プラットフォームドライバへの NodePool 対応追加
version: v1
status: done
updated: 2026-02-20T00:20:00Z
language: ja
adrs:
  - K4x-ADR-019
tasks:
  - 20260216a-nodepool-providerdriver-spec
  - 20260216b-nodepool-aks-spec
  - 20260216c-nodepool-kubeconverter-spec
  - 20260216d-nodepool-aks-driver-impl
  - 20260217a-nodepool-cli-impl
  - 20260217b-nodepool-cli-e2e-before-label-zone
  - 20260217c-kom-app-deployment-impl
  - 20260217d-nodepool-doc-sync
  - 20260218a-nodepool-tests
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
- Phase 5 実装では `create/update` を `--file` 必須の YAML/JSON 入力方式とし、YAML を正・JSON を互換入力として扱う。
- Phase 5 実装では `usecase/nodepool` の公開 DTO に JSON タグ (`snake_case`) を追加し、UseCase DTO 規約との整合を確保する。
- NodePool の file spec (`nodePoolSpec`) は pointer フィールドを利用し、未指定と zero-value を区別して update 時の部分更新意図を保持する。

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
- [x] Phase 5: CLI の NodePool 操作コマンドを実装する。
  - [x] Task: [20260217a-nodepool-cli-impl]
  - [x] `kompoxops cluster nodepool list --cluster-id <clusterID>` を追加する。
  - [x] `kompoxops cluster nodepool create --cluster-id <clusterID> ...` を追加する。
  - [x] `kompoxops cluster nodepool update --cluster-id <clusterID> ...` を追加する。
  - [x] `kompoxops cluster nodepool delete --cluster-id <clusterID> --name <poolName>` を追加する。
  - [x] [Kompox-CLI] 設計との整合を確認し、差分は [20260217a-nodepool-cli-impl] で追跡する。
- [x] Phase 6: AKS の NodePool ラベル/ゾーン整合を実装する。
  - [x] Task: [20260217c-kom-app-deployment-impl]
  - [x] 着手前提として [20260217b-nodepool-cli-e2e-before-label-zone] の受け入れ条件を満たす。
  - [x] 追加・更新される Agent Pool に `kompox.dev/node-pool` / `kompox.dev/node-zone` ラベルを一貫して設定する。
  - [x] `deployment.pool/zone` のスケジューリング指定と、AKS 側 NodePool 設定の整合チェックを実装する。
- [x] Phase 7: テストと検証を追加する。
  - [x] Task: [20260217b-nodepool-cli-e2e-before-label-zone]
  - [x] Task: [20260218a-nodepool-tests]
  - [x] `cmd/kompoxops cluster nodepool` の E2E テスト(create/update/delete/list)を先行追加し、Phase 6 の回帰基準を固定化する。
  - [x] AKS driver の NodePool unit test（変換/validation/冪等判定）を追加する。
  - [x] `cmd/kompoxops cluster nodepool` のコマンド層テスト(引数バリデーション/呼び出し経路)を追加する。
  - [x] 既存 AKS E2E シナリオに NodePool の追加/更新/削除ケースを追加する。
  - [x] 既存機能(ClusterProvision/Install、Volume 系)の回帰がないことを確認する。
- [x] Phase 8: NodePool 対応の設計ドキュメントを同期する。
  - [x] Task: [20260217d-nodepool-doc-sync]
  - [x] [Kompox-ProviderDriver] に NodePool 実装/テスト進捗の最新状態を反映する。
  - [x] [Kompox-ProviderDriver-AKS] に AKS 側の NodePool 運用・検証手順の最新状態を反映する。
  - [x] [Kompox-CLI] に `cluster nodepool` コマンド群の最新仕様と検証観点を反映する。
  - [x] [Kompox-KubeConverter] に `deployment.pool/zone/pools/zones` と NodePool 契約の接続点を最新状態へ同期する。
- [x] Phase 9: ADR ステータス判定を行う。
  - [x] [K4x-ADR-019] を `accepted` に変更する。
  - [x] 判定条件を満たしたことを確認する。
    - [x] Phase 1〜3 の design docs 更新が完了している。
    - [x] AKS driver の NodePool 実装とテスト(Phase 4〜7)が完了している。
    - [x] 未対応ドライバでの `not implemented` 挙動と互換性方針が確認されている。

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
- 2026-02-17T02:00:29Z Phase 5 実装タスク [20260217a-nodepool-cli-impl] を追加し、計画へ紐付け
- 2026-02-17T04:50:15Z PR #10 (`Add NodePool CLI commands (Phase 5)`) の `main` マージを確認。Phase 5 を完了化し、YAML/JSON file-input 仕様・DTO JSON タグ整備・pointer による partial update 意図保持を差分サマリへ反映
- 2026-02-17T07:05:46Z Phase 6 着手前に NodePool CLI E2E を先行追加する方針を反映し、Task [20260217b-nodepool-cli-e2e-before-label-zone] を追加
- 2026-02-17T08:27:51Z `tests/aks-e2e-nodepool` の通し実行成功を確認。Phase 6 着手前提と Phase 7 の先行 E2E 追加項目を完了へ更新
- 2026-02-17T09:18:35Z Phase 6 の KOM 定義 / KubeConverter 更新タスクとして [20260217c-kom-app-deployment-impl] を計画へ追加
- 2026-02-17T12:44:29Z [20260217c-kom-app-deployment-impl] を `done` とし、KOM/NodePool/Bicep のスケジューリングラベル方針整理を反映。現行 AKS 構成で `kompox.dev/node-pool` / `kompox.dev/node-zone` 契約との整合が取れるため、Phase 6 を完了へ更新
- 2026-02-17T23:03:50Z 残タスク整理を実施。Phase 7 の進捗を 2/6 完了として明記し、旧 Phase 8 (実装タスク分割) を削除。新たに `Kompox-ProviderDriver` / `Kompox-ProviderDriver-AKS` / `Kompox-CLI` の文書同期を Phase 8 として追加
- 2026-02-17T23:07:43Z Phase 8 着手タスク [20260217d-nodepool-doc-sync] を作成し、計画チェックリストと tasks 一覧に紐付け
- 2026-02-17T23:10:22Z Phase 8 対象文書に [Kompox-KubeConverter] を追加
- 2026-02-17T23:18:51Z [20260217d-nodepool-doc-sync] を完了。ProviderDriver/AKS/CLI/KubeConverter の NodePool 関連文書を同期し、Phase 8 を完了へ更新
- 2026-02-18T02:11:40Z Phase 7 残タスクとして [20260218a-nodepool-tests] を追加
- 2026-02-19T23:38:59Z [20260218a-nodepool-tests] を完了。CLI コマンド層テスト追加、NodePool E2E 拡張（`aks-e2e-nodepool` / `aks-e2e-basic`）、回帰検証（`aks-e2e-basic` / `aks-e2e-volume`）および `make gen-index` を実施
- 2026-02-20T00:15:00Z Phase 9 の判定条件充足を確認し、[K4x-ADR-019] を `accepted` へ更新

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
- [20260217a-nodepool-cli-impl]
- [20260217b-nodepool-cli-e2e-before-label-zone]
- [20260217c-kom-app-deployment-impl]
- [20260217d-nodepool-doc-sync]
- [20260218a-nodepool-tests]

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
[20260217a-nodepool-cli-impl]: ../../tasks/2026/02/17/20260217a-nodepool-cli-impl.ja.md
[20260217b-nodepool-cli-e2e-before-label-zone]: ../../tasks/2026/02/17/20260217b-nodepool-cli-e2e-before-label-zone.ja.md
[20260217c-kom-app-deployment-impl]: ../../tasks/2026/02/17/20260217c-kom-app-deployment-impl.ja.md
[20260217d-nodepool-doc-sync]: ../../tasks/2026/02/17/20260217d-nodepool-doc-sync.ja.md
[20260218a-nodepool-tests]: ../../tasks/2026/02/18/20260218a-nodepool-tests.ja.md
