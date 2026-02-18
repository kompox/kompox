---
id: Kompox-ProviderDriver-OKE-DesignStudy
title: OKE Provider Driver 設計検討
version: v1
status: draft
updated: 2026-02-18T12:26:37Z
language: ja
---

# OKE Provider Driver 設計検討

本書は Kompox での OKE (Oracle Container Engine for Kubernetes) Provider Driver 実装の設計検討をまとめる。
AKS Provider Driver ([Kompox-ProviderDriver-AKS]) を実装済みの参照として、論点ごとに「AKS での実装」と「OKE での方針」を対比しながら記述する。

Driver インターフェースの共通契約については [Kompox-ProviderDriver] を参照。

---

## 1. 認証

### AKS の実装

`AZURE_AUTH_METHOD` の値に応じて `azcore.TokenCredential` を生成し、全 Azure SDK クライアントで共有する。
サポートする認証方式: `client_secret` / `managed_identity` / `workload_identity` / `azure_cli` / `azure_developer_cli`。

### OKE での方針

`OCI_AUTH_METHOD` の値に応じて `common.ConfigurationProvider` を生成し、全 OCI SDK クライアントで共有する。

| `OCI_AUTH_METHOD` | 追加設定 | 備考 |
|---|---|---|
| `instance_principal` | なし | OCI Compute インスタンス上での実行。AKS の `managed_identity` に相当 |
| `user_principal` | `OCI_USER_OCID`, `OCI_FINGERPRINT`, `OCI_PRIVATE_KEY_FILE` | API Key 認証。AKS の `client_secret` に最も近い |
| `workload_identity` | `OCI_RESOURCE_PRINCIPAL_VERSION` (環境変数) | OKE Pod 上からの呼び出し。AKS の `workload_identity` に相当 |

**共通点**: どの方式も factory 関数内で `configProvider` を生成し、ドライバが保持する。以降の全 API 呼び出しはこれを渡すだけでよい。

**差異**: OCI には `azure_cli` / `azure_developer_cli` 相当のローカル開発向け認証方式がない。`user_principal` を `OCI_PRIVATE_KEY_FILE` とともに使用することがローカル開発向けの選択肢になる。

---

## 2. スコープ管理: Resource Group とコンパートメント

### AKS の実装

Azure では **Resource Group (RG)** がリソース境界。RG は削除可能なコンテナであり、中身ごと一括削除できる (`BeginDelete` で非同期削除。ポーリング待機)。

- クラスタ用 RG: `{prefix}_cls_{cluster.name}_{cls_hash}` で決定的命名、`ClusterProvision` 時に自動作成
- アプリ用 RG: `{prefix}_app_{app.name}_{app_hash}` で決定的命名、Volume 初回 Create 時に自動作成

`AZURE_RESOURCE_GROUP_NAME` で明示指定も可能 (既存 RG を使う場合)。

### OKE での方針

OCI では **コンパートメント** が RG に対応するスコープ境界。

```
Azure                                OCI
──────────────────────────────────   ──────────────────────────────────────────
Subscription                    ←→  Tenancy (root compartment)
  (ドライバに直接対応なし)         ←→  OCI_COMPARTMENT_OCID (親、Provider Settings で指定)
  Resource Group (クラスタ用)    ←→  子コンパートメント (クラスタ用、自動作成)
    AKS Cluster                          OKE Cluster
    Key Vault                             OCI Vault
    Log Analytics                         OCI Logging
  Resource Group (アプリ用)      ←→  子コンパートメント (アプリ用、自動作成)
    Managed Disk                           Block Volume
    Storage Account + Azure Files          File Storage
```

#### 追加スコープ設定

AKS では Subscription が固定スコープだが、OCI では親コンパートメントを Provider Settings で指定する。
この親コンパートメントの下にクラスタ用・アプリ用の子コンパートメントを自動作成する。

| OCI Provider Setting | 対応する AKS の概念 | 必須 |
|---|---|---|
| `OCI_TENANCY_OCID` | Subscription Tenant ID (参照のみ) | ○ |
| `OCI_COMPARTMENT_OCID` | Subscription ID の下位スコープ | ○ |

#### コンパートメントの削除

AKS の `ensureAzureResourceGroupDeleted()` は `BeginDelete` でコンパートメント内のリソースを含めて一括削除できる。
OCI の `DeleteCompartment` はコンパートメント内にリソースが残っている場合エラーになる。
このため、OKE ドライバの `ClusterDeprovision()` および `VolumeDiskDelete()` は「**内部リソースをすべて個別削除してからコンパートメントを削除**」という順序を守らなければならない。

