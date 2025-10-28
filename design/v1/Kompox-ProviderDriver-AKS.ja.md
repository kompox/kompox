---
id: Kompox-ProviderDriver-AKS
title: AKS Provider Driver 実装ガイド
version: v1
status: synced
updated: 2025-10-28
language: ja
---

# AKS Provider Driver 実装ガイド v1

本書では、Kompox の AKS Provider Driver の実装仕様を解説する。Driver インターフェースの各メソッドが AKS ドライバでどのように実装されているかをセクションごとに説明する。

## AKS 設定と認証

### Provider Settings

KOM Provider リソースの `spec.settings` または `kompoxops.yml` の `provider.settings` で指定する。

- `AZURE_SUBSCRIPTION_ID`: Azure サブスクリプション ID (必須)
- `AZURE_LOCATION`: Azure リージョン (必須、例: `japaneast`)
- `AZURE_AUTH_METHOD`: 認証方式 (必須)
- `AZURE_RESOURCE_PREFIX`: リソース名プレフィクス

認証方式ごとに追加の設定が必要になる場合がある (後述)。

### Cluster Settings

KOM Cluster リソースの `spec.settings` または `kompoxops.yml` の `cluster.settings` で指定する。

- `AZURE_RESOURCE_GROUP_NAME`: クラスタ用リソースグループ名
- `AZURE_AKS_DNS_ZONE_RESOURCE_IDS`: DNS ゾーンリソース ID リスト (カンマまたはスペース区切り、`ClusterDNSApply()` で使用、`ClusterInstall()` で DNS Zone Contributor ロールを付与)
- `AZURE_AKS_CONTAINER_REGISTRY_RESOURCE_IDS`: Azure Container Registry リソース ID リスト (カンマまたはスペース区切り、`ClusterInstall()` で AcrPull ロールを付与)

また次のような AKS ノードプール設定も指定する。これらは main.bicep デプロイ時のパラメータとして渡される。

```yaml
AZURE_AKS_SYSTEM_VM_SIZE: Standard_D2ds_v4
AZURE_AKS_SYSTEM_VM_DISK_TYPE: Ephemeral
AZURE_AKS_SYSTEM_VM_DISK_SIZE_GB: 64
AZURE_AKS_SYSTEM_VM_PRIORITY: Regular
AZURE_AKS_SYSTEM_VM_ZONES:
AZURE_AKS_USER_VM_SIZE: Standard_D2ds_v4
AZURE_AKS_USER_VM_DISK_TYPE: Ephemeral
AZURE_AKS_USER_VM_DISK_SIZE_GB: 64
AZURE_AKS_USER_VM_PRIORITY: Regular
AZURE_AKS_USER_VM_ZONES: 1
```

### App Settings

KOM App リソースの `spec.settings` または `kompoxops.yml` の `app.settings` で指定する。

- `AZURE_RESOURCE_GROUP_NAME`: アプリ用リソースグループ名

### Azure 認証方式

Azure の認証情報は Provider Settings で設定する。

`AZURE_AUTH_METHOD` で認証方式を指定する。未対応の認証方式は `unsupported AZURE_AUTH_METHOD` エラーとなる。

| `AZURE_AUTH_METHOD` | 状態 | 必須追加設定 | 備考 |
|---------------------|------|---------------|------|
| `client_secret` | サポート | `AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET` | Azure AD アプリ資格情報 |
| `client_certificate` | 非サポート | - | 実装内で未対応エラー返却 |
| `managed_identity` | サポート | (`AZURE_CLIENT_ID` 任意) | UAMI 指定可能、未指定でシステム割当 |
| `workload_identity` | サポート | `AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_FEDERATED_TOKEN_FILE` | OIDC Federation |
| `azure_cli` | サポート | なし | ローカル開発向け CLI 認証 |
| `azure_developer_cli` | サポート | なし | Azure Developer CLI 認証 |

### Azure リソース命名規則

#### 短縮ハッシュ文字列

リソース名の重複を避けるため、次の種類の短縮ハッシュ文字列(6文字)を `naming.NewHashes()` で生成して使用する。
- `{prv_hash}`: workspace/provider 名から生成
- `{cls_hash}`: workspace/provider/cluster 名から生成
- `{app_hash}`: workspace/provider/app 名から生成 (App ID ハッシュ)

#### リソース名プレフィクス

