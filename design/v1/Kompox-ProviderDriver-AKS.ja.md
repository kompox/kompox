---
title: Kompox Provider Driver AKS ガイド
version: v1
status: out-of-sync
updated: 2025-09-26
language: ja
---

# Kompox Provider Driver AKS ガイド v1

## 概要

本書は Kompox の AKS プロバイダドライバ（`adapters/drivers/provider/aks`）の設計と現行実装の振る舞いを解説します。コード実装に基づく現状仕様であり、将来計画は含みません。

## 対象バージョン / スコープ
* 対象: 現行 `main` ブランチ実装
* AKS API 利用: `Microsoft.ContainerService/managedClusters@2025-05-02-preview`
* Azure Deployment Stacks をサブスクリプションスコープで利用

## ドライバ概要
AKS クラスタおよび付随する監視 / レジストリ / Key Vault / ユーザ割当マネージドID等を **単一の Deployment Stack**（ARM テンプレート `main.json` 埋込）で一括作成 / 管理する。クラスタのインストール処理では Ingress (Traefik) をデフォルト名前空間（または指定値）に導入する。

## ドライバ識別子と関連名
| 項目 | 値 |
|------|----|
| Driver ID | `aks` |
| Deployment Stack 名 | `kompox_<ServiceName>_<ProviderName>_<ClusterName>` |
| 付与クラスタタグ (Azure 全リソース共通タグ) | `kompox-cluster` = `<ServiceName>/<ProviderName>/<ClusterName>` |
| 追加タグ | `managed-by=kompox` |

`ServiceName` はサービスが nil の場合 `(nil)` となる。

## 設定パラメータ
### Provider 設定 (必須 / 任意)
| キー | 必須 | 内容 |
|------|------|------|
| `AZURE_SUBSCRIPTION_ID` | 必須 | 操作対象サブスクリプション ID |
| `AZURE_LOCATION` | 必須 | 主要ロケーション (全リソース location) |
| `AZURE_AUTH_METHOD` | 必須 | 認証方式 (下記参照) |
| `AZURE_TENANT_ID` | 認証方式依存 | 一部方式で必須 |
| `AZURE_CLIENT_ID` | 認証方式依存 | 一部方式で必須 / オプション (Managed Identity の特定) |
| `AZURE_CLIENT_SECRET` | client_secret 方式必須 | クライアントシークレット |
| `AZURE_FEDERATED_TOKEN_FILE` | workload_identity 方式必須 | トークンファイルパス |

### Cluster 設定
| キー | 必須 | 内容 |
|------|------|------|
| `AZURE_RESOURCE_GROUP_NAME` | 必須 | 作成 / 利用するリソースグループ名。存在しない場合 Stack により作成。 |

クラスタ側では現状追加の AKS パラメータ (ノードプール詳細等) は受け取らない。

## 認証方式サポート
| `AZURE_AUTH_METHOD` | 状態 | 必須追加設定 | 備考 |
|---------------------|------|---------------|------|
| `client_secret` | サポート | `AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET` | Azure AD アプリ資格情報 |
| `client_certificate` | 非サポート | - | 実装内で未対応エラー返却 |
| `managed_identity` | サポート | (`AZURE_CLIENT_ID` 任意) | UAMI 指定可能、未指定でシステム割当 |
| `workload_identity` | サポート | `AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_FEDERATED_TOKEN_FILE` | OIDC Federation |
| `azure_cli` | サポート | なし | ローカル開発向け CLI 認証 |
| `azure_developer_cli` | サポート | なし | Azure Developer CLI 認証 |

上記以外は `unsupported AZURE_AUTH_METHOD` エラー。

## プロビジョニング動作
1. ClusterProvision 呼び出し時に `AZURE_RESOURCE_GROUP_NAME` を取得（未設定でエラー）。
2. 埋込 JSON テンプレート (`main.json`) をアンマーシャル。
3. Deployment Stack (サブスクリプションスコープ) を `BeginCreateOrUpdateAtSubscription` で開始。
4. 既に同名 Stack が `Succeeded` の場合はそのまま完了 (冪等性確保)。
5. Stack Parameters は以下のみ設定: `environmentName` = `cluster.Name`, `location` = Provider 設定, `resourceGroupName` = Cluster 設定値。その他テンプレート内パラメータはデフォルト扱い。
6. Poll 完了まで待機 (最大 30 分のコンテキストタイムアウト)。