```
AKS ClusterDeprovision:
  1. ensureAzureDeploymentDeleted (ベストエフォート)
  2. ensureAzureResourceGroupDeleted (RG ごと一括)

OKE ClusterDeprovision: (順序が重要)
  1. Node Pool 削除
  2. OKE Cluster 削除
  3. IAM Policy / Dynamic Group 削除
  4. Vault Secret パージ (ベストエフォート)
  5. VCN / Subnet 削除
  6. ensureCompartmentDeleted (コンパートメントが空になってから)
```

---

## 3. ステート管理: 単一情報源の設計

### AKS の実装 (現行・負債あり)

AKS の `ClusterProvision()` は ARM テンプレートをサブスクリプションスコープでデプロイし、そのデプロイメントレコードの出力 (`azureDeploymentOutputs()`) を**単一情報源**として使用する。

記録される主な非決定的な値:

| 出力キー | 値 |
|---|---|
| `AZURE_AKS_CLUSTER_NAME` | AKS Cluster のリソース名 |
| `AZURE_AKS_CLUSTER_PRINCIPAL_ID` | Cluster Identity の Principal ID |
| `AZURE_AKS_KUBELET_PRINCIPAL_ID` | Kubelet Identity の Principal ID |
| `AZURE_INGRESS_SERVICE_ACCOUNT_PRINCIPAL_ID` | Ingress SA の Principal ID |
| `AZURE_INGRESS_SERVICE_ACCOUNT_CLIENT_ID` | Ingress SA の Client ID |

この設計の弱点:
- サブスクリプションスコープのデプロイメント数には上限 (800 件) があり、Azure が古いレコードを自動削除する可能性がある
- 運用者が誤ってデプロイメントレコードを手動削除するとすべての管理操作が不可能になる
- outputs の更新には ARM テンプレートの再デプロイが必要

設計ドキュメント ([Kompox-ProviderDriver-AKS]) には将来の改善として「**RG タグへの移行 (Phase 3)**」が記載されている。

### OKE での方針

OKE ドライバは最初から **コンパートメントの Freeform Tags** を単一情報源として採用し、AKS の負債を回避する。

クラスタ用コンパートメントに記録する値:

| タグキー | 記録する値 | AKS 出力キーとの対応 |
|---|---|---|
| `kompox/oke-cluster-ocid` | OKE Cluster の OCID | `AZURE_AKS_CLUSTER_NAME` |
| `kompox/oke-node-dynamic-group-ocid` | Node Pool Dynamic Group の OCID | `AZURE_AKS_KUBELET_PRINCIPAL_ID` |
| `kompox/ingress-sa-principal-ocid` | Ingress SA 用 Principal の OCID | `AZURE_INGRESS_SERVICE_ACCOUNT_PRINCIPAL_ID` |
| `kompox/ingress-sa-client-id` | Workload Identity Client ID | `AZURE_INGRESS_SERVICE_ACCOUNT_CLIENT_ID` |

これは [Kompox-ProviderDriver] が推奨する「リソースタグ / ラベル」パターンの適用であり、AKS の将来目標を OKE では設計当初から実現する。

コンパートメントはクラスタの存続期間中は消滅しないため、このアプローチは ARM デプロイメントレコードより堅牢である。コンパートメント名は決定的に導出できるため、参照を喪失することもない。

---

## 4. IaC の採用可否

### AKS の実装

`infra/aks/infra/main.bicep` が以下のリソースを一括作成する:

- Resource Group
- Key Vault
- Log Analytics Workspace + Application Insights
- User Assigned Managed Identity + Federated Credential
- Storage Account
- AKS Cluster (System Node Pool + User Node Pool を含む)

Bicep を ARM JSON (`main.json`) にコンパイルし、Go バイナリに `//go:embed` で埋め込む。`ClusterProvision()` はこの JSON をサブスクリプションスコープのデプロイメントとして実行し、Outputs をステートとして使用する。

### OKE での方針: IaC を採用しない

OCI には以下の 2 つの選択肢がある:

**選択肢 A: OCI Resource Manager (Terraform スタック)**
OCI マネージド環境でTerraform HCL を実行するサービス。ARM デプロイメントに構造が近い。
ただし Terraform のステートファイルが Object Storage バケットに保存されるため、Kompox の設計原則「ユーザーにステートファイルを管理させない」に反する。