リソース名には特定のプレフィクスを付加する。このドキュメントでは `{prefix}` で参照する。

命名規則:
- Provider Settings の `AZURE_RESOURCE_PREFIX` が指定されている場合はその値を使用
- 未指定の場合は `k4x-{prv_hash}` 形式で自動生成
- 実装: aks ドライバ初期化関数 (driver.go 参照)

#### クラスタ用リソースグループ

命名規則:
- Cluster Settings に `AZURE_RESOURCE_GROUP_NAME` が指定されている場合はその値を使用
- 未指定の場合は `{prefix}_cls_{cluster.name}_{cls_hash}` 形式で自動生成
- 最大長 72 文字 (Azure RG 制約 90 文字の内、ハッシュ分を考慮) でトランケートされ、ハッシュ部分は保持される
- 実装: clusterResourceGroupName()

格納されるリソースの例 (ARM テンプレート main.bicep で記述):
- AKS マネージドクラスタ
- User Assigned Managed Identity
- Key Vault
- Log Analytics Workspace
- Storage Account

#### アプリ用リソースグループ

命名規則:
- App Settings に `AZURE_RESOURCE_GROUP_NAME` が指定されている場合はその値を使用
- 未指定の場合は `{prefix}_app_{app.name}_{app_hash}` 形式で自動生成
- 最大長 72 文字 (Azure RG 制約 90 文字の内、ハッシュ分を考慮) でトランケートされ、ハッシュ部分は保持される
- 実装: appResourceGroupName()

格納されるリソースの例:
- Azure Managed Disks (Type=disk)
- Azure Managed Disk Snapshots (Type=disk)
- Azure Storage Accounts (Type=files)

## AKS Cluster

### ClusterProvision()

AKS クラスタをプロビジョニングする。

実装概要:
- Cluster Settings から `AZURE_RESOURCE_GROUP_NAME` を取得、または自動生成する
- Bicep テンプレート (`infra/aks/main.bicep`) を使用してサブスクリプションレベルデプロイメントを実行する
- デプロイメントはリソースグループ、AKS クラスタ、Workload Identity 設定、ロール割り当てなどを含む
- 既存のデプロイメントがある場合は冪等に動作する (Force オプションが指定されている場合は再デプロイ)

主な Azure リソース:
- リソースグループ (クラスタ用)
- AKS マネージドクラスタ
- Workload Identity 用 Federated Credential
- ロール割り当て (kubelet マネージド ID への権限付与)

### ClusterDeprovision()

AKS クラスタをデプロビジョニングする。

実装概要:
- サブスクリプションレベルデプロイメントを削除する (ベストエフォート)
- クラスタのリソースグループ全体を削除する
- リソースグループ内に Key Vault が存在する場合は論理削除されたものをパージする

冪等性:
- リソースグループが存在しない場合はエラーにならない

### ClusterInstall()

クラスタ内リソース (Ingress Controller など) をインストールする。

実装概要:
1. クラスタの kubeconfig を取得
2. Ingress 用 Namespace を作成 (冪等)
3. デプロイメント出力から ServiceAccount 情報を取得
4. ServiceAccount を作成し、Workload Identity のアノテーションを追加
5. DNS Zone Contributor ロールを付与 (ベストエフォート)
6. ACR AcrPull ロールを付与 (ベストエフォート)
7. Traefik Ingress Controller を Helm でインストール

DNS Zone Contributor ロール付与:
- Cluster Settings の `AZURE_AKS_DNS_ZONE_RESOURCE_IDS` に設定された全ゾーンに対して実行
- kubelet マネージド ID に対してロールを割り当てる
- クロスサブスクリプションの DNS ゾーンもサポート
- ロールが既に付与されている場合は冪等 (スキップ)
- 失敗時は Warn ログを出力して継続 (ベストエフォート)

ACR AcrPull ロール付与:
- Cluster Settings の `AZURE_AKS_CONTAINER_REGISTRY_RESOURCE_IDS` に設定された全 ACR に対して実行
- kubelet マネージド ID に対して AcrPull ロールを割り当てる
- クロスサブスクリプションの ACR もサポート
- ロールが既に付与されている場合は冪等 (スキップ)
- 失敗時は Warn ログを出力して継続 (ベストエフォート)

### ClusterUninstall()

クラスタ内リソース (Ingress Controller など) をアンインストールする。