### テンプレート (main.json) 主な処理
サブスクリプションスコープ → RG / 内部サブデプロイ群:
* Resource Group (存在しない場合作成)
* Key Vault
* Log Analytics Workspace / Application Insights (+ 任意ダッシュボード)
* Container Registry (ACR) + Diagnostic Settings
* User Assigned Managed Identity
* Federated Identity Credential (Traefik 用: subject=`system:serviceaccount:traefik:traefik`)
* AKS クラスタ
	* SystemAssigned ID
	* System ノードプール 1 つ (事前定義プリセット: CostOptimised/Standard/HighSpec から `CostOptimised` デフォルト)
	* `networkPlugin=azure`, `networkPolicy=azure`
	* Addon: azurepolicy(v2), KeyVault Secrets Provider(ローテーション有効), OMS Agent (Log Analytics 連携), Web App Routing, OIDC issuer, Workload Identity
	* SKU Tier: `Free` デフォルト
	* Kubernetes Version: テンプレート上デフォルト `1.29` (Stack 呼び出しでは明示 `1.33` をパラメータ投入)
* RBAC Role Assignments: Cluster Admin / ACR Pull / Key Vault Secrets User / Key Vault Certificates User
* Federated Identity Credential は AKS OIDC Issuer URL 出力を利用し Traefik SA に紐付け

### Deployment Stack Outputs
テンプレート outputs (一部):
| 出力キー | 説明 |
|----------|------|
| `AZURE_LOCATION` | Location |
| `AZURE_TENANT_ID` | Tenant ID |
| `AZURE_SUBSCRIPTION_ID` | Subscription ID |
| `AZURE_RESOURCE_GROUP_NAME` | 使用 RG 名 |
| `AZURE_AKS_CLUSTER_NAME` | AKS クラスタ名 |
| `AZURE_AKS_OIDC_ISSUER_URL` | AKS OIDC Issuer |
| `AZURE_CONTAINER_REGISTRY_ENDPOINT` | ACR Login Server |
| `AZURE_CONTAINER_REGISTRY_NAME` | ACR 名 |

ドライバ内部では `AZURE_RESOURCE_GROUP_NAME`, `AZURE_AKS_CLUSTER_NAME` を取得する。ARM 出力キー大文字化のため取得時に UpperCase 正規化。

## デプロビジョニング動作
`ClusterDeprovision` は Stack 存在チェック後、 `BeginDeleteAtSubscription` を実行し **管理対象リソース/リソースグループを削除** 指定。存在しない場合は成功扱い。タイムアウト 30 分。

## ステータス判定
`ClusterStatus`:
1. Stack 存在 & `Succeeded` で `Provisioned=true`。
2. 追加で AKS ManagedCluster の `ProvisioningState == Succeeded` を確認。
3. インストール状態は Kubernetes API で Ingress 名前空間が存在するかで判定 (`Installed=true`)。
	 * 名前空間: `cluster.Ingress["namespace"]` が string ならその値、未指定は `default`。
4. 取得不能エラー（認証・通信）はインストール判定を失敗扱いで握り潰し、Provisioned フラグのみ反映。

## インストール / アンインストール
`ClusterInstall`:
1. 管理者 kubeconfig 取得。
2. Ingress 用名前空間を作成 (存在すれば何もしない)。
3. Traefik をマニフェストで導入 (冪等)。

`ClusterUninstall`:
1. Traefik 削除 (ベストエフォート)。
2. Ingress 名前空間削除 (ベストエフォート / 冪等)。

## kubeconfig 取得
`ClusterKubeconfig` は Stack Outputs から RG / Cluster 名を再取得 → `ListClusterAdminCredentials` を呼び最初の kubeconfig を返す。タイムアウト 2 分。kubeconfig が無い場合エラー。

## タイムアウト設定
| 操作 | タイムアウト |
|------|--------------|
| Provision / Deprovision | 30 分 |
| Status | 5 分 |
| Kubeconfig 取得 | 2 分 |

## 付与される主なロール (Role Assignments)
| 対象 | ロール | 用途 |
|------|--------|------|
| AKS Cluster Scope | Cluster Admin Role (GUID `b1ff04bb-...`) | Azure RBAC 経由 Kubernetes 管理 |
| Key Vault | Key Vault Secrets User | CSI Secret Provider アドオン用 |
| Key Vault | Key Vault Certificates Officer | 証明書アクセス |
| ACR | AcrPull | AKS ノードからのイメージ Pull |

## リソース命名規則 (抜粋)
テンプレート内部変数 `abbrs` により各種既定プレフィクス (例: `aks-`, `rg-`, `kv-`, `cr`, `log-`, `appi-`) と `uniqueString` による `resourceToken` を結合。クラスタ名を直接反映するリソースは AKS DNS Prefix など一部。