**選択肢 B: OCI Go SDK による直接 API 呼び出し (採用)**
ARM テンプレートが一括作成していたリソースを、それぞれ `ensure*()` 関数として Go コードに実装する。

```
ClusterProvision()
  ├── ensureCompartmentCreated()       // RG 作成に相当
  ├── ensureVCNCreated()               // VNet に相当
  ├── ensureOKEClusterCreated()        // AKS Cluster に相当 (Work Request ポーリング)
  ├── ensureSystemNodePoolCreated()    // System Agent Pool に相当
  ├── ensureDynamicGroupCreated()      // Managed Identity に相当
  ├── ensurePoliciesCreated()          // RBAC Role Assignment に相当
  └── [オプション] ensureVaultCreated() // Key Vault に相当
```

この方針により:
- `infra/oke/` ディレクトリや埋め込みテンプレートは不要
- デプロイメントレコードへの依存が最初から存在しない
- コンパートメントタグを情報源とする設計が自然に成立する
- `ensure*()` の冪等性は各 OCI SDK 呼び出しで個別に管理できる

| 比較観点 | ARM テンプレート (AKS) | OCI Resource Manager | OCI Go SDK 直接 (採用) |
|---|---|---|---|
| ステート依存 | デプロイメントレコード (上限・削除リスク) | Terraform ステートファイル | **不要** (コンパートメントタグ) |
| ユーザー管理コスト | デプロイメント履歴を管理 | ステートバックエンドを管理 | **なし** |
| 部分更新 | テンプレート全体の再デプロイ | 差分適用 (ステート依存) | **リソース単位で即時** |
| コード量 | IaC + Go | IaC + Go | Go のみ |

---

## 5. クラウドリソース対応表

### 5.1 コア (K8s クラスタ基盤)

| AKS リソース | Azure サービス | OKE 対応リソース | OCI サービス | 備考 |
|---|---|---|---|---|
| Managed Cluster | AKS | OKE Cluster | Container Engine for Kubernetes | `ClusterProvision` の主対象 |
| System Agent Pool | AKS Agent Pool | Node Pool (初期 system 用) | OKE Node Pool | ARM テンプレート内で一括作成 → `ensure*()` で個別作成 |
| User Agent Pool | AKS Agent Pool | Node Pool | OKE Node Pool | `NodePoolCreate` の対象 |
| OIDC Issuer URL | AKS 組み込み | OIDC Issuer | OKE 組み込み | Workload Identity に使用 |

### 5.2 スコープ境界・プロビジョニング

| AKS リソース / 概念 | OKE 対応 | 備考 |
|---|---|---|
| Resource Group (クラスタ用) | コンパートメント (クラスタ用) | 決定的命名で自動作成 |
| Resource Group (アプリ用) | コンパートメント (アプリ用) | Volume 初回 Create 時に自動作成 |
| ARM デプロイメントレコード | コンパートメント Freeform Tags | 非決定的な値の記録先 |
| Log Analytics Workspace | OCI Logging Analytics (オプション) | `ClusterProvision` のオプション扱い |

### 5.3 認証・アクセス制御

| AKS リソース | Azure サービス | OKE 対応リソース | OCI サービス |
|---|---|---|---|
| Cluster Identity (system-assigned MI) | Azure Managed Identity | Instance Principal (Dynamic Group) | OCI IAM |
| Kubelet Identity (user-assigned MI) | Azure Managed Identity | Instance Principal (Node Pool の Dynamic Group) | OCI IAM |
| Role Assignment (Contributor on RG) | Azure RBAC | IAM Policy (コンパートメントスコープ) | OCI IAM |
| Role Assignment (DNS Zone Contributor) | Azure RBAC | `manage dns-zones in compartment` Policy | OCI IAM |
| Role Assignment (AcrPull) | Azure RBAC | `read repos in compartment` Policy | OCI IAM |
| Role Assignment (Key Vault Secrets User) | Azure RBAC | `read secret-bundles in vault` Policy | OCI IAM |
| Workload Identity Federated Credential | Azure AD | OKE Workload Identity (OIDC) | OCI IAM |

**AKS との重要な差異**: Azure の Managed Identity は個別のリソースとして作成・参照するが、OCI の Instance Principal は Dynamic Group + IAM Policy の組み合わせで実現する。`ensureAzureRole()` に相当する `ensureOCIPolicy()` を実装する。

