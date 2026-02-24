---
id: 2026ac-aks-arm-rest-migration
title: AKS Provider Driver の ARM REST API ベース移行
version: v1
status: draft
updated: 2026-02-24T10:31:42Z
language: ja
adrs:
  - K4x-ADR-020
tasks: []
---

# Plan: AKS Provider Driver の ARM REST API ベース移行

## 目的

- AKS Provider Driver のクラスタ作成/削除経路を、ARM Templates デプロイ依存から ARM REST API に移行する。
- `ensure*()` メソッドによる収束型(冪等)実装へ置き換え、再実行時の安全性を高める。
- 運用コスト要件を維持するため、監視データの保存先として Azure Storage を継続利用する。

## 非目的

- NodePool / Volume / Secret など、クラスタ作成/削除以外の全ライフサイクルを同時に移行すること。
- ACR ライフサイクル管理を AKS Driver の責務に含めること。
- Key Vault を新規作成・管理すること。

## 背景

- 現行 AKS Driver は subscription-scope の ARM deployment と deployment outputs を中心に構成されている。
- design では deployment outputs 依存のリスクが既知であり、将来的なメタデータ管理見直しが示唆されている。
- 今回は変更範囲を最小化しつつ、AKS クラスタの作成/削除経路を REST API ベースへ置き換える。

## 対象となる design docs

- In scope:
  - [Kompox-ProviderDriver]: Provider Driver 共通の設計原則(冪等性、責務分離、エラー方針)を更新する。
  - [Kompox-ProviderDriver-AKS]: AKS 固有の ARM REST API 呼び出し順序、必須/任意リソース、出力契約を更新する。
- Out of scope:
  - [Kompox-CLI] のコマンド体系変更。
  - ACR の作成/権限付与を含む AKS 外部依存の統合管理。

## 差分サマリ

- ARM deployment 一括実行を廃止し、AKS driver 内でリソース単位 `ensure*()` を実行する。
- リソース方針は次を採用する。
  - 必須: Resource Group, Log Analytics, Storage Account, User Assigned Managed Identity, AKS Managed Cluster, Federated Identity Credential。
  - 維持: AKS Diagnostic Settings の Storage 出力。
  - 除外: Key Vault 一式、ACR 一式、Application Insights Dashboard。
- 1 リソース 1 メソッドの実装単位を基本としつつ、呼び出し境界は段階的オーケストレーションで統制する。

## 計画 (チェックリスト)

- [ ] Phase 1: AKS driver の ARM REST API 実行基盤を追加する。
  - [ ] ARM 認証情報(TokenCredential)を再利用する REST クライアント責務を明確化する。
  - [ ] 非同期 LRO ポーリングの最小共通処理(作成/削除)を定義する。
  - [ ] HTTP エラー分類(401/403/404/409/429/5xx)を最小統一する。
- [ ] Phase 2: クラスタ作成経路を `ensure*()` へ置換する。
  - [ ] `ensureAKSResourceGroupCreated`
  - [ ] `ensureAKSLogAnalyticsWorkspaceCreated`
  - [ ] `ensureAKSStorageAccountCreated`
  - [ ] `ensureAKSUserAssignedIdentityCreated`
  - [ ] `ensureAKSManagedClusterCreated`
  - [ ] `ensureAKSManagedClusterDiagnosticsConfigured`
  - [ ] `ensureAKSFederatedIdentityCredentialCreated`
- [ ] Phase 3: クラスタ削除経路を `ensure*()` / `delete*()` へ置換する。
  - [ ] 作成時と同一命名規則で対象リソースを再発見する。
  - [ ] NotFound を成功扱いにする冪等削除規約を適用する。
  - [ ] 長時間削除の状態遷移をポーリングで収束させる。
- [ ] Phase 4: 出力契約とメタデータ参照を整理する。
  - [ ] クラスタ識別に必要な値の取得経路を deployment outputs 依存から分離する。
  - [ ] UAI/FIC 前提の install 経路で必要な識別子を取得できることを確認する。
  - [ ] 既存 usecase/domain 契約を維持し、adapter 内部差し替えに閉じる。
- [ ] Phase 5: 検証と文書同期を行う。
  - [ ] `make build` / `make test` で回帰がないことを確認する。
  - [ ] AKS E2E の provision/deprovision 経路で動作確認する。
  - [ ] 関連設計文書を更新し、差分の責務境界を明文化する。

## リスク/未解決点

- ARM REST API 直叩きでは API version 管理を driver 側で持つ必要がある。
- RBAC 反映遅延や identity 伝播遅延により、FIC 作成の初回失敗が発生する可能性がある。
- deployment outputs 依存を外す過程で、既存 install/secret 系処理との契約差分が顕在化する可能性がある。

## Migration notes

- 既存運用との互換性を優先し、初期移行ではクラスタ作成/削除のみを対象とする。
- ACR は AKS Driver 管理対象から切り離し、必要時は別責務で扱う。
- Key Vault は現行運用に合わせて作成対象から除外する。
- Azure Storage は Log Analytics コスト補完の代替保存先として維持する。

## 参照

- [design/plans/README.md]
- [Kompox-ProviderDriver]
- [Kompox-ProviderDriver-AKS]
- [Kompox-CLI]

[design/plans/README.md]: ../README.md
[Kompox-ProviderDriver]: ../../v1/Kompox-ProviderDriver.ja.md
[Kompox-ProviderDriver-AKS]: ../../v1/Kompox-ProviderDriver-AKS.ja.md
[Kompox-CLI]: ../../v1/Kompox-CLI.ja.md