## 制約 / 既知の制限
* System ノードプールのみ。追加プールや Windows ノード未対応。
* Kubernetes バージョンはコード内で `1.33` をパラメータ設定（テンプレートデフォルト `1.29`）し固定。任意指定不可。
* Cluster / Provider 設定からの AKS 詳細カスタマイズ (VM サイズ、オートスケール閾値他) は未対応。プリセット `CostOptimised` のみ使用。
* Update 系 API (スケール / バージョン更新) 非実装。
* `client_certificate` 認証方式未対応。
* インストール判定は Ingress 名前空間存在のみで Traefik Pod 健全性までは確認しない。
* kubeconfig は常に admin 資格。ユーザ/グループスコープ発行無し。

## エラーハンドリング指針 (実装現状)
| 事象 | 動作 |
|------|------|
| 必須設定欠如 | プロビジョニング開始前にエラー返却 |
| 認証方式パラメータ不足 | エラー返却 |
| Stack 取得失敗 (Status 判定) | 非既存扱い / Installed 判定スキップ |
| Ingress 名前空間問い合わせ 404 | Installed=false |
| Ingress 名前空間問い合わせ その他エラー | Installed=false 扱い (エラー表面化しない) |

## 今後拡張候補 (参考)
* 追加ノードプール構成・カスタムパラメータ受け付け
* Kubernetes バージョン選択 / 自動アップグレード方針
* Workload Identity / Federated Identity の動的追加
* Helm / CRDs インストールオーケストレーション抽象化

以上。

----

## Volume 管理 (Azure Disk)

本節は論理ボリューム `app.volumes[volName]` に紐づく Azure Managed Disk (以下 Volume Disk) の列挙 / 作成 / 割当 / 削除操作を定義する。

### 用語
- Logical Volume Key: `volName-idHASH`
- idHASH: [Kompox-Convert-Draft.ja.md](Kompox-Convert-Draft.ja.md) を参照
- Volume Disk Name (`diskName`): ULID (UTC, モノトニック生成) — 文字集合 Crockford Base32, 長さ 26。時刻順ソート可能。
- Azure Disk Name: `volName-idHASH-diskName`
- Assigned フラグ: タグ `kompox-disk-assigned` が `true` の Volume Disk はその Logical Volume に対し現在アクティブに選択された 1 個 (排他)。
  - 初回に作成された Volume Disk は自動的に `kompox-disk-assigned=true` で作成される。それ以降に作成されるディスクは `false`。

### 共通仕様
- 対象 Resource Group: `app.settings.AZURE_RESOURCE_GROUP_NAME`
  - Create 時: RG が存在しなければ作成。
  - List / Assign / Delete 時: RG 不存在なら空結果または冪等成功（副作用での新規作成は行わない）。
- 必要権限: `Microsoft.Compute/disks/read/write/delete`, `Microsoft.Resources/subscriptions/resourceGroups/read`, `.../resourceGroups/write` (Create 時)。
- フィルタ条件: 以下すべて
  1. Resource Group 一致
  2. タグ `kompox-volume` = `volName-idHASH`
  3. Disk 名がプレフィクス `volName-idHASH-` で始まる
- サイズ単位: GiB。最小 1GiB。整数。将来拡張で拡張 (Resize) 未実装。
- SKU: `Premium_LRS` (固定)。将来拡張時にパラメータ化。
- タグ操作は指定タグのみ更新（既存の他ユーザタグは保持 / Merge）。

### VolumeDiskList
- 処理: Resource Group 内の Managed Disk を列挙 → 上記フィルタで抽出 → `timeCreated` を取得しクライアント側で降順ソート。
- 返却項目例: `name`, `diskName`, `sizeGiB`, `assigned(bool)`, `timeCreated`.
- エラー: RG 不存在 → 空配列。Azure API エラーは上位へ伝播。

### VolumeDiskCreate
- 入力: `volName`
- 前提: `app.volumes[volName].sizeGiB` が仕様範囲内。
- Disk 名: `volName-idHASH-<ULID>`
- Tags:
  - `kompox-volume` = `volName-idHASH`
  - `kompox-disk-name` = `<ULID>`
  - `kompox-disk-assigned` = 初回ディスクのみ `true`、それ以外は `false`
  - `managed-by` = `kompox`
- 冪等性: 同名 Disk が既に存在する場合は衝突 (409) とし再生成不可（ULID 重複は極稀でありエラー扱い）。
- タイムアウト: 2 分 (推奨)。
- 戻り: 作成した Volume Disk のメタ。