実装概要:
1. クラスタの kubeconfig を取得
2. Traefik Ingress Controller を Helm でアンインストール
3. Ingress 用 Namespace を削除

### ClusterStatus()

クラスタの状態を取得する。

実装概要:
- kubeconfig の取得を試行してクラスタの存在 (Provisioned) を判定
- Ingress エンドポイント (LoadBalancer IP/FQDN) の取得を試行してインストール状態 (Installed) を判定

返却される ClusterStatus:
- `Existing`: クラスタ定義が既存かどうか
- `Provisioned`: クラスタがプロビジョニング済みかどうか
- `Installed`: Ingress Controller がインストール済みかどうか
- `IngressGlobalIP`: Ingress の LoadBalancer IP
- `IngressFQDN`: Ingress の FQDN

### ClusterKubeconfig()

AKS クラスタの kubeconfig を取得する。

実装概要:
- デプロイメント出力から AKS クラスタのリソース情報を取得
- Azure AKS SDK の `GetAccessProfile` API を使用して管理者資格情報を取得
- kubeconfig をバイト列として返す (ファイルには書き込まない)

### ClusterDNSApply()

DNS レコードセットをプロバイダ管理の DNS ゾーンに適用する。

実装概要:
1. Cluster Settings から `AZURE_AKS_DNS_ZONE_RESOURCE_IDS` を取得
2. FQDN に対する最長一致で DNS ゾーンを選択 (ZoneHint が指定されている場合は優先)
3. 入力検証と正規化 (TTL デフォルト値 300秒、CNAME の RData 件数チェックなど)
4. DryRun の場合はログ出力のみ
5. RData が空の場合は Delete、非空の場合は Upsert を実行

サポートするレコードタイプ:
- A レコード (IPv4 アドレス)
- AAAA レコード (IPv6 アドレス)
- CNAME レコード (別名)

レコード操作:
- Upsert: 既存レコードは上書き (冪等)
- Delete: レコードが存在しない場合もエラーにならない (冪等)
- 複数 DNS ゾーンが指定されている場合、各ゾーンに同じレコードを作成

エラーハンドリング:
- Strict オプションが指定されている場合は失敗をエラーとして返す
- 既定 (ベストエフォート) の場合は Warn ログで継続

前提条件:
- DNS Zone Contributor ロールが必要 (`ClusterInstall()` で付与)
- DNS ゾーンは事前に作成されている必要がある

詳細は [2025-10-07-aks-dns.ja.md] を参照。

## AKS Volumes

### Volume Type

Kompox は論理ボリュームに対して `Type` 属性をサポートする。AKS ドライバは以下の Volume Type に対応する。

- `disk`: Azure Managed Disk (ブロックストレージ、通常 RWO)
- `files`: Azure Files (ネットワークファイル共有、RWX)

`Type` が省略された場合は `disk` として扱う。

詳細は [K4x-ADR-014] を参照。

### リソース命名規則

ディスクリソース名:
- 形式: `{prefix}_disk_{vol.name}_{disk.name}_{hash}`
- `vol.name` は最大 16 文字、`disk.name` は最大 24 文字

スナップショットリソース名:
- 形式: `{prefix}_snap_{vol.name}_{snap.name}_{hash}`
- `vol.name` は最大 16 文字、`snap.name` は最大 24 文字

デフォルト名生成:
- ディスクやスナップショットの指定が省略された場合は `naming.NewCompactID()` を使用してデフォルト名を生成する

### タグ戦略

AKS ドライバは Azure リソースに以下のタグを付与する。

共通タグ:
- `kompox-workspace-name`: ワークスペース名
- `kompox-provider-name`: プロバイダ名
- `kompox-app-name`: アプリ名
- `kompox-app-id-hash`: アプリ ID ハッシュ (6 文字)
- `managed-by`: `"kompox"`

ボリューム固有タグ:
- `kompox-volume`: ボリューム名
- `kompox-disk-name`: ディスク名 (ディスクリソースのみ)
- `kompox-disk-assigned`: `"true"` または `"false"` (ディスクリソースのみ)
- `kompox-snapshot-name`: スナップショット名 (スナップショットリソースのみ)

これらのタグにより、同一ボリュームに属するリソースを確実に識別できる。

### オペーク Source 文字列の仕様

`VolumeDiskCreate` と `VolumeSnapshotCreate` の `source` パラメータは、作成元リソースを指定する不透明な文字列である。CLI/UseCase 層ではパース・検証を行わず、そのままドライバに渡される。


