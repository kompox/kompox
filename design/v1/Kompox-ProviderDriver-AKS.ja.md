---
id: Kompox-ProviderDriver-AKS
title: AKS Provider Driver 実装ガイド
version: v1
status: synced
updated: 2026-02-17T23:42:52Z
language: ja
---

# AKS Provider Driver 実装ガイド v1

本書は Kompox の AKS Provider Driver の実装仕様を解説する。現実装 (`adapters/drivers/provider/aks/`) を一次情報源とし、Driver インターフェースの各メソッドが AKS でどのように具現化されているかをセクションごとに説明する。

親契約については [Kompox-ProviderDriver] を参照。

---

## 1. 初期化と認証

### 1.1 ドライバ構造体

AKS ドライバは `driver` 構造体で状態を保持する (driver.go)。

| フィールド | 型 | 用途 |
|---|---|---|
| `workspaceName` | `string` | ワークスペース名 (nil 時は `"(nil)"`) |
| `providerName` | `string` | プロバイダ名 |
| `resourcePrefix` | `string` | Azure リソース名プレフィクス |
| `TokenCredential` | `azcore.TokenCredential` | 全 Azure SDK クライアントで共有する認証情報 |
| `AzureSubscriptionId` | `string` | サブスクリプション ID (小文字正規化) |
| `AzureLocation` | `string` | リージョン (小文字正規化) |
| `volumeBackends` | `map[string]volumeBackend` | Volume Type → バックエンド実装のディスパッチマップ |

ドライバは `init()` 内で `providerdrv.Register("aks", factory)` により自己登録される。

### 1.2 Provider Settings

KOM Provider リソースの `spec.settings` または `kompoxops.yml` の `provider.settings` で指定する。

| キー | 必須 | 説明 |
|---|---|---|
| `AZURE_SUBSCRIPTION_ID` | ○ | Azure サブスクリプション ID |
| `AZURE_LOCATION` | ○ | Azure リージョン (例: `japaneast`) |
| `AZURE_AUTH_METHOD` | ○ | 認証方式 (後述) |
| `AZURE_RESOURCE_PREFIX` | — | リソース名プレフィクス (省略時は自動生成) |

認証方式ごとに追加設定が必要になる (後述)。

### 1.3 Cluster Settings

KOM Cluster リソースの `spec.settings` または `kompoxops.yml` の `cluster.settings` で指定する。

| キー | 説明 |
|---|---|
| `AZURE_RESOURCE_GROUP_NAME` | クラスタ用リソースグループ名 (省略時は自動生成) |
| `AZURE_AKS_DNS_ZONE_RESOURCE_IDS` | DNS ゾーンリソース ID リスト (カンマまたはスペース区切り) |
| `AZURE_AKS_CONTAINER_REGISTRY_RESOURCE_IDS` | ACR リソース ID リスト (カンマまたはスペース区切り) |

ノードプール設定 (ARM デプロイパラメータ):

| キー | 説明 |
|---|---|
| `AZURE_AKS_SYSTEM_VM_SIZE` | System プール VM サイズ |
| `AZURE_AKS_SYSTEM_VM_DISK_TYPE` | System プール OS ディスクタイプ |
| `AZURE_AKS_SYSTEM_VM_DISK_SIZE_GB` | System プール OS ディスクサイズ (GB) |
| `AZURE_AKS_SYSTEM_VM_PRIORITY` | System プール VM 優先度 |
| `AZURE_AKS_SYSTEM_VM_ZONES` | System プール 可用性ゾーン |
| `AZURE_AKS_USER_VM_SIZE` | User プール VM サイズ |
| `AZURE_AKS_USER_VM_DISK_TYPE` | User プール OS ディスクタイプ |
| `AZURE_AKS_USER_VM_DISK_SIZE_GB` | User プール OS ディスクサイズ (GB) |
| `AZURE_AKS_USER_VM_PRIORITY` | User プール VM 優先度 |
| `AZURE_AKS_USER_VM_ZONES` | User プール 可用性ゾーン |

これらは `paramSettingMap` (azure_aks.go) を経由して ARM テンプレートパラメータにマッピングされる。

### 1.4 App Settings

KOM App リソースの `spec.settings` または `kompoxops.yml` の `app.settings` で指定する。

| キー | 説明 |
|---|---|
| `AZURE_RESOURCE_GROUP_NAME` | アプリ用リソースグループ名 (省略時は自動生成) |

### 1.5 Azure 認証方式

ファクトリ関数 (driver.go) 内で `AZURE_AUTH_METHOD` に基づき `azcore.TokenCredential` を生成する。

| `AZURE_AUTH_METHOD` | 状態 | 追加設定 | 備考 |
|---|---|---|---|
| `client_secret` | サポート | `AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET` | SP 資格情報 |
| `client_certificate` | 非サポート | — | 実装内で未対応エラー返却 |
| `managed_identity` | サポート | (`AZURE_CLIENT_ID` 任意) | UAMI 指定可能、未指定でシステム割当 |
| `workload_identity` | サポート | `AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_FEDERATED_TOKEN_FILE` | OIDC Federation |
| `azure_cli` | サポート | なし | ローカル開発向け |
| `azure_developer_cli` | サポート | なし | azd 認証 |

未対応の値は `unsupported AZURE_AUTH_METHOD` エラーとなる。

---

## 2. Azure リソース命名規則

実装は naming.go に集約される。

### 2.1 短縮ハッシュ

`naming.NewHashes()` により以下の 6 文字ハッシュを生成する。

| ハッシュ | 入力 | 用途 |
|---|---|---|
| `{prv_hash}` | workspace + provider | プロバイダスコープのリソース識別 |
| `{cls_hash}` | workspace + provider + cluster | クラスタスコープのリソース識別 |
| `{app_hash}` | workspace + provider + app | アプリスコープのリソース識別 |

### 2.2 リソースプレフィクス

- `AZURE_RESOURCE_PREFIX` が指定されていればその値 (最大 32 文字でトランケート)
- 未指定の場合は `k4x-{prv_hash}` 形式で自動生成