### 5.4 DNS

| AKS リソース | Azure サービス | OKE 対応リソース | OCI サービス |
|---|---|---|---|
| DNS Zone | Azure DNS | DNS Zone | OCI DNS |
| DNS Record Set (A/AAAA/CNAME) | Azure DNS Record Set | DNS RRSet | OCI DNS |

AKS Cluster Settings の `AZURE_AKS_DNS_ZONE_RESOURCE_IDS` (Azure Resource ID 形式) に対応して、OKE では `OCI_OKE_DNS_ZONE_IDS` (OCID 形式) を使う。
`ClusterDNSApply()` は OCI DNS の `PatchZoneRecords` / `DeleteZoneRecords` を使用して実装する。DNS Zone Contributor ロールの代わりに `manage dns-zones in compartment` Policy を `ClusterInstall()` で付与する。

### 5.5 コンテナレジストリ

| AKS リソース | Azure サービス | OKE 対応リソース | OCI サービス |
|---|---|---|---|
| Azure Container Registry | ACR | OCIR (レポジトリパスで識別) | OCI Container Registry |

**重要な差異**: ACR はリソース単位でリソース ID (`/subscriptions/…/Microsoft.ContainerRegistry/registries/<name>`) を持つが、OCIR はテナント共有のレジストリでありリソース ID の概念が異なる。Cluster Settings では **リポジトリパスプレフィクス** (`<region>.ocir.io/<tenancy-namespace>`) を `OCI_OKE_CONTAINER_REGISTRY_REPOS` で指定する。

`AcrPull` ロールの代わりに `read repos in compartment` Policy を Kubelet Identity (Node Pool Dynamic Group) に付与する。

### 5.6 ストレージ (Volume バックエンド)

| AKS リソース | Azure サービス | OKE 対応リソース | OCI サービス |
|---|---|---|---|
| Managed Disk | Azure Managed Disks | Block Volume | OCI Block Storage |
| Managed Disk Snapshot | Azure Managed Disk Snapshots | Block Volume Backup | OCI Block Storage |
| Storage Account + Azure Files | Azure Files | File System + Mount Target + Export | OCI File Storage |
| Azure Files: SMB プロトコル | Azure Files (SMB 対応) | **非対応** | OCI File Storage は NFS のみ |

### 5.7 証明書・シークレット

| AKS リソース | Azure サービス | OKE 対応リソース | OCI サービス |
|---|---|---|---|
| Key Vault | Azure Key Vault | Vault | OCI Vault |
| Key Vault Secret | Azure Key Vault Secret | Secret | OCI Vault Secret |
| SecretProviderClass (`azure` provider) | CSI Secrets Store | SecretProviderClass (`oci` provider) | Secrets Store CSI Driver |

`cluster.ingress.certificates[].source` の値が AKS では Key Vault Secret URL (`https://<vault>.vault.azure.net/secrets/<name>`) であるのに対し、OKE では **OCI Vault Secret の OCID** (`ocid1.vaultsecret.oc1…`) となる。`parseKeyVaultSecretURL()` に相当する `parseOCIVaultSecretOCID()` を実装する。

---

## 6. ノードプール

### AKS の実装

AKS の AgentPool は `mode=System` / `mode=User` を持ち、Kompox の `NodePool.Mode` に直接対応する。
ゾーン識別子は数字のみ (`"1"`, `"2"`, `"3"`) であり、Kompox の `<region>-<index>` 形式への変換はドライバが担う。

VM サイズは単一の `vmSize` 文字列 (例: `Standard_D2ds_v4`)。
Spot インスタンスは `ScaleSetPriority=Spot` で指定。

### OKE での方針

