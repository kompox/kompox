---
id: 2026ad-oke-driver-implementation
title: OKE Provider Driver 実装
version: v1
status: active
updated: 2026-02-27T11:08:13Z
language: ja
adrs: []
tasks:
  - 20260227a-oke-driver-design-doc
---

# Plan: OKE Provider Driver 実装

## 目的

- OKE Provider Driver の MVP レベル実装を完了する。
- [Kompox-ProviderDriver-OKE-DesignStudy] の方針に沿って、MVP で必要な機能範囲と未対応境界を明確化したうえで実装する。
- 既存の Provider Driver 契約を維持し、`driver: oke` を実運用に向けた最小成立ラインまで到達させる。

## 非目的

- OKE 以外のクラウドドライバ (AKS/K3s 以外) の同時実装。
- OKE 固有機能の全面展開 (Cluster Autoscaler 同時導入、OCIR 自動 Secret 配布など)。
- CLI 体系や usecase 公開契約の大幅な再設計。

## 仕様サマリ

- OKE 実装は [Kompox-ProviderDriver-OKE-DesignStudy] を基準にし、AKS 参照実装と同等の責務分割で進める。
- ステート管理は ARM deployment outputs ではなく、コンパートメント Freeform Tags を単一情報源として採用する。
- 実装方式は IaC 埋め込みではなく OCI Go SDK の `ensure*()` ベースとし、収束型・冪等実装を前提にする。
- MVP スコープは `instance_principal` + `user_principal` 認証、CA 非対応、OCIR 詳細後続とする。
- 破壊系操作は子リソース削除を先行し、最後にコンパートメントを削除する順序を必須とする。
- Provider Driver 契約は維持し、追加変更は `oke` ドライバ内部に閉じる。
- 未対応機能は明示的に `not implemented` を返し、呼び出し側で判定可能にする。
- 既存 AKS/K3s 経路への非影響を回帰条件として扱う。

## 関連ドキュメント

- [Kompox-ProviderDriver-OKE] - OKE Driver の実装規約・責務分割・MVP境界を定義する本体ドキュメント（本計画で新規作成）。
- [Kompox-ProviderDriver-OKE-DesignStudy] - OKE 対応の設計検討と方針の基準文書。認証・ステート管理・各機能差分の前提を確認する。
- [Kompox-ProviderDriver] - 全 Provider Driver 共通の契約・エラー方針・実装原則を定義する上位ドキュメント。
- [Kompox-ProviderDriver-AKS] - 既存実装の参照先。責務分割、命名、ロギング、実装パターンの比較に用いる。
- [2026ac-aks-arm-rest-migration] - `ensure*()` ベースの収束型実装方針を確認する関連 plan。実装アプローチの整合確認に用いる。

## 計画 (チェックリスト)

- [ ] Phase 1: OKE Driver の設計基準ドキュメントを作成する。
  - [x] Task: [20260227a-oke-driver-design-doc]
  - [ ] `design/v1/Kompox-ProviderDriver-OKE.ja.md` を新規作成する。
  - [ ] [Kompox-ProviderDriver-OKE-DesignStudy] の方針を、実装規約(必須/任意/非対応)として再編する。
  - [ ] MVP 範囲(`instance_principal` + `user_principal`、CA非対応、OCIR後続)を明記する。
- [ ] Phase 2: Provider 共通契約との整合を固定する。
  - [ ] [Kompox-ProviderDriver] の契約に対する OKE マッピング方針を [Kompox-ProviderDriver-OKE] へ反映する。
  - [ ] `not implemented` 境界と、MVP後続範囲の境界を文書で明確化する。
  - [ ] 命名/タグキー/エラー方針を AKS と同系統で統一する。
- [ ] Phase 3: OKE driver パッケージ骨格を追加する。
  - [ ] `adapters/drivers/provider/oke` を追加し、`driver`/`logging`/`naming`/`settings` の基礎ファイルを作成する。
  - [ ] `cmd/kompoxops/main.go` に `oke` driver 登録の blank import を追加する。
  - [ ] `go.mod` に OCI SDK 依存を追加し、ビルド可能な最小状態にする。
- [ ] Phase 4: 認証ファクトリと OCI クライアント基盤を実装する。
  - [ ] `OCI_AUTH_METHOD` の `instance_principal` / `user_principal` を実装する。
  - [ ] 共通クライアント生成と Work Request ポーリングユーティリティを追加する。
  - [ ] 設定値バリデーションと共通エラー分類を実装する。