### 2.3 クラスタ用リソースグループ

- `AZURE_RESOURCE_GROUP_NAME` (Cluster Settings) が指定されていればその値
- 未指定: `{prefix}_cls_{cluster.name}_{cls_hash}`
- 最大 72 文字で `safeTruncate()` によりトランケート (ハッシュサフィクスは保持)
- 実装: `clusterResourceGroupName()`

### 2.4 アプリ用リソースグループ

- `AZURE_RESOURCE_GROUP_NAME` (App Settings) が指定されていればその値
- 未指定: `{prefix}_app_{app.name}_{app_hash}`
- 最大 72 文字で `safeTruncate()` によりトランケート
- 実装: `appResourceGroupName()`

### 2.5 ディスクリソース名

- 形式: `{prefix}_disk_{vol.name}_{disk.name}_{app_hash}`
- `vol.name` 最大 16 文字、`disk.name` 最大 24 文字
- 実装: `appDiskName()`

### 2.6 スナップショットリソース名

- 形式: `{prefix}_snap_{vol.name}_{snap.name}_{app_hash}`
- `vol.name` 最大 16 文字、`snap.name` 最大 24 文字
- 実装: `appSnapshotName()`

### 2.7 ストレージアカウント名

- 形式: `k4x{prv_hash}{app_hash}` (15 文字、小文字英数字のみ)
- Azure Storage Account の名前制約 (3–24 文字、小文字英数字のみ) に適合
- アプリ単位で 1 つのストレージアカウントを使用
- 実装: `appStorageAccountName()`

### 2.8 デフォルト名生成

ディスク名やスナップショット名が省略された場合は `naming.NewCompactID()` でデフォルト名を生成する。

---

## 3. Azure Identity と権限モデル

AKS クラスタは 2 種類の Managed Identity を使用し、それぞれ異なる目的で権限が付与される。

### 3.1 Cluster Identity (system-assigned managed identity)

- **用途**: Azure リソース操作 (DNS Zone 更新、ACR アクセス、Key Vault アクセス)
- **ロール割り当て**:
  - DNS Zone Contributor: `ClusterInstall()` で付与
  - AcrPull: `ClusterInstall()` で付与
  - Key Vault Secrets User: `ensureAzureRoleKeyVaultSecret()` で付与
- **Bicep 出力キー**: `AZURE_AKS_CLUSTER_PRINCIPAL_ID`
- **取得元**: `aks.identity.principalId`

### 3.2 Kubelet Identity (managed identity for node pools)

- **用途**: Azure Disk/Files CSI ドライバー操作 (ストレージアカウントキー取得、ディスクアタッチ)
- **ロール割り当て**:
  - Contributor: アプリ用リソースグループに対して
  - 割り当てタイミング: Disk 初回作成時 (`ensureAzureResourceGroupCreated`) またはストレージアカウント作成時 (`ensureStorageAccountCreated`)
- **Bicep 出力キー**: `AZURE_AKS_KUBELET_PRINCIPAL_ID`
- **取得元**: `aks.properties.identityProfile.kubeletidentity.objectId`
- **重要**: Azure Files CSI ドライバーがストレージアカウントキーを取得するには、Kubelet Identity に Contributor ロールが必須

### 3.3 ロール割り当ての実装

- `ensureAzureRole()` (azure_roles.go) がべき等なロール割り当てを実行
- `RoleAssignmentExists` / HTTP 409 は成功扱い
- ロール定義 ID は定数で管理: `roleDefIDContributor`, `roleDefIDDNSZoneContributor`, `roleDefIDKeyVaultSecretsUser`, `roleDefIDAcrPull`
- PrincipalType は `ServicePrincipal` 固定

---

## 4. タグ戦略

### 4.1 タグ名定数

タグ名は naming.go で定数化されている。

| 定数名 | タグ名 | 用途 |
|---|---|---|
| `tagWorkspaceName` | `kompox-workspace-name` | ワークスペース識別 |
| `tagProviderName` | `kompox-provider-name` | プロバイダ識別 |
| `tagClusterName` | `kompox-cluster-name` | クラスタ識別 |
| `tagClusterHash` | `kompox-cluster-hash` | クラスタハッシュ |
| `tagAppName` | `kompox-app-name` | アプリ識別 |
| `tagAppIDHash` | `kompox-app-id-hash` | アプリ ID ハッシュ |
| `tagVolumeName` | `kompox-volume` | ボリューム識別 |
| `tagDiskName` | `kompox-disk-name` | ディスク名 |
| `tagDiskAssigned` | `kompox-disk-assigned` | 割り当て状態 (`"true"` / `"false"`) |
| `tagSnapshotName` | `kompox-snapshot-name` | スナップショット名 |
| — | `managed-by` | `"kompox"` 固定 |

### 4.2 クラスタスコープタグ

`clusterResourceTags()` が生成:
- `kompox-workspace-name`, `kompox-provider-name`, `kompox-cluster-name`, `kompox-cluster-hash`, `managed-by`

### 4.3 アプリスコープタグ

`appResourceTags()` が生成:
- `kompox-workspace-name`, `kompox-provider-name`, `kompox-app-name`, `kompox-app-id-hash`, `managed-by`

### 4.4 ボリュームスコープタグ

Azure Managed Disk:
- 共通タグ + `kompox-volume`, `kompox-disk-name`, `kompox-disk-assigned`

Azure Files 共有メタデータ:
- `kompox_volume_name`, `kompox_files_share_name`, `kompox_files_share_assigned`

---

## 5. ロギング

### 5.1 Span パターン

AKS ドライバは `withMethodLogger()` (logging.go) でメソッド開始/終了を記録する。

```go
ctx, cleanup := d.withMethodLogger(ctx, "ClusterProvision")
defer func() { cleanup(err) }()
```

ログメッセージ形式:
- 開始: `AKS:<method>/S`
- 成功: `AKS:<method>/EOK`
- 失敗: `AKS:<method>/EFAIL`