#### フォーマット

- `[<type>:]<name>`
- `<type>`: `disk` または `snapshot` (Kompox 予約語彙)
- `<name>`: Kompox 管理のディスク名またはスナップショット名、またはプロバイダネイティブなリソース ID

#### AKS ドライバでの解釈

Type チェック:
- `Type=files` のボリュームではスナップショットをサポートしないため、空文字以外の `source` が渡されるとドライバはエラーを返す。

基本動作: source 文字列のプレフィクスを見て Azure リソース ID 文字列に解決する。
- 空文字列: 新規作成 (スナップショットの場合はエラー)
- `"snapshot:<name>"`: Kompox スナップショット名による指定
- `"disk:<name>"`: Kompox ディスク名による指定
- `"/subscriptions/..."`: Azure リソース ID
- `"arm:..."`: Azure リソース ID
- `"resourceId:..."`: Azure リソース ID

フォールバック動作:
- `VolumeDiskCreate` で source が解決不能な場合は `snapshot:` プレフィクスを付与して再解決 (スナップショットからディスク)
- `VolumeSnapshotCreate` で source が解決不能な場合は `disk:` プレフィクスを付与して再解決 (ディスクからスナップショット)

詳細は [2025-09-27-disk-snapshot-unify.ja.md] および [2025-09-28-disk-snapshot-cli-flags.ja.md] を参照。

## Azure Disks

AKS Volume が `Type=disk` の場合は次の Azure リソースをアプリ用リソースグループ内で扱う。
- Disk: Azure Managed Disks
- Snapshot: Azure Managed Disk Snapshots

### 用語と命名規則

Azure Disk リソース名:
- 形式: `{prefix}_disk_{vol.name}_{disk.name}_{app_hash}`
  - `vol.name` は最大 16 文字、`disk.name` は最大 24 文字
- Azure の制約によりトランケートされる場合がある

Handle:
- Azure Disk リソース ID
- 形式: `/subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Compute/disks/{name}`

タグ:
- `kompox-volume`: ボリューム名 (論理ボリューム識別子)
- `kompox-disk-name`: ディスク名 (Kompox 管理名)
- `kompox-disk-assigned`: `"true"` または `"false"` (割り当て状態)
- 共通タグ (workspace/provider/app/managed-by) も付与

Assigned フラグ:
- 各論理ボリュームに対して、`Assigned=true` のディスクは 1 つのみ存在する
- 初回作成されたディスクは自動的に `Assigned=true` になる
- 2 個目以降のディスクは `Assigned=false` で作成される

### VolumeDiskList()

指定された論理ボリュームに属するディスクの一覧を取得する。

実装概要:
- アプリのリソースグループ内の全ディスクを取得
- `kompox-volume` タグで指定ボリュームに属するディスクをフィルタリング
- `newVolumeDisk()` を使用して `model.VolumeDisk` に変換
- `CreatedAt` 降順でソート (同時刻の場合は `Name` 昇順)

返却される `VolumeDisk`:
- `Name`: ディスク名 (`kompox-disk-name` タグから取得)
- `VolumeName`: ボリューム名
- `Assigned`: 割り当て状態 (`kompox-disk-assigned` タグから取得)
- `Size`: サイズ (バイト単位、Azure の GB 値を 30 ビット左シフト)
- `Zone`: 可用性ゾーン
- `Options`: SKU や IOPS などのオプション
- `Handle`: Azure Disk リソース ID
- `CreatedAt`: 作成日時
- `UpdatedAt`: 更新日時 (現在は `CreatedAt` と同じ)

### VolumeDiskCreate()

新規ディスクを作成する。

実装概要:
1. アプリのリソースグループを取得または作成
2. kubelet マネージド ID にリソースグループへの Contributor ロールを付与 (ベストエフォート)
3. 既存ディスク一覧を取得
4. `diskName` が省略されている場合は `naming.NewCompactID()` で生成、指定されている場合は重複チェック
5. `source` を解決して作成オプションを決定:
   - 空文字列: 空ディスク (`CreateOption=Empty`)
   - 非空: source を Azure リソース ID に解決し、コピー元として使用 (`CreateOption=Copy`)
6. 最初のディスクの場合は `Assigned=true`、それ以外は `Assigned=false`
7. Azure Disk を作成し、完了を待機
8. 作成されたディスクから `model.VolumeDisk` を生成して返す