| Kompox DTO | AKS パラメータ | OKE パラメータ | 差異 |
|---|---|---|---|
| `NodePool.Mode` (`system`/`user`) | `AgentPoolMode` | **直接対応なし** | OKE は NodePool に mode 概念がない。`kompox.dev/node-pool` ラベルをノードに付与して役割を識別する |
| `NodePool.InstanceType` | `vmSize` | `nodeShape` | 例: `VM.Standard.E4.Flex` |
| `NodePool.OSDiskType` | `osDiskType` (`Managed`/`Ephemeral`) | **直接対応なし** | OKE の Boot Volume は常に Block Storage。Ephemeral OS Disk に相当する機能は現時点でない |
| `NodePool.OSDiskSizeGiB` | `osDiskSizeGB` | Boot Volume サイズ (GB) | 同等 |
| `NodePool.Priority` (`regular`/`spot`) | `ScaleSetPriority` (`Regular`/`Spot`) | `isPreemptible` (`true`/`false`) | OCI Preemptible Instance は最大 24 時間後に強制回収。AKS Spot の任意削除とは挙動が異なる |
| `NodePool.Zones` | `availabilityZones` (例: `["1","2"]`) | `availabilityDomain` (例: `<hash>:AP-OSAKA-1-AD-1`) | OCI の AD 識別子はテナント固有のハッシュ付き文字列で、`AD-1`/`AD-2`/`AD-3` との正規化変換が必要 |
| `NodePool.Autoscaling.*` | `EnableAutoScaling`, `MinCount`, `MaxCount` | Cluster Autoscaler (別 Pod) | AKS では組み込み。OKE では Cluster Autoscaler を `ClusterInstall()` で別途デプロイする必要がある可能性がある |

**Flexible Shape の追加パラメータ**: OKE の Flex Shape (`VM.Standard.E4.Flex` 等) は OCPU 数とメモリ量を個別に指定する。AKS の `vmSize` は単一文字列で固定スペックが決まるが、OKE では `OCI_OKE_USER_OCPUS` / `OCI_OKE_USER_MEMORY_GB` のような追加設定キーが必要になる。

**immutable / mutable 境界**:

| フィールド | AKS の可変性 | OKE の可変性 |
|---|---|---|
| `InstanceType` | immutable | immutable |
| `OSDiskType` | immutable | — (概念なし) |
| `OSDiskSizeGiB` | immutable | mutable (拡張のみ可) |
| `Priority` | immutable | immutable |
| `Zones` | immutable | immutable |
| `Labels` | mutable | mutable |
| `Autoscaling.*` | mutable | mutable (Cluster Autoscaler 設定変更) |

---

## 7. Volume 操作

### 7.1 Block Volume (Type=disk): AKS との対比

| 項目 | AKS | OKE |
|---|---|---|
| Azure/OCI サービス | Azure Managed Disks | OCI Block Storage |
| Handle | Azure Resource ID | Block Volume OCID |
| Assigned フラグ記録 | `kompox-disk-assigned` タグ | `kompox-disk-assigned` Freeform Tag (同じキー名) |
| Snapshot 種別 | Managed Disk Snapshot (増分) | Block Volume Backup (Full または Incremental) |
| デフォルト SKU | `Premium_LRS` | Balanced Performance (`10` VPU/GB) |
| CSI Driver | `disk.csi.azure.com` | `blockvolume.csi.oraclecloud.com` |
| StorageClassName | `managed-csi` | `oci-bv` |
| AccessModes | `ReadWriteOnce` | `ReadWriteOnce` |
| Snapshot 後の disk create source | `CreateOption=Copy` | Block Volume Clone |

Kubelet Identity の権限: AKS では Kubelet Identity に対してアプリ用 RG への `Contributor` ロールを付与する。OKE では Node Pool の Dynamic Group に対してアプリ用コンパートメントへの `manage volumes in compartment` Policy を付与する。

### 7.2 File Storage (Type=files): AKS との対比

| 項目 | AKS (Azure Files) | OKE (File Storage) |
|---|---|---|
| Azure/OCI サービス | Azure Files | OCI File Storage |
| プロトコル | SMB (固定) | NFS のみ (SMB 非対応) |
| アプリ単位の管理単位 | Storage Account (1) + File Share (N) | Mount Target (1) + File System (N) + Export (N) |
| Handle | `{rg}#{account}#{share}#####` (CSI 独自形式) | NFS マウントパス (`<MT IP>:<export path>`) |
| 初回 DiskCreate 時の自動作成 | `ensureStorageAccountCreated()` | `ensureMountTargetCreated()` |
| Assigned フラグ記録 | File Share メタデータ | File System タグ (Freeform Tag) |
| Snapshot | 非サポート | OCI File Storage Snapshot (将来対応候補) |
| CSI Driver | `file.csi.azure.com` | `fss.csi.oraclecloud.com` |
| StorageClassName | `azurefile-csi` | `oci-fss` |
| AccessModes | `ReadWriteMany` | `ReadWriteMany` |