すべて INFO レベル。ロガー属性に `driver=AKS.<method>` を付与。エラーメッセージは 32 文字でトランケートされる。

---

## 6. AKS Cluster ライフサイクル

### 6.1 ClusterProvision()

AKS クラスタをプロビジョニングする。

- **タイムアウト**: 30 分
- **ロギング**: Span パターン適用
- **処理**:
  1. `clusterResourceGroupName()` でリソースグループ名を導出
  2. `clusterResourceTags()` でタグを生成
  3. Force オプションを解決
  4. `ensureAzureDeploymentCreated()` でサブスクリプションスコープデプロイメントを実行
- **ARM テンプレート**: `main.json` (embed.go 経由で Go バイナリに埋め込み)
- **ARM パラメータ**: `environmentName`, `location`, `resourceGroupName`, `ingressServiceAccountName`, `ingressServiceAccountNamespace`, `tags` + Cluster Settings から `paramSettingMap` 経由でマッピング
- **主な Azure リソース (ARM テンプレートで定義)**:
  - リソースグループ
  - AKS マネージドクラスタ
  - User Assigned Managed Identity
  - Key Vault
  - Log Analytics Workspace
  - Storage Account
  - Workload Identity 用 Federated Credential
- **冪等性**: 既存の成功済みデプロイメントがあり Force が false の場合はスキップ

### 6.1a ARM デプロイメント出力 (Deployment Outputs)

AKS ドライバは ARM テンプレートで作成されたサブスクリプションスコープのデプロイメントリソースを **プロビジョニング後のすべてのクラスタ管理操作における単一の情報源 (single source of truth)** として使用する。

#### デプロイメントリソースの特定

- **デプロイメント名**: `azureDeploymentName()` が返す値はリソースグループ名 (`clusterResourceGroupName()`) と同一であり、決定的に導出される
- **スコープ**: サブスクリプションスコープ (`DeploymentsClient.GetAtSubscriptionScope`)
- **取得関数**: `azureDeploymentOutputs()` (azure_aks.go) がデプロイメントリソースの `Properties.Outputs` を読み取り、キーを大文字に正規化して返す

#### 出力変数一覧

Bicep テンプレート (`infra/aks/infra/main.bicep`) の末尾で定義される出力変数:

| 出力キー | 値の由来 | 主な用途 |
|---|---|---|
| `AZURE_LOCATION` | `location` パラメータ | (参考情報) |
| `AZURE_TENANT_ID` | `tenant().tenantId` | Workload Identity アノテーション |
| `AZURE_SUBSCRIPTION_ID` | `subscription().subscriptionId` | (参考情報) |
| `AZURE_RESOURCE_GROUP_NAME` | `rg.name` | kubeconfig 取得時の RG 指定 |
| `AZURE_AKS_CLUSTER_NAME` | `aks.outputs.clusterName` | kubeconfig 取得時のクラスタ名指定 |
| `AZURE_AKS_CLUSTER_PRINCIPAL_ID` | `aks.outputs.clusterPrincipalId` | DNS Zone Contributor / AcrPull / Key Vault ロール割り当て |
| `AZURE_AKS_KUBELET_PRINCIPAL_ID` | `aks.outputs.kubeletPrincipalId` | Storage Account / Managed Disk の Contributor ロール割り当て |
| `AZURE_AKS_OIDC_ISSUER_URL` | `aks.outputs.oidcIssuerUrl` | (Federated Credential で使用、Bicep 内) |
| `AZURE_INGRESS_SERVICE_ACCOUNT_NAMESPACE` | `ingressServiceAccountNamespace` パラメータ | Ingress SA の Namespace 検証 |
| `AZURE_INGRESS_SERVICE_ACCOUNT_NAME` | `ingressServiceAccountName` パラメータ | Ingress SA 名の検証 |
| `AZURE_INGRESS_SERVICE_ACCOUNT_PRINCIPAL_ID` | `userIdentity.outputs.principalId` | Key Vault Secrets User ロール割り当て |
| `AZURE_INGRESS_SERVICE_ACCOUNT_CLIENT_ID` | `userIdentity.outputs.clientId` | Workload Identity アノテーション |

Go コード側の定数は `azure_aks.go` の `output*` 定数群で定義される。

#### 出力変数の依存箇所

`azureDeploymentOutputs()` は以下のメソッドから呼び出される:

| 呼び出し元 | 使用する出力変数 | 目的 |
|---|---|---|
| `azureKubeconfig()` | `AZURE_RESOURCE_GROUP_NAME`, `AZURE_AKS_CLUSTER_NAME` | AKS Admin Credentials API のパラメータ |
| `ClusterInstall()` | `AZURE_TENANT_ID`, `AZURE_INGRESS_SERVICE_ACCOUNT_*` | ServiceAccount の Workload Identity 設定 |
| `ClusterInstall()` | `AZURE_AKS_CLUSTER_PRINCIPAL_ID`, `AZURE_AKS_KUBELET_PRINCIPAL_ID` | DNS / ACR ロール割り当て |
| `ensureStorageAccountCreated()` | `AZURE_AKS_KUBELET_PRINCIPAL_ID` | Storage Account の Contributor ロール割り当て |
| `volumeBackendDisk` (2 箇所) | `AZURE_AKS_CLUSTER_PRINCIPAL_ID` | Disk RG への Contributor ロール割り当て |
| `secretProviderClassForCluster()` | `AZURE_INGRESS_SERVICE_ACCOUNT_PRINCIPAL_ID` | Key Vault Secrets User ロール割り当て |

#### 設計上の制約と影響

`azureDeploymentOutputs()` がデプロイメントリソースを取得できない場合、AKS ドライバはクラスタに対する **すべての管理操作が不可能**になる。具体的には:

- **kubeconfig 取得不可** → kubectl 操作、アプリデプロイ、Ingress 設定がすべて失敗
- **Principal ID 不明** → RBAC ロール割り当て不可 → Volume 操作、DNS 更新、ACR プル、Key Vault アクセスが失敗
- **Workload Identity 情報不明** → Ingress ServiceAccount 設定不可