`source` 解決:
- `resolveSourceSnapshotResourceID()` を使用してスナップショットリソース ID に解決
- 解決できない場合はエラー

SKU とパフォーマンスオプション:
- `setAzureDiskOptions()` を使用してボリューム Options から SKU、IOPS、MBps を設定
- デフォルト SKU は `Premium_LRS`

サポートする SKU:
- `Standard_LRS`
- `Premium_LRS`
- `StandardSSD_LRS`
- `UltraSSD_LRS`
- `Premium_ZRS`
- `StandardSSD_ZRS`
- `PremiumV2_LRS`

### VolumeDiskAssign()

指定されたディスクを割り当て、それ以外のディスクの割り当てを解除する。

実装概要:
1. 既存ディスク一覧を取得
2. 指定されたディスク名が存在することを確認
3. 全ディスクに対して `kompox-disk-assigned` タグを更新:
   - 指定されたディスク: `"true"`
   - それ以外: `"false"`
4. タグの更新のみを行い、ディスクの内容には変更を加えない

冪等性:
- 既に正しい割り当て状態の場合は更新をスキップ

### VolumeDiskDelete()

指定されたディスクを削除する。

実装概要:
1. ディスクの Azure リソース名を生成
2. Azure Disk を削除し、完了を待機

冪等性:
- ディスクが存在しない場合 (`NotFound`) はエラーにならない

### VolumeClass()

`Type=disk` の論理ボリュームに対して Kubernetes で使用するストレージクラス情報を返す。

返却される VolumeClass:
- `StorageClassName`: `"managed-csi"` (AKS の既定 Managed Disk クラス)
- `CSIDriver`: `"disk.csi.azure.com"` (Azure Disk CSI Driver)
- `FSType`: `"ext4"` (既定ファイルシステム)
- `Attributes`: `{"fsType": "ext4"}` (CSI ドライバへのパラメータ)
- `AccessModes`: `["ReadWriteOnce"]` (RWO アクセスモード)
- `ReclaimPolicy`: `"Retain"` (削除時に PV を保持)
- `VolumeMode`: `"Filesystem"` (ファイルシステムモード)

注意事項:
- 空フィールドは「ノーオピニオン」を表し、呼び出し側はマニフェストに含めない
- 将来的には `Options` から SKU やパフォーマンスティアを反映する可能性がある

### VolumeSnapshotList()

指定された論理ボリュームに属するスナップショットの一覧を取得する。

実装概要:
- アプリのリソースグループ内の全スナップショットを取得
- `kompox-volume` タグで指定ボリュームに属するスナップショットをフィルタリング
- `newVolumeSnapshot()` を使用して `model.VolumeSnapshot` に変換
- `CreatedAt` 降順でソート (同時刻の場合は `Name` 昇順)

返却される `VolumeSnapshot`:
- `Name`: スナップショット名 (`kompox-snapshot-name` タグから取得)
- `VolumeName`: ボリューム名
- `Size`: サイズ (バイト単位)
- `Handle`: Azure Snapshot リソース ID
- `CreatedAt`: 作成日時
- `UpdatedAt`: 更新日時 (現在は `CreatedAt` と同じ)

### VolumeSnapshotCreate()

新規スナップショットを作成する。

実装概要:
1. `snapName` が省略されている場合は `naming.NewCompactID()` で生成
2. `source` を解決:
   - 空文字列: 現在 Assigned されているディスクを自動選択 (単一でない場合はエラー)
   - 非空: source を Azure リソース ID に解決
3. source リソース ID を解析して種別 (Disk or Snapshot) を判定:
   - Disk の場合: `CreateOption=Copy`
   - Snapshot の場合: `CreateOption=CopyStart`
4. Azure Snapshot を作成し、完了を待機 (増分スナップショット、SKU は `Standard_ZRS`)
5. 作成されたスナップショットから `model.VolumeSnapshot` を生成して返す

`source` 解決:
- `resolveSourceDiskResourceID()` を使用してディスクリソース ID に解決
- 空文字列の場合は Assigned ディスクを自動選択

### VolumeSnapshotDelete()

指定されたスナップショットを削除する。

実装概要:
1. スナップショットの Azure リソース名を生成
2. Azure Snapshot を削除し、完了を待機

冪等性:
- スナップショットが存在しない場合 (`NotFound`) はエラーにならない