**OCI File Storage 固有の設計課題**:
Mount Target はサブネットに配置する必要があるため、アプリ用コンパートメントにサブネットが必要になる。クラスタ VCN のサブネットを流用するか、アプリ用コンパートメントに独立したサブネットを作成するかを実装時に決定する。

---

## 8. 命名規則と Freeform Tags

### AKS との共通部分

OCI ドライバも AKS と同じ命名ハッシュ (`naming.NewHashes()`) とプレフィクス生成ロジックを使用する:

- プレフィクス: `OCI_RESOURCE_PREFIX` 指定 or `k4x-{prv_hash}`
- コンパートメント名 (クラスタ用): `{prefix}_cls_{cluster.name}_{cls_hash}` (AKS の RG 命名と同パターン)
- コンパートメント名 (アプリ用): `{prefix}_app_{app.name}_{app_hash}`

コンパートメント名の最大長は 100 文字 (AKS: RG は 90 文字)。`safeTruncate()` の上限値を調整するだけで同じ実装を使い回せる。

### Freeform Tags

タグキー名は AKS のものをそのまま引き継ぐ。OCI の Freeform Tag は `map[string]string` で表現し、AKS の `map[string]*string` (to.Ptr 経由) との形式の違いはドライバ内で吸収する。

| タグキー | 用途 |
|---|---|
| `kompox-workspace-name` | ワークスペース識別 |
| `kompox-provider-name` | プロバイダ識別 |
| `kompox-cluster-name` / `kompox-cluster-hash` | クラスタ識別 |
| `kompox-app-name` / `kompox-app-id-hash` | アプリ識別 |
| `kompox-volume` | ボリューム識別 |
| `kompox-disk-name` / `kompox-disk-assigned` | ディスク識別・割り当て状態 |
| `kompox-snapshot-name` | スナップショット識別 |
| `managed-by` | `"kompox"` 固定 |
| `kompox/oke-cluster-ocid` | OKE Cluster の OCID (ステート記録) |
| `kompox/oke-node-dynamic-group-ocid` | Node Pool Dynamic Group の OCID (ステート記録) |
| `kompox/ingress-sa-principal-ocid` | Ingress SA Principal OCID (ステート記録) |
| `kompox/ingress-sa-client-id` | Ingress SA Client ID (ステート記録) |

---

## 9. Work Request ポーリング

### AKS との差異

Azure SDK の多くの非同期操作は `Poller[T]` を返し、`PollUntilDone(ctx, nil)` で完了待機できる。
OCI SDK の非同期操作は **Work Request** を返す。完了待機には Work Request OCID をポーリングする独自ループを実装する必要がある。

```go
// OKE クラスタ作成の例
resp, err := okeClient.CreateCluster(ctx, oci_ce.CreateClusterRequest{...})
// resp.OpcWorkRequestId がポーリング対象
workReqID := resp.OpcWorkRequestId
// WorkRequestsClient.GetWorkRequest() でステータスを確認
// Succeeded / Failed / Canceled が終了状態
```

`ensureOKEClusterCreated()` 内にこのポーリングループを実装する。タイムアウトは `context.WithTimeout` で制御する (AKS と同様)。

---

## 10. Provider Settings / Cluster Settings の設計案

### Provider Settings

| キー | 必須 | 説明 |
|---|---|---|
| `OCI_AUTH_METHOD` | ○ | 認証方式 |
| `OCI_TENANCY_OCID` | ○ | テナンシーの OCID |
| `OCI_COMPARTMENT_OCID` | ○ | 親コンパートメントの OCID |
| `OCI_REGION` | ○ | OCI リージョン (例: `ap-osaka-1`) |
| `OCI_RESOURCE_PREFIX` | — | リソース名プレフィクス (省略時: `k4x-{prv_hash}`) |
| `OCI_USER_OCID` | `user_principal` 時 | API Key 認証 |
| `OCI_FINGERPRINT` | `user_principal` 時 | API Key 認証 |
| `OCI_PRIVATE_KEY_FILE` | `user_principal` 時 | API Key 認証 |

### Cluster Settings