この設計の安全性は以下の特性に依存する:

1. **決定的な命名**: デプロイメント名はクラスタ定義から一意に導出されるため、名前の不一致による参照喪失は発生しない
2. **サブスクリプションスコープ**: デプロイメントはリソースグループ内ではなくサブスクリプションスコープに存在するため、リソースグループを誤って削除してもデプロイメントレコード自体は残る
3. **Azure のデプロイメント履歴制限**: サブスクリプションスコープのデプロイメント数上限は 800 件。上限に達すると Azure が古いデプロイメントを自動削除する可能性があるが、kompox が管理するデプロイメント数は通常少数であり、実運用上のリスクは低い
4. **手動削除リスク**: Azure Portal や CLI からデプロイメントレコードを手動で削除すると復旧できない。ただし、同じパラメータで `ClusterProvision()` を再実行すれば ARM テンプレートが再デプロイされ、出力変数も復元される

#### 今後の改善方針: リソースグループタグへの移行

現在の deployment outputs 方式はデプロイメントレコードのライフサイクルに依存しており、以下のリスクがある:

- Azure のデプロイメント履歴上限 (800 件) による自動削除
- 運用者による意図しないデプロイメントレコードの手動削除
- outputs の更新が ARM テンプレートの再デプロイを必要とする（部分更新不可）

**リソースグループタグ方式**への移行により、これらのリスクを解消できる。

| 特性 | Deployment Outputs (現行) | RG タグ (移行先) |
|---|---|---|
| ライフサイクル | デプロイメントレコードに依存 | RG が存在する限り不滅 |
| 手動削除リスク | レコードは「掃除」で消されやすい | タグは通常消されない |
| 更新タイミング | ARM テンプレート再デプロイ時のみ | `TagsClient` で即時更新可能 |
| 記録可能な値 | ARM テンプレートの output で定義した変数のみ | 任意の key-value (値 256 文字まで) |
| 取得コスト | `GetAtSubscriptionScope` 1 回 | `ResourceGroupsClient.Get` 1 回 |

RG はリソースグループ名が決定的に導出できるため参照可能であり、kompox が管理する一級リソースとして RG 自体がライフサイクルを持つ。UUID (36 文字) は 256 文字制限に余裕をもって収まる。

**記録するタグ** (4 個):

| タグキー | 値 | 長さ |
|---|---|---|
| `kompox/aks-cluster-principal-id` | Cluster Identity の UUID | 36 文字 |
| `kompox/aks-kubelet-principal-id` | Kubelet Identity の UUID | 36 文字 |
| `kompox/ingress-sa-principal-id` | Ingress SA の Principal ID | 36 文字 |
| `kompox/ingress-sa-client-id` | Ingress SA の Client ID | 36 文字 |

既に決定的に導出可能な値 (`AZURE_RESOURCE_GROUP_NAME`, `AZURE_TENANT_ID`, `AZURE_INGRESS_SERVICE_ACCOUNT_NAME/NAMESPACE`) はタグに記録する必要がない。

**段階的移行パス**:

1. **Phase 1 (互換)**: `ClusterProvision()` 完了後に deployment outputs の値を RG タグに書き込む。読み取りは引き続き deployment outputs から行う
2. **Phase 2 (フォールバック)**: `azureDeploymentOutputs()` 失敗時に RG タグからフォールバック読み取りを行う
3. **Phase 3 (完了)**: RG タグを primary な情報源とし、deployment outputs への参照を削除

### 6.2 ClusterDeprovision()

AKS クラスタをデプロビジョニングする。

- **タイムアウト**: 30 分
- **ロギング**: Span パターン適用
- **処理**:
  1. `clusterResourceGroupName()` でリソースグループ名を導出
  2. `ensureAzureDeploymentDeleted()` でサブスクリプションスコープデプロイメントを削除 (ベストエフォート)
  3. `ensureAzureResourceGroupDeleted()` でリソースグループ全体を削除
- **Key Vault パージ**: リソースグループ内に Key Vault が存在する場合、RG 削除後に論理削除された Key Vault をパージ (azure_kv.go)
- **冪等性**: リソースグループが存在しない場合はエラーにならない

### 6.3 ClusterStatus()

クラスタの状態を取得する。

- **タイムアウト**: 5 分
- **処理**:
  1. `kubeClient()` で kube クライアント生成を試行 → 成功すれば `Provisioned=true`
  2. `kc.IngressEndpoint()` で Ingress エンドポイント取得を試行 → 成功すれば `Installed=true`
- **返却**: `model.ClusterStatus`
  - `Existing`: cluster 定義の既存フラグ
  - `Provisioned`: kubeconfig 取得可能な状態
  - `Installed`: Ingress エンドポイントが存在する状態
  - `IngressGlobalIP`: LoadBalancer IP
  - `IngressFQDN`: LoadBalancer FQDN

### 6.4 ClusterInstall()

クラスタ内リソースをインストールする。

- **タイムアウト**: 10 分
- **ロギング**: Span パターン適用
- **処理ステップ**:
  1. `kubeClient()` で kube クライアントを生成
  2. Ingress 用 Namespace を作成 (`kube.IngressNamespace`)
  3. ARM デプロイメント出力から ServiceAccount 情報を取得
     - `AZURE_INGRESS_SERVICE_ACCOUNT_NAMESPACE`, `AZURE_INGRESS_SERVICE_ACCOUNT_NAME`
     - `AZURE_TENANT_ID`, `AZURE_INGRESS_SERVICE_ACCOUNT_CLIENT_ID`
  4. ServiceAccount を作成し、Workload Identity アノテーションを付与
     - `azure.workload.identity/tenant-id`, `azure.workload.identity/client-id`
  5. (オプション) TLS 証明書が設定されている場合、Key Vault 連携で SecretProviderClass を生成 (後述)
  6. DNS Zone Contributor ロールを付与 (Cluster Identity、ベストエフォート)
  7. AcrPull ロールを付与 (Kubelet Identity、ベストエフォート)
  8. Traefik Ingress Controller を Helm でインストール
     - Pod ラベル: `azure.workload.identity/use: "true"`
     - Service: `externalTrafficPolicy: Local` (クライアント IP 保持)
     - (オプション) CSI ボリュームマウントと `certs.yaml` 注入