## Azure Files

AKS Volume が `Type=files` の場合は次の Azure リソースをアプリ用リソースグループ内で扱う。
- Disk: Azure Storage Account のファイル共有 (Azure Files)
- Snapshot: サポートしない

### 用語と命名規則

ストレージアカウント:
- アプリ単位で 1 つのストレージアカウントを使用する
- Disk (`Type=files`) 初回作成時に自動作成される
- 形式: `k4x{prv_hash}{app_hash}` (小文字英数字のみ、15文字)
- そのアプリの全 Azure Files 共有はこのアカウント内に作成される

Azure Files 共有名:
- 形式: `{vol.name}-{disk.name}`
- `vol.name` は最大 16 文字、`disk.name` は最大 24 文字
- 合計 41 文字 (ハイフン含む) で Azure Files の共有名上限 (3..63 文字) 内に収まる
- 文字集合は DNS-1123 制約に整合 (小文字英数字とハイフン)

Handle:
- SMB: `smbs://{account}.file.core.windows.net/{share}`

メタデータ:
- Azure Files の共有メタデータで論理ボリュームの属性を保持
- `kompox-volume`: `{vol.name}-{idHash}` (論理ボリューム識別子)
- `kompox-disk-assigned`: `"true"` または `"false"` (割り当て状態)

プロトコル:
- 当面は SMB のみをサポート
- NFS は将来的な拡張として検討

### VolumeClass()

`Type=files` の論理ボリュームに対して Kubernetes で使用するストレージクラス情報を返す。

返却される VolumeClass:
- `StorageClassName`: `"azurefile-csi"` (AKS の Azure Files クラス)
- `CSIDriver`: `"file.csi.azure.com"` (Azure Files CSI Driver)
- `AccessModes`: `["ReadWriteMany"]` (RWX アクセスモード)
- `VolumeMode`: `"Filesystem"` (ファイルシステムモード)
- `Attributes`: SKU などのオプション
  - `protocol`: `"smb"` (固定)
  - `skuName`: ストレージアカウントの SKU (例: `"Standard_LRS"`, `"Premium_ZRS"`)
  - `availability`: 可用性レベル (例: `"zrs"`, `"lrs"`)

注意事項:
- 当面は SMB プロトコルのみサポート
- 具体的な StorageClass 名やパラメータはクラスタにより異なる
- ドライバ既定を採用しつつ、`Options` で上書き可能
- 空フィールドは「ノーオピニオン」を表し、呼び出し側はマニフェストに含めない

### VolumeDiskList()

指定された論理ボリュームに属する Azure Files 共有の一覧を取得する。

実装概要:
- アプリのリソースグループ内のストレージアカウントから全共有を取得
- 共有メタデータの `kompox-volume` で指定ボリュームに属する共有をフィルタリング
- `CreatedAt` 降順でソート (同時刻の場合は `Name` 昇順)

返却される `VolumeDisk`:
- `Name`: 共有名 (`{vol.name}-{disk.name}`)
- `VolumeName`: ボリューム名
- `Assigned`: 割り当て状態 (共有メタデータから取得)
- `Size`: クォータサイズ (バイト単位、0 は未設定)
- `Zone`: 空文字列 (Azure Files はリージョナルサービス)
- `Options`: プロトコルや SKU などのオプション
- `Handle`: Azure Files の URI (`smbs://...` または `nfs://...`)
- `CreatedAt`: 作成日時
- `UpdatedAt`: 更新日時

エラーハンドリング:
- リソースグループまたはストレージアカウントが存在しない場合は空配列を返す
- Azure API エラーは上位へ伝播

### VolumeDiskCreate()

新規 Azure Files 共有を作成する。

実装概要:
1. アプリのリソースグループを取得または作成
2. ストレージアカウントが存在しなければ作成 (アプリ単位で 1 つ)
3. 既存共有一覧を取得
4. `diskName` が省略されている場合は `naming.NewCompactID()` で生成、指定されている場合は重複チェック
5. 共有 `{vol.name}-{disk.name}` を作成、クォータに `sizeGiB` を設定
6. 最初の共有の場合は `kompox-disk-assigned=true`、それ以外は `kompox-disk-assigned=false`
7. 作成された共有から `model.VolumeDisk` を生成して返す

`source` パラメータ:
- Azure Files ではスナップショットやコピー元からの復元をサポートしない
- `source` が指定された場合はエラーを返す