### VolumeDiskAssign
- 入力: `volName`, `diskName`
- 手順:
  1. List を実行（最新状態取得）。
  2. 対象 Disk を特定。存在しなければ NotFound。
  3. まとめてタグ更新:
  - 指定 Disk: `kompox-disk-assigned=true`
     - その他同一 Logical Volume Disk: `...=false`
- 競合制御: なし（同一 Logical Volume に属する全ディスクのタグを順次更新して排他を担保）。
- 冪等性: 既に指定 Disk が唯一 true なら無変更で成功。
- タイムアウト: 1 分。

### VolumeDiskDelete
- 入力: `volName`, `diskName`
- 手順: 対象 Disk 名=`volName-idHASH-diskName` を取得し削除。存在しなければ冪等成功。
- 削除対象が `assigned=true` でも制約無し（運用側で事前に別インスタンスへ Assign することを推奨）。
- タイムアウト: 2 分。

### エラーハンドリング (Volume)
| 事象 | 挙動 |
|------|------|
| RG 不存在 (List/Assign/Delete) | List: 空配列 / Assign: NotFound / Delete: 成功扱い |
| 必須設定欠如 (`AZURE_RESOURCE_GROUP_NAME`) | 直ちにエラー |
| Disk API 429 / 5xx | バックオフリトライ (指数, 最大 3〜5 回) 後失敗 |
| Assign 競合 (ETag) | リトライ枯渇でエラー |
| NotFound (指定 diskName) | Delete: 成功 / Assign: エラー |
| タグ欠如 (不正リソース混入) | スキップ |
| タイムアウト | 操作失敗 (コンテキストエラー返却) |

### 将来拡張 (参考)
- Resize / Snapshot / 暗号化セット連携
- マルチ AZ / ZRS Disk
- Disk 暗号化 (CMK) オプション
- `assigned` を Kubernetes PVC 状態と同期する再協調処理

---

## Volume スナップショット (Azure Managed Disk Snapshot)

本節は論理ボリュームに属する Azure Managed Disk のスナップショットを列挙/作成/削除/復元する AKS ドライバ実装の仕様を定義する。

### 用語・命名
- Logical Volume Key: `volName-idHASH`（上記 Volume と同一）
- Snapshot 名 (`snapName`): ULID（UTC, モノトニック）
- Azure Snapshot Name: `volName-idHASH-<ULID>`
- 所属タグ:
  - `kompox-volume` = `volName-idHASH`
  - `kompox-snapshot-name` = `<ULID>`
  - `kompox-disk-name` = `<Disk ULID>`（作成元の Volume Disk の ULID）
  - `managed-by` = `kompox`

### 共通仕様
- 対象 Resource Group: Volume と同一（`app.settings.AZURE_RESOURCE_GROUP_NAME`）。
- フィルタ条件（List）:
  1. RG 一致
  2. タグ `kompox-volume` 一致
  3. リソース名がプレフィクス `volName-idHASH-` で始まる
- 返却は `CreatedAt` 降順（同時刻は名前昇順で安定化は実装依存）。
- SKU 既定: `Standard_LRS`（スナップショット）。
- 作成は Incremental Snapshot（`incremental=true`）。

### SnapshotList
- RG 内の Snapshot を列挙し、上記フィルタに合致するものを返す。RG 不存在は空配列。

### SnapshotCreate
- 入力: `diskName`（作成元 Volume Disk の ULID）。
- `CreationData` は `CreateOption=Copy`, `SourceResourceID` は作成元 Disk を指す。
- 戻り値: 作成した Snapshot のメタ（`snapName`, `handle`, `sizeGiB`, `createdAt`）。

### SnapshotDelete
- 入力: `snapName`（ULID）。
- 該当 Snapshot（`volName-idHASH-snapName`）を削除。`NotFound` は冪等成功。

### SnapshotRestore
- 入力: `snapName`（ULID）。
- 指定 Snapshot から新しい Volume Disk を作成する。
  - Disk `CreationData` は `CreateOption=Copy`, `SourceResourceID` に Snapshot を指定。
  - Disk SKU は Volume Disk と同じ `Premium_LRS`。
  - 返す Disk の `Assigned` は常に `false`（切替は `VolumeDiskAssign` で行う）。

### 必要権限
- `Microsoft.Compute/snapshots/*`, `Microsoft.Compute/disks/*`, `Microsoft.Resources/subscriptions/resourceGroups/*`（Create/Restore 時 RG 作成の可能性）。

### タイムアウト（実装）
- List: 1 分 / Create: 3 分 / Delete: 2 分 / Restore: 3 分。