### 6.5 ClusterUninstall()

クラスタ内リソースをアンインストールする。

- **タイムアウト**: 10 分
- **ロギング**: Span パターン適用
- **処理**:
  1. `kubeClient()` で kube クライアントを生成
  2. Traefik を Helm でアンインストール
  3. Ingress 用 Namespace を削除

### 6.6 ClusterKubeconfig()

AKS の管理者 kubeconfig をバイト列として返す。

- **タイムアウト**: 2 分
- **処理**: `azureKubeconfig()` を呼び出し (azure_aks.go)
  - ARM デプロイメント出力から AKS クラスタリソース情報を取得
  - `armcontainerservice` SDK の `GetAccessProfile` API で管理者資格情報を取得
  - kubeconfig をバイト列として返却 (ファイル出力しない)

---

## 7. DNS 管理

### 7.1 設定

DNS ゾーンリソース ID は Cluster Settings の `AZURE_AKS_DNS_ZONE_RESOURCE_IDS` にカンマまたはスペース区切りで指定する。

各 ID は `arm.ParseResourceID()` で検証され、`Microsoft.Network/dnszones` リソースタイプであることが確認される。

### 7.2 ClusterDNSApply()

DNS レコードセットをプロバイダ管理の Azure DNS ゾーンに適用する。

- **処理**:
  1. `collectDNSZoneIDs()` で DNS ゾーン情報を収集
  2. `normalizeDNSRecordSet()` で入力を正規化・検証
     - FQDN 末尾ドット除去
     - TTL デフォルト値 300 秒
     - CNAME は RData 1 件のみ許可
  3. `selectDNSZone()` でゾーンを選択
     - ZoneHint 指定時: ID またはゾーン名で一致
     - 未指定時: FQDN に対する最長一致ヒューリスティック
  4. DryRun の場合はログ出力のみ
  5. RData が空の場合は `deleteAzureDNSRecord()` で削除
  6. RData が非空の場合は `upsertAzureDNSRecord()` で作成/更新

- **サポートするレコードタイプ**: A, AAAA, CNAME
- **冪等性**: Upsert は上書き、Delete は NotFound を吸収
- **ベストエフォート**: Strict オプション未指定時は失敗を Warn ログで吸収
- **クロスサブスクリプション**: ゾーンの ResourceID からサブスクリプションを抽出するため対応可能

- **前提条件**:
  - DNS Zone Contributor ロールが必要 (`ClusterInstall()` で付与)
  - DNS ゾーンは事前に作成されている必要がある

詳細は [K4x-ADR-004] を参照。

---

## 8. Ingress TLS 証明書 (Key Vault 連携)

### 8.1 概要

`cluster.Ingress.Certificates` に Azure Key Vault の Secret URL が設定されている場合、`ClusterInstall()` 内で以下を実行する。

1. Key Vault Secrets User ロールを Ingress ServiceAccount の Managed Identity へ付与 (`ensureAzureRoleKeyVaultSecret`)
2. Key Vault ごとに SecretProviderClass リソースを生成 (`ensureSecretProviderClassFromKeyVault`)
3. Traefik Pod に CSI ボリュームをマウント
4. Traefik の `certs.yaml` に証明書パスを注入

### 8.2 Key Vault Secret URL パース

`parseKeyVaultSecretURL()` (secret.go) が `https://<vault>.vault.azure.net/secrets/<name>[/<version>]` 形式を解析する。

### 8.3 SecretProviderClass 命名

`spcNameForVault()` により `<traefik-release>-kv-<sanitized-vault-name>` 形式で生成。

### 8.4 Key Vault リソース ID の解決

`azureKeyVaultResourceIDs()` (azure_kv.go) が Azure Resource Graph を使用して参照されている Key Vault のリソース ID を検索する。

---

## 9. ACR 連携

### 9.1 設定

ACR リソース ID は Cluster Settings の `AZURE_AKS_CONTAINER_REGISTRY_RESOURCE_IDS` にカンマまたはスペース区切りで指定する。

各 ID は `arm.ParseResourceID()` で検証され、`Microsoft.ContainerRegistry/registries` リソースタイプであることが確認される。

### 9.2 ロール付与

`ClusterInstall()` 内で `ensureAzureContainerRegistryRoles()` (azure_cr.go) が AcrPull ロールを Kubelet Identity に付与する。

- ベストエフォート: 失敗は Warn ログで継続
- クロスサブスクリプション: ResourceID からサブスクリプションを抽出するため対応可能
- 冪等性: 既存割り当ては `RoleAssignmentExists` で吸収

---

## 10. Volume 操作

### 10.1 Volume Backend パターン

AKS ドライバは Volume Type ごとに異なるバックエンド実装を持つ。ディスパッチは `volume.go` の `resolveVolumeDriver()` で行われる。

```
volume.go           → resolveVolumeDriver(vol) → volumeBackend インターフェース
volume_backend.go   → volumeBackend interface 定義
volume_backend_disk.go  → volumeBackendDisk (Azure Managed Disks)
volume_backend_files.go → volumeBackendFiles (Azure Files)
```

`volumeBackend` インターフェースは以下のメソッドを持つ:
- `DiskList`, `DiskCreate`, `DiskDelete`, `DiskAssign`
- `SnapshotList`, `SnapshotCreate`, `SnapshotDelete`
- `Class`

### 10.2 Volume Type

| Type | バックエンド | Azure サービス |
|---|---|---|
| `disk` (デフォルト) | `volumeBackendDisk` | Azure Managed Disks |
| `files` | `volumeBackendFiles` | Azure Files |