プロトコル:
- SMB プロトコルのみサポート (固定)
- `Options.protocol` が `"smb"` 以外の場合はエラーを返す

ストレージアカウント SKU:
- `Options.skuName` で指定、未指定の場合は `Standard_LRS` (既定)
- サポートする SKU: `Standard_LRS`, `Standard_ZRS`, `Premium_LRS`, `Premium_ZRS`

### VolumeDiskAssign()

指定された Azure Files 共有を割り当て、それ以外の共有の割り当てを解除する。

実装概要:
1. 既存共有一覧を取得
2. 指定された共有名 (`{vol.name}-{disk.name}`) が存在することを確認
3. 全共有に対して `kompox-disk-assigned` メタデータを更新:
   - 指定された共有以外: `"false"` に更新
   - 指定された共有: `"true"` に更新

冪等性:
- 既に正しい割り当て状態の場合は更新をスキップ

エラーハンドリング:
- リソースグループまたはストレージアカウントが存在しない場合は `NotFound` エラー
- 指定された共有が存在しない場合は `NotFound` エラー

注意事項:
- メタデータ更新は順次実行 (Azure Storage にトランザクション機能なし)
- 部分的な失敗時はエラーを返すが、一部の更新は完了している可能性がある

### VolumeDiskDelete()

指定された Azure Files 共有を削除する。

実装概要:
1. 共有 `{vol.name}-{disk.name}` を削除

冪等性:
- 共有が存在しない場合 (`NotFound`) はエラーにならない
- ストレージアカウントが存在しない場合もエラーにならない

注意事項:
- `Assigned=true` の共有でも削除可能 (運用上は切替後の削除を推奨)
- 最後の共有を削除してもストレージアカウントは削除されない

### スナップショット非対応

Azure Files のスナップショット操作は現在サポートしない。

`VolumeSnapshot*` メソッド:
- `VolumeSnapshotList`: `ErrNotSupported` を返す
- `VolumeSnapshotCreate`: `ErrNotSupported` を返す
- `VolumeSnapshotDelete`: `ErrNotSupported` を返す

将来の拡張:
- Azure Files NFS プロトコルのサポート
- Azure Files のネイティブスナップショット機能の統合を検討
- Azure NetApp Files (ANF) への対応 (`backend=anf` オプション)
- Azure Managed Lustre への対応 (`backend=lustre` オプション)
- Azure Blob via FUSE (azureblob) への対応 (`backend=azureblob` オプション)

## 参考文献

- [Kompox-ProviderDriver.ja.md]: Provider Driver の公開契約と実装ガイドライン
- [Kompox-KOM.ja.md]: Kompox Ops Manifest 仕様
- [K4x-ADR-003]: Disk/Snapshot CLI フラグ統一と Source パラメータの不透明化
- [K4x-ADR-004]: Cluster ingress endpoint DNS 自動更新
- [K4x-ADR-014]: Volume Type の導入 (disk/files)
- [2025-09-27-disk-snapshot-unify.ja.md]: Disk/Snapshot の Source パススルー実装タスク
- [2025-09-28-disk-snapshot-cli-flags.ja.md]: Disk/Snapshot CLI フラグ統一タスク
- [2025-10-07-aks-dns.ja.md]: AKS Driver ClusterDNSApply 実装タスク
- [2025-10-23-aks-cr.ja.md]: AKS Driver ACR 権限付与対応タスク

[Kompox-ProviderDriver.ja.md]: ./Kompox-ProviderDriver.ja.md
[Kompox-KOM.ja.md]: ./Kompox-KOM.ja.md
[K4x-ADR-003]: ../adr/K4x-ADR-003.md
[K4x-ADR-004]: ../adr/K4x-ADR-004.md
[K4x-ADR-014]: ../adr/K4x-ADR-014.md
[2025-09-27-disk-snapshot-unify.ja.md]: ../../_dev/tasks/2025-09-27-disk-snapshot-unify.ja.md
[2025-09-28-disk-snapshot-cli-flags.ja.md]: ../../_dev/tasks/2025-09-28-disk-snapshot-cli-flags.ja.md
[2025-10-07-aks-dns.ja.md]: ../../_dev/tasks/2025-10-07-aks-dns.ja.md
[2025-10-23-aks-cr.ja.md]: ../../_dev/tasks/2025-10-23-aks-cr.ja.md