- [ ] Phase 5: Cluster ライフサイクルの MVP 経路を実装する。
  - [ ] `ClusterProvision` を `ensure*()` 連鎖で実装し、コンパートメントタグを単一情報源として記録する。
  - [ ] `ClusterInstall` は DNS/Registry/IAM の最小要件に限定し、CA は未対応として明示する。
  - [ ] `ClusterDeprovision` は依存順序(子リソース削除→コンパートメント削除)で冪等実装する。
- [ ] Phase 6: NodePool の MVP 機能を実装する。
  - [ ] `NodePoolList/Create/Update/Delete` を OKE API にマッピングして実装する。
  - [ ] mutable/immutable 境界を実装し、未対応更新は validation error / not implemented とする。
  - [ ] AD 正規化ルールを MVP 仕様として固定する。
- [ ] Phase 7: Volume と DNS の MVP 機能を実装する。
  - [ ] `VolumeDisk*` と `VolumeFiles*` の最小機能を実装し、assigned タグ契約を適用する。
  - [ ] `ClusterDNSApply` を OCI DNS API で実装し、Zone OCID 入力契約を確定する。
  - [ ] OCIR 詳細(自動 secret 配布)は後続タスクとして明示する。
- [ ] Phase 8: CLI 接続・検証を実装する。
  - [ ] 既存 usecase/port 経路で `driver: oke` が呼び出されることを確認する。
  - [ ] OKE 向け最小 fixture を追加し、MVP 経路の統合テストを整備する。
  - [ ] AKS/K3s の既存経路に回帰がないことを確認する。
- [ ] Phase 9: 文書同期と完了条件を確定する。
  - [ ] [Kompox-ProviderDriver-OKE] / [Kompox-ProviderDriver] / [Kompox-CLI] の同期更新を行う。
  - [ ] 本 plan の `tasks` と進捗を実績に合わせて更新する。
  - [ ] MVP 完了判定基準(実装/テスト/未対応境界)を明記して `active` 化する。

## リスク/未解決点

- OKE Workload Identity アノテーション仕様が未確定で、`ClusterInstall` 実装の境界に影響する。
- Availability Domain の正規化規則が未確定で、NodePool の zone 指定互換に影響する。
- File Storage の Mount Target 配置方針 (クラスタ流用 or アプリ独立) によりネットワーク前提が変わる。
- OCI SDK 依存導入後の API version 差分や Work Request 待機時間がテスト時間に影響する。

## Migration notes

- Provider Driver 契約は維持し、追加実装は `oke` ドライバ内部に閉じる。
- OKE 未対応機能は明示的 `not implemented` を返し、呼び出し側で判定可能にする。
- 既存 AKS/K3s の挙動を変えないことを回帰条件とする。

## 進捗

- 2026-02-27T10:44:27Z OKE Provider Driver MVP 実装計画の新規 plan を作成

## 参照

- [design/plans/README.md]
- [Kompox-ProviderDriver-OKE]
- [Kompox-ProviderDriver-OKE-DesignStudy]
- [Kompox-ProviderDriver]
- [Kompox-ProviderDriver-AKS]
- [2026ac-aks-arm-rest-migration]
- [Kompox-KOM]
- [Kompox-CLI]
- [20260227a-oke-driver-design-doc]

[design/plans/README.md]: ../README.md
[Kompox-ProviderDriver-OKE]: ../../v1/Kompox-ProviderDriver-OKE.ja.md
[Kompox-ProviderDriver-OKE-DesignStudy]: ../../v1/Kompox-ProviderDriver-OKE-DesignStudy.ja.md
[Kompox-ProviderDriver]: ../../v1/Kompox-ProviderDriver.ja.md
[Kompox-ProviderDriver-AKS]: ../../v1/Kompox-ProviderDriver-AKS.ja.md
[2026ac-aks-arm-rest-migration]: ./2026ac-aks-arm-rest-migration.ja.md
[Kompox-KOM]: ../../v1/Kompox-KOM.ja.md
[Kompox-CLI]: ../../v1/Kompox-CLI.ja.md
[20260227a-oke-driver-design-doc]: ../../tasks/2026/02/27/20260227a-oke-driver-design-doc.ja.md