`Type` が省略された場合は `disk` として扱う。詳細は [K4x-ADR-014] を参照。

### 10.3 Source パラメータ解釈

`source` パラメータは不透明な文字列として CLI/UseCase からドライバに渡される。AKS ドライバでの解釈:

| 入力パターン | 解釈 |
|---|---|
| 空文字列 | 新規作成 (ディスク) / Assigned ディスクから自動選択 (スナップショット) |
| `snapshot:<name>` | Kompox スナップショット名 |
| `disk:<name>` | Kompox ディスク名 |
| `/subscriptions/...` | Azure リソース ID |
| `arm:...` | Azure リソース ID |
| `resourceId:...` | Azure リソース ID |

フォールバック:
- `VolumeDiskCreate`: 解決不能時に `snapshot:` プレフィクスを付与して再解決
- `VolumeSnapshotCreate`: 解決不能時に `disk:` プレフィクスを付与して再解決

`Type=files` では source 非サポート (空文字以外はエラー)。

詳細は [K4x-ADR-003] を参照。

---

## 11. Azure Disks (Type=disk)

Azure Managed Disks と Azure Managed Disk Snapshots をアプリ用リソースグループ内で管理する。

実装: volume_backend_disk.go

### 11.1 Handle

- Azure Disk リソース ID: `/subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Compute/disks/{name}`

### 11.2 Assigned フラグ

- `kompox-disk-assigned` タグで管理 (`"true"` / `"false"`)
- 各論理ボリュームに対して `Assigned=true` のディスクは 1 つのみ
- 初回作成ディスクは自動的に `Assigned=true`
- 2 個目以降は `Assigned=false`

### 11.3 VolumeDiskList()

- **タイムアウト**: 1 分
- アプリ RG 内の全ディスクを `armcompute.DisksClient` でページング取得
- `kompox-volume` タグでフィルタリング
- `newDisk()` で `model.VolumeDisk` に変換 (ボリューム名不一致は `nil` で除外)
- `CreatedAt` 降順ソート (同時刻は `Name` 昇順)
- RG 不存在 (404) は空配列を返却

### 11.4 VolumeDiskCreate()

- **タイムアウト**: 2 分
- **処理**:
  1. `ensureAzureResourceGroupCreated()` で RG を確保 (Kubelet Identity に Contributor ロール付与)
  2. 既存ディスク一覧を取得
  3. `diskName` 空の場合は `naming.NewCompactID()` で生成 / 指定時は重複チェック
  4. `source` を解決: 空 → `CreateOption=Empty` / 非空 → `CreateOption=Copy`
  5. 初回は `Assigned=true`、それ以外は `Assigned=false`
  6. Azure Disk を作成し poller で完了待機
  7. `model.VolumeDisk` を返却
- **SKU デフォルト**: `Premium_LRS`
- **サポート SKU**: `Standard_LRS`, `Premium_LRS`, `StandardSSD_LRS`, `UltraSSD_LRS`, `Premium_ZRS`, `StandardSSD_ZRS`, `PremiumV2_LRS`

### 11.5 VolumeDiskAssign()

- 既存ディスク一覧を取得
- 指定ディスクが存在しなければエラー
- 全ディスクの `kompox-disk-assigned` タグを更新 (指定ディスク → `"true"`, 他 → `"false"`)
- タグ更新のみ (ディスク内容は変更しない)
- 冪等性: 既に正しい状態なら更新スキップ

### 11.6 VolumeDiskDelete()

- Azure Disk を削除し poller で完了待機
- 冪等性: NotFound は成功扱い

### 11.7 VolumeSnapshotList()

- アプリ RG 内の全スナップショットを取得
- `kompox-volume` タグでフィルタリング
- `CreatedAt` 降順ソート

### 11.8 VolumeSnapshotCreate()

- **処理**:
  1. `ensureAzureResourceGroupCreated()` で RG を確保
  2. `snapName` 空の場合は `naming.NewCompactID()` で生成
  3. `source` 解決: 空 → Assigned ディスクを自動選択 (単一でない場合はエラー)
  4. source リソース ID の種別判定: Disk → `CreateOption=Copy`, Snapshot → `CreateOption=CopyStart`
  5. 増分スナップショット、SKU は `Standard_ZRS`
  6. Azure Snapshot を作成し poller で完了待機
- **冪等性**: 既存同名スナップショットがある場合はエラー (ディスクと異なり上書きしない)

### 11.9 VolumeSnapshotDelete()

- Azure Snapshot を削除し poller で完了待機
- 冪等性: NotFound は成功扱い

### 11.10 VolumeClass()

| フィールド | 値 |
|---|---|
| `StorageClassName` | `"managed-csi"` |
| `CSIDriver` | `"disk.csi.azure.com"` |
| `FSType` | `"ext4"` |
| `Attributes` | `{"fsType": "ext4"}` |
| `AccessModes` | `["ReadWriteOnce"]` |
| `ReclaimPolicy` | `"Retain"` |
| `VolumeMode` | `"Filesystem"` |

---

## 12. Azure Files (Type=files)

Azure Storage Account のファイル共有 (Azure Files) をアプリ用リソースグループ内で管理する。

実装: volume_backend_files.go, azure_storage.go

### 12.1 ストレージアカウント

- アプリ単位で 1 つのストレージアカウントを使用
- 初回の `DiskCreate` 時に `ensureStorageAccountCreated()` で自動作成
- 名前形式: `k4x{prv_hash}{app_hash}` (15 文字)
- SKU は `Options.sku` で指定 (デフォルト: `Standard_LRS`)
- サポート SKU: `Standard_LRS`, `Standard_GRS`, `Standard_RAGRS`, `Standard_ZRS`, `Premium_LRS`, `Premium_ZRS`
- セキュリティ設定: `AllowBlobPublicAccess=false`, `MinimumTLSVersion=TLS1.2`, `HTTPSTrafficOnly=true`

### 12.2 共有命名規則

- Azure Files 共有名: `{vol.name}-{disk.name}` (最大 41 文字)
- Azure Files の共有名制約 (3–63 文字) に適合