| キー | 説明 | AKS 対応 |
|---|---|---|
| `OCI_COMPARTMENT_NAME` | クラスタ用コンパートメント名 (省略時自動生成) | `AZURE_RESOURCE_GROUP_NAME` |
| `OCI_OKE_DNS_ZONE_IDS` | DNS Zone OCID リスト | `AZURE_AKS_DNS_ZONE_RESOURCE_IDS` |
| `OCI_OKE_CONTAINER_REGISTRY_REPOS` | OCIR リポジトリパスプレフィクスリスト | `AZURE_AKS_CONTAINER_REGISTRY_RESOURCE_IDS` |
| `OCI_OKE_SYSTEM_SHAPE` | System Pool の Shape | `AZURE_AKS_SYSTEM_VM_SIZE` |
| `OCI_OKE_SYSTEM_OCPUS` | System Pool の OCPU 数 | — (AKS は vmSize に込み) |
| `OCI_OKE_SYSTEM_MEMORY_GB` | System Pool のメモリ量 (GB) | — (AKS は vmSize に込み) |
| `OCI_OKE_SYSTEM_BOOT_VOLUME_GB` | System Pool の Boot Volume (GB) | `AZURE_AKS_SYSTEM_VM_DISK_SIZE_GB` |
| `OCI_OKE_SYSTEM_ZONES` | System Pool の Availability Domain | `AZURE_AKS_SYSTEM_VM_ZONES` |
| `OCI_OKE_USER_SHAPE` | User Pool の Shape | `AZURE_AKS_USER_VM_SIZE` |
| `OCI_OKE_USER_OCPUS` | User Pool の OCPU 数 | — |
| `OCI_OKE_USER_MEMORY_GB` | User Pool のメモリ量 (GB) | — |
| `OCI_OKE_USER_BOOT_VOLUME_GB` | User Pool の Boot Volume (GB) | `AZURE_AKS_USER_VM_DISK_SIZE_GB` |
| `OCI_OKE_USER_PREEMPTIBLE` | `"true"` / `"false"` | `AZURE_AKS_USER_VM_PRIORITY` (`Spot`/`Regular`) |
| `OCI_OKE_USER_ZONES` | User Pool の Availability Domain | `AZURE_AKS_USER_VM_ZONES` |

### App Settings

| キー | 説明 | AKS 対応 |
|---|---|---|
| `OCI_COMPARTMENT_NAME` | アプリ用コンパートメント名 (省略時自動生成) | `AZURE_RESOURCE_GROUP_NAME` |

---

## 11. kompoxapp.yml の例

AKS 版 (`_tmp/tests/aks-e2e-basic/kompoxapp.yml`) との対比:

```yaml
apiVersion: ops.kompox.dev/v1alpha1
kind: Defaults
metadata:
  name: defaults
spec: {}
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: oke-e2e-basic-20260218-121345
  annotations:
    ops.kompox.dev/id: /ws/oke-e2e-basic-20260218-121345
spec: {}
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: oke1
  annotations:
    ops.kompox.dev/id: /ws/oke-e2e-basic-20260218-121345/prv/oke1
spec:
  driver: oke
  settings:
    # AKS: AZURE_AUTH_METHOD: azure_cli
    OCI_AUTH_METHOD: instance_principal
    # AKS: AZURE_SUBSCRIPTION_ID
    OCI_TENANCY_OCID: ocid1.tenancy.oc1..aaaaaaaxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
    # AKS に対応概念なし (サブスクリプション直下に RG を作るが、OCI では親コンパートメントが必要)
    OCI_COMPARTMENT_OCID: ocid1.compartment.oc1..aaaaaaaxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
    # AKS: AZURE_LOCATION: eastus
    OCI_REGION: ap-osaka-1
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  name: cluster1
  annotations:
    ops.kompox.dev/id: /ws/oke-e2e-basic-20260218-121345/prv/oke1/cls/cluster1
spec:
  existing: false
  protection:
    provisioning: readOnly
    installation: readOnly
  ingress:
    certEmail: user@example.com
    certResolver: staging
    domain: cluster1.oke1.exp.kompox.dev
    certificates:
      # AKS: source: https://<vault>.vault.azure.net/secrets/<name>
      - name: l0wdevtls-cluster
        source: ocid1.vaultsecret.oc1.ap-osaka-1.amaaaaaa...
  settings:
    # AKS: AZURE_AKS_SYSTEM_VM_SIZE: Standard_D2ds_v4
    OCI_OKE_SYSTEM_SHAPE: VM.Standard.E4.Flex
    # AKS: (vmSize に込み)
    OCI_OKE_SYSTEM_OCPUS: "2"
    OCI_OKE_SYSTEM_MEMORY_GB: "8"
    # AKS: AZURE_AKS_SYSTEM_VM_DISK_SIZE_GB: 64
    OCI_OKE_SYSTEM_BOOT_VOLUME_GB: "50"
    # AKS: AZURE_AKS_SYSTEM_VM_DISK_TYPE: Ephemeral (OKE に相当なし)
    # AKS: AZURE_AKS_SYSTEM_VM_PRIORITY: Regular (system は常に regular)
    # AKS: AZURE_AKS_SYSTEM_VM_ZONES: ""
    OCI_OKE_SYSTEM_ZONES: ""
    # AKS: AZURE_AKS_USER_VM_SIZE: Standard_D2ds_v4
    OCI_OKE_USER_SHAPE: VM.Standard.E4.Flex
    OCI_OKE_USER_OCPUS: "2"
    OCI_OKE_USER_MEMORY_GB: "8"
    # AKS: AZURE_AKS_USER_VM_DISK_SIZE_GB: 64
    OCI_OKE_USER_BOOT_VOLUME_GB: "50"
    # AKS: AZURE_AKS_USER_VM_PRIORITY: Regular → Spot に対応
    OCI_OKE_USER_PREEMPTIBLE: "false"
    # AKS: AZURE_AKS_USER_VM_ZONES: 1
    OCI_OKE_USER_ZONES: "AD-1"
    # AKS: AZURE_AKS_DNS_ZONE_RESOURCE_IDS: /subscriptions/…/Microsoft.Network/dnszones/…
    OCI_OKE_DNS_ZONE_IDS: ocid1.dns-zone.oc1..aaaaaaaxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
    # AKS: AZURE_AKS_CONTAINER_REGISTRY_RESOURCE_IDS: /subscriptions/…/Microsoft.ContainerRegistry/registries/…
    OCI_OKE_CONTAINER_REGISTRY_REPOS: ap-osaka-1.ocir.io/mytenancy
---
apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  name: basic
  annotations:
    ops.kompox.dev/id: /ws/oke-e2e-basic-20260218-121345/prv/oke1/cls/cluster1/app/basic
spec:
  compose: file:compose.yml
  ingress:
    certResolver: staging
    rules:
      - name: main
        port: 8080
        hosts: [whoami.custom.exp.kompox.dev]
  deployment:
    # AKS: zone: "1" (数字のみ)
    zone: "AD-1"
```

---

## 12. 未決事項

| 項目 | 内容 |
|---|---|
| OKE Workload Identity のアノテーション仕様 | `ClusterInstall()` で ServiceAccount に付与するアノテーションの正確な形式。OKE 固有か標準 OIDC か |
| Cluster Autoscaler の扱い | AKS は組み込みだが OKE は別途 Cluster Autoscaler Pod のデプロイが必要。`ClusterInstall()` に含めるか |
| File Storage の Mount Target サブネット | クラスタ VCN のサブネットを流用するか、アプリ用コンパートメントに独立サブネットを作るか |
| Availability Domain の自動検出 | `OCI_OKE_USER_ZONES` 未指定時のデフォルト動作 (全 AD に分散か、AD-1 固定か) |
| AD 識別子の正規化方式 | OCI の `<hash>:AP-OSAKA-1-AD-1` ↔ Kompox の `AD-1` / `ap-osaka-1-1` の変換ルール |
| Preemptible Instance の動作差異の扱い | AKS Spot は Eviction Policy で任意削除可能だが OCI Preemptible は 24 時間後に強制回収。設定キーの説明に明記 |
| OCIR の認証トークン管理 | OCIR は Docker Hub 方式の認証 (`docker login`) が必要。Node Pool が OCIR から Image を pull するための ImagePullSecret または Instance Principal 設定の詳細 |

---

## 参考文献

- [Kompox-ProviderDriver] — Provider Driver の公開契約と実装ガイドライン
- [Kompox-ProviderDriver-AKS] — AKS 固有の実装ガイド (主要参照実装)
- [Kompox-KOM] — Kompox Ops Manifest 仕様
- [Kompox-Logging] — ロギング仕様

[Kompox-ProviderDriver]: ./Kompox-ProviderDriver.ja.md
[Kompox-ProviderDriver-AKS]: ./Kompox-ProviderDriver-AKS.ja.md
[Kompox-KOM]: ./Kompox-KOM.ja.md
[Kompox-Logging]: ./Kompox-Logging.ja.md