### 12.3 メタデータ

Azure Files では Azure タグの代わりに共有メタデータで論理ボリューム属性を保持する。

| メタデータキー | 用途 |
|---|---|
| `kompox_volume_name` | ボリューム名 |
| `kompox_files_share_name` | ディスク名 (Kompox 管理名) |
| `kompox_files_share_assigned` | 割り当て状態 (`"true"` / `"false"`) |

### 12.4 Handle

CSI volumeHandle 形式: `{rg}#{account}#{share}#####{subscription}` (6 個の `#` で 7 フィールド区切り)

Azure Resource ID ではなく、Azure Files CSI ドライバー固有の形式。

### 12.5 プロトコル

- 当面は SMB のみサポート (固定)
- `Options.protocol` が `"smb"` 以外の場合はエラー

### 12.6 VolumeDiskList()

- ストレージアカウントから全共有を `armstorage.FileSharesClient` でページング取得 (`Expand: metadata`)
- `kompox_volume_name` メタデータでフィルタリング
- `CreatedAt` 降順ソート
- RG またはストレージアカウント不存在 (404) は空配列を返却

### 12.7 VolumeDiskCreate()

- **処理**:
  1. プロトコル検証 (SMB のみ)
  2. SKU を Options から取得
  3. `ensureStorageAccountCreated()` でストレージアカウントを確保 (Kubelet Identity に Contributor ロール付与)
  4. `diskName` 空の場合は `naming.NewCompactID()` で自動生成
  5. 共有名 `{vol.name}-{disk.name}` を生成
  6. 既存共有一覧を取得し、初回共有か確認、重複チェック
  7. クォータ設定 (volume サイズから GiB に変換)
  8. 共有を作成しメタデータを設定
- **source**: 非サポート (指定時はエラー)

### 12.8 VolumeDiskAssign()

- 既存共有一覧を取得
- 指定共有が存在しなければ NotFound エラー
- 全共有の `kompox_files_share_assigned` メタデータを更新
- メタデータ更新は順次実行 (Azure Storage にトランザクション機能なし)

### 12.9 VolumeDiskDelete()

- 共有 `{vol.name}-{disk.name}` を削除
- 冪等性: NotFound は成功扱い
- 最後の共有を削除してもストレージアカウントは削除されない

### 12.10 VolumeClass()

| フィールド | 値 |
|---|---|
| `StorageClassName` | `"azurefile-csi"` |
| `CSIDriver` | `"file.csi.azure.com"` |
| `FSType` | `""` (空文字列) |
| `AccessModes` | `["ReadWriteMany"]` |
| `VolumeMode` | `"Filesystem"` |
| `Attributes.protocol` | `"smb"` |

- **FSType が空の理由**: Azure Files は SMB/NFS プロトコルを使用するため、ext4 等のファイルシステムタイプは不要。空にすることで `"diskname could not be empty"` エラーを回避。

### 12.11 スナップショット非対応

`VolumeSnapshot*` メソッドは `ErrNotSupported` を返す。

将来の拡張候補:
- NFS プロトコルサポート
- Azure Files ネイティブスナップショット
- Azure NetApp Files (ANF) (`backend=anf`)
- Azure Managed Lustre (`backend=lustre`)
- Azure Blob via FUSE (`backend=azureblob`)

---

## 13. NodePool 操作

本セクションは [K4x-ADR-019] および [Kompox-ProviderDriver] の契約に基づく AKS 固有の実装準拠仕様を示す。

### 13.1 用語マッピング

| Kompox (DTO) | AKS (Azure SDK) |
|---|---|
| `NodePool` | Agent Pool (`armcontainerservice.AgentPool`) |
| `NodePool.Name` | Agent Pool `Name` |
| `NodePool.Mode` (`system` / `user`) | `AgentPoolMode` (`System` / `User`) |
| `NodePool.InstanceType` | `VMSize` |
| `NodePool.OSDiskType` | `OSDiskType` (`Managed` / `Ephemeral`) |
| `NodePool.OSDiskSizeGiB` | `OSDiskSizeGB` |
| `NodePool.Priority` (`regular` / `spot`) | `ScaleSetPriority` (`Regular` / `Spot`) |
| `NodePool.Zones` | `AvailabilityZones` |
| `NodePool.Labels` | `NodeLabels` |
| `NodePool.Autoscaling.Enabled` | `EnableAutoScaling` |
| `NodePool.Autoscaling.Min` | `MinCount` |
| `NodePool.Autoscaling.Max` | `MaxCount` |
| `NodePool.Autoscaling.Desired` | `Count` (オートスケール無効時のみ) |

### 13.2 mutable / immutable 境界

| フィールド | 可変性 | 備考 |
|---|---|---|
| `Mode` | immutable | AKS API 制約 |
| `InstanceType` (VMSize) | immutable | AKS API 制約 |
| `OSDiskType` | immutable | AKS API 制約 |
| `OSDiskSizeGiB` | immutable | AKS API 制約 |
| `Priority` | immutable | AKS API 制約 |
| `Zones` | immutable | AKS API 制約 |
| `Labels` | mutable | `NodePoolUpdate` で更新可能 |
| `Autoscaling.*` | mutable | `NodePoolUpdate` で更新可能 |

`NodePoolUpdate` で immutable フィールドが non-nil で指定された場合は validation error を返す。

### 13.3 ラベルと zone 正規化

- `kompox.dev/node-pool`: ドライバが Agent Pool 作成時に NodeLabels として付与
- `kompox.dev/node-zone`: ドライバが zone 値を正規化して NodeLabels に付与
  - AKS の zone 値は `"1"`, `"2"`, `"3"` (数字のみ)
  - Kompox の zone 値はリージョン付き (`japaneast-1` 等) の可能性あり → ドライバで変換
- zone 値の正規化・変換は provider driver の責務 ([K4x-ADR-019])

### 13.4 NodePool メソッド実装記載

本節は、AKS driver に NodePool メソッドを追加する際の実装準拠仕様を定義する。

#### 13.4.1 NodePoolList

- `armcontainerservice.AgentPoolsClient.NewListPager()` で Agent Pool 一覧を取得する。
- 各 Agent Pool を `model.NodePool` に変換して返却する。
- 変換時は次を満たす:
  - `Mode`: `System` / `User` を `system` / `user` に正規化
  - `Zones`: AKS 数字ゾーン (`"1"` など) を Kompox 形式へ変換
  - `Labels`: `NodeLabels` をそのまま取り込み、`kompox.dev/node-pool` / `kompox.dev/node-zone` が存在する場合は整合を検証
- クラスタ未作成・RG 未存在に起因する NotFound は空配列を返す。

#### 13.4.2 NodePoolCreate

- `armcontainerservice.AgentPoolsClient.BeginCreateOrUpdate()` を使用する。
- `Name` が空の場合は validation error。
- immutable 項目 (`Mode`, `InstanceType`, `OSDiskType`, `OSDiskSizeGiB`, `Priority`, `Zones`) は作成時のみ受理する。
- `Autoscaling.Enabled=true` の場合:
  - `Min` / `Max` は必須
  - `Desired` は指定されても API リクエストの `Count` には反映しない
- `Autoscaling.Enabled=false` の場合:
  - `Desired` を `Count` に反映
- `kompox.dev/node-pool` / `kompox.dev/node-zone` を NodeLabels に付与する。

#### 13.4.3 NodePoolUpdate

- 対象 Agent Pool を取得し、`BeginCreateOrUpdate()` による upsert で更新する。
- リクエストは non-nil フィールドのみを既存値へマージする (partial update)。
- immutable 項目に non-nil 指定がある場合は validation error。
- mutable 項目の更新:
  - `Labels`: 差分を反映し、`kompox.dev/node-pool` / `kompox.dev/node-zone` を再計算
  - `Autoscaling.*`: `EnableAutoScaling`, `MinCount`, `MaxCount`, `Count` を 13.4.2 と同じ規則で更新
- 変更差分がない場合は API 呼び出しをスキップして現在値を返す (冪等)。

#### 13.4.4 NodePoolDelete

- `armcontainerservice.AgentPoolsClient.BeginDelete()` を使用する。
- 削除完了は poller で待機する。
- 対象未存在 (`NotFound`) は成功扱いとする (冪等)。

#### 13.4.5 エラー分類

- 入力不正・契約違反: validation error
- 機能未対応: `not implemented`
- Azure API 失敗: provider error として透過

---

## 14. AKS ドライバ E2E テスト

- `tests/aks-e2e-nodepool`: `cluster nodepool` の list/create/update/delete を対象に、AKS 上で NodePool 操作経路を検証する。
- `tests/aks-e2e-volume`: Disk/Snapshot 系の操作を対象に、AKS ドライバの Volume 操作経路を検証する。

---

## 15. ソースファイル構成

| ファイル | 責務 |
|---|---|
| `driver.go` | ドライバ構造体定義、ファクトリ、`init()` による自己登録 |
| `cluster.go` | Cluster ライフサイクルメソッド (`Provision` / `Deprovision` / `Status` / `Install` / `Uninstall` / `Kubeconfig` / `DNSApply`) |
| `naming.go` | 命名規則 (定数、RG 名生成、ディスク/スナップショット/ストレージアカウント名生成、タグ定数) |
| `logging.go` | `withMethodLogger()` Span パターン |
| `embed.go` | ARM テンプレート (`main.json`) の `//go:embed` |
| `main.json` | サブスクリプションスコープ ARM テンプレート |
| `volume.go` | Volume メソッドのエントリポイント (Type 別ディスパッチ) |
| `volume_backend.go` | `volumeBackend` インターフェース定義 |
| `volume_backend_disk.go` | Azure Managed Disks バックエンド |
| `volume_backend_files.go` | Azure Files バックエンド |
| `azure_aks.go` | AKS SDK ヘルパー (デプロイメント管理、kubeconfig 取得) |
| `azure_dns.go` | Azure DNS SDK ヘルパー (ゾーン解析、レコード操作) |
| `azure_cr.go` | Azure Container Registry SDK ヘルパー |
| `azure_kv.go` | Azure Key Vault SDK ヘルパー (リソース検索、パージ) |
| `azure_resources.go` | Azure Resource Group SDK ヘルパー (作成、削除、KV パージ対応) |
| `azure_roles.go` | Azure RBAC ロール割り当てヘルパー |
| `azure_storage.go` | Azure Storage Account 作成ヘルパー |
| `nodepool.go` | NodePool (`List/Create/Update/Delete`) 実装 |
| `nodepool_test.go` | NodePool 変換/正規化/immutable 検証ユニットテスト |
| `secret.go` | Key Vault → SecretProviderClass 生成 |
| `naming_test.go` | 命名規則ユニットテスト |
| `azure_dns_test.go` | DNS ヘルパーユニットテスト |
| `azure_cr_test.go` | ACR ヘルパーユニットテスト |

---

## 参考文献

- [Kompox-ProviderDriver] — Provider Driver の公開契約と実装ガイドライン
- [Kompox-KOM] — Kompox Ops Manifest 仕様
- [K4x-ADR-003] — Disk/Snapshot CLI フラグ統一と Source パラメータの不透明化
- [K4x-ADR-004] — Cluster ingress endpoint DNS 自動更新
- [K4x-ADR-014] — Volume Type の導入 (disk/files)
- [K4x-ADR-019] — NodePool 抽象の導入

[Kompox-ProviderDriver]: ./Kompox-ProviderDriver.ja.md
[Kompox-KOM]: ./Kompox-KOM.ja.md
[K4x-ADR-003]: ../adr/K4x-ADR-003.md
[K4x-ADR-004]: ../adr/K4x-ADR-004.md
[K4x-ADR-014]: ../adr/K4x-ADR-014.md
[K4x-ADR-019]: ../adr/K4x-ADR-019.md
