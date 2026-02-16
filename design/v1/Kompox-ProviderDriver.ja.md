---
id: Kompox-ProviderDriver
title: Kompox Provider Driver ガイド
version: v1
status: synced
updated: 2026-02-16T18:33:19Z
language: ja
---

# Kompox Provider Driver ガイド v1

## 概要

本書は Kompox のクラウドプロバイダ用ドライバ(以下、プロバイダドライバ)の設計と公開契約を解説します。usecase はオーケストレーション、adapters は I/O 実装という責務分離に基づきます。

## 目的と範囲

- 目的: クラウドプロバイダ依存の操作(プロビジョニング/認証/前後処理)を担う。
- 非対象: Kubernetes API の共通操作(Namespace 作成、マニフェスト適用、待機など)は `adapters/kube` に委譲する。

## 配置と命名

- ディレクトリ: `/adapters/drivers/provider/`
- パッケージ名: `providerdrv`
- 各プロバイダの配置: `/adapters/drivers/provider/<id>/`(例: `aks/`, `k3s/`)
- 依存関係の原則: `api(cmd) → usecase → domain ← adapters(drivers, store, kube)`
  - adapters は domain に依存してよいが、usecase には依存しない。
  - usecase は adapters の抽象(ポート/ドライバ)を経由して操作を指示する。

## 公開契約(Driver インターフェース)

> 実体は `/adapters/drivers/provider/registry.go` を参照。

```go
// Driver abstracts provider-specific behavior (identifier, hooks, etc.).
// Implementations live under adapters/drivers/provider/<name> and should return a
// provider driver identifier such as "aks" via ID().
type Driver interface {
    // ID returns the provider driver identifier (e.g., "aks").
    ID() string

    // WorkspaceName returns the workspace name associated with this driver instance.
    // May return "(nil)" if no workspace is associated (e.g., for testing).
    WorkspaceName() string

    // ProviderName returns the provider name associated with this driver instance.
    ProviderName() string

    // ClusterProvision provisions a Kubernetes cluster according to the cluster specification.
    ClusterProvision(ctx context.Context, cluster *model.Cluster, opts ...model.ClusterProvisionOption) error

    // ClusterDeprovision deprovisions a Kubernetes cluster according to the cluster specification.
    ClusterDeprovision(ctx context.Context, cluster *model.Cluster, opts ...model.ClusterDeprovisionOption) error

    // ClusterStatus returns the status of a Kubernetes cluster.
    ClusterStatus(ctx context.Context, cluster *model.Cluster) (*model.ClusterStatus, error)

    // ClusterInstall installs in-cluster resources (Ingress Controller, etc.).
    ClusterInstall(ctx context.Context, cluster *model.Cluster, opts ...model.ClusterInstallOption) error

    // ClusterUninstall uninstalls in-cluster resources (Ingress Controller, etc.).
    ClusterUninstall(ctx context.Context, cluster *model.Cluster, opts ...model.ClusterUninstallOption) error

    // ClusterKubeconfig returns kubeconfig bytes for connecting to the target cluster.
    // Implementations may fetch admin/user credentials depending on provider capability.
    ClusterKubeconfig(ctx context.Context, cluster *model.Cluster) ([]byte, error)

    // ClusterDNSApply applies a DNS record set in the provider-managed DNS zones.
    // The method must be idempotent and best-effort: providers should suppress recoverable
    // write failures unless opts request strict handling. Invalid input or context
    // cancellation should still return an error.
    ClusterDNSApply(ctx context.Context, cluster *model.Cluster, rset model.DNSRecordSet, opts ...model.ClusterDNSApplyOption) error

    // VolumeDiskList returns a list of disks of the specified logical volume.
    VolumeDiskList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, opts ...model.VolumeDiskListOption) ([]*model.VolumeDisk, error)

    // VolumeDiskCreate creates a disk of the specified logical volume. diskName and source
    // are forwarded from CLI/usecase as opaque strings. Empty values indicate provider defaults.
    VolumeDiskCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, source string, opts ...model.VolumeDiskCreateOption) (*model.VolumeDisk, error)

    // VolumeDiskDelete deletes a disk of the specified logical volume.
    VolumeDiskDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, opts ...model.VolumeDiskDeleteOption) error

    // VolumeDiskAssign assigns a disk to the specified logical volume.
    VolumeDiskAssign(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, opts ...model.VolumeDiskAssignOption) error

    // VolumeSnapshotList returns a list of snapshots of the specified volume.
    VolumeSnapshotList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, opts ...model.VolumeSnapshotListOption) ([]*model.VolumeSnapshot, error)

    // VolumeSnapshotCreate creates a snapshot. snapName and source follow the same semantics as
    // disk creation and must be interpreted by the driver.
    VolumeSnapshotCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, source string, opts ...model.VolumeSnapshotCreateOption) (*model.VolumeSnapshot, error)

    // VolumeSnapshotDelete deletes the specified snapshot.
    VolumeSnapshotDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, opts ...model.VolumeSnapshotDeleteOption) error

    // VolumeClass returns provider specific volume provisioning parameters for the given logical volume.
    // Empty fields mean "no opinion" and the caller should omit them from generated manifests rather than
    // substituting provider-specific defaults. This keeps kube layer free from provider assumptions.
    VolumeClass(ctx context.Context, cluster *model.Cluster, app *model.App, vol model.AppVolume) (model.VolumeClass, error)

    // NodePoolList returns a list of node pools for the specified cluster.
    // Implementations should return pools with associated metadata (name, zones, instance type, autoscaling, etc.).
    // Supports filtering via opts (e.g., by pool name).
    NodePoolList(ctx context.Context, cluster *model.Cluster, opts ...model.NodePoolListOption) ([]*model.NodePool, error)

    // NodePoolCreate creates a new node pool in the cluster.
    // The pool parameter specifies the desired configuration. Implementations must validate required fields
    // and return created pool metadata including provider-assigned identifiers.
    NodePoolCreate(ctx context.Context, cluster *model.Cluster, pool model.NodePool, opts ...model.NodePoolCreateOption) (*model.NodePool, error)

    // NodePoolUpdate updates mutable fields of an existing node pool.
    // Only non-nil pointer fields in pool are applied. Attempting to modify immutable fields should
    // return a validation error. Implementations determine which fields are mutable.
    NodePoolUpdate(ctx context.Context, cluster *model.Cluster, pool model.NodePool, opts ...model.NodePoolUpdateOption) (*model.NodePool, error)

    // NodePoolDelete deletes the specified node pool from the cluster.
    // poolName identifies the pool to delete. NotFound is acceptable for idempotency.
    NodePoolDelete(ctx context.Context, cluster *model.Cluster, poolName string, opts ...model.NodePoolDeleteOption) error
}
```

## 要求事項(横断)

初期段階(MVP)で必須とする要求事項:

- 冪等性: 同じ入力で複数回実行しても安全であること。Delete 系操作の `NotFound` は成功扱いを許容する。
- エラー分類: 機能未対応は `not implemented`、入力不備や不変項目変更は `validation error` として返し、利用側が確実に判別できること。
- コンテキスト: `ctx` をすべての外部呼び出しに伝播し、タイムアウト/キャンセルを尊重する。
- エラー伝播: 失敗は `fmt.Errorf("...: %w", err)` でラップし、原因追跡可能な情報を維持する。
- セキュリティ: kubeconfig/証明書/トークン/シークレットをログやエラーメッセージへ出力しない。機密情報をディスクへ平文出力しない。
- 実装方式: `kubectl`/`helm` などの shell-out を避け、Go SDK / client-go を利用する。
- ログ: [Kompox-Logging] に従い、`log/slog` による構造化ログを出力する。
- UserAgent: 外部 API 呼び出しには `kompoxops/<module>` を付与し、呼び出し元コンポーネントを識別可能にする。

初期段階で必須としないが、将来の実装で考慮すべき要求事項:

- 再試行方針: transient failure の判定条件、最大試行回数、バックオフ戦略を定義する。
- 並行実行制御: 同一リソースへの競合更新時の排他/整合ルールを定義する。
- ページング: List 系 API の件数上限、ページ境界、安定ソート条件を定義する。
- 監査項目の詳細仕様: 監査ログに残す項目(request id, actor, resource id など)を定義する。

## 設計原則: ステートレスドライバとクラウドネイティブな状態保持

### 原則

プロバイダドライバは**ローカルのステートストアを持たない**。クラウドで動的に生成されるステートや設定（Managed Identity の Principal ID、クラスタの OIDC Issuer URL など）は、**クラウドリソース自体に self-contained で保持**し、必要時に API で取得する。

ユーザーにステートファイルやバックエンドストレージの設定・管理・バックアップを要求しない。

### 背景

Terraform などの IaC ツールはステートファイルに依存し、ユーザーにリモートバックエンドの設定（S3 + DynamoDB、Azure Blob Storage など）やステートのロック・バックアップ管理を求める。Kompox ではこの運用負荷を排除する。

### 実現方法

各プロバイダドライバは、そのクラウドプラットフォームが提供するネイティブな仕組みを活用して非決定的な値を保持・取得する:

| パターン | 例 | 適用先 |
|---|---|---|
| リソースタグ / ラベル | RG タグ, EKS タグ, GKE ラベル, OCI Freeform Tags | 全クラウド共通の推奨方式 |
| リソース API への直接クエリ | GKE API で Cluster Identity を取得 | GCP |
| リソース識別子の決定的導出 | GCP SA メールアドレス (`name@project.iam.gserviceaccount.com`) | GCP |
| ローカルファイル | `/etc/rancher/k3s/k3s.yaml` | K3s |

### 推奨方式: リソースタグ / ラベル

クラウドが動的に生成する非決定的な値（Principal ID、OIDC Issuer URL 等）の保持には、**決定的に特定可能なリソースに付与するタグ / ラベル**を推奨する。すべての主要クラウドがこの仕組みを提供しており、IaC デプロイメント履歴などの副次的レコードに依存するよりもロバストである。

| クラウド | タグ付与先リソース | メタデータ機構 | 値の長さ上限 | 上限タグ数 |
|---|---|---|---|---|
| Azure | Resource Group | タグ (key-value) | 256 文字 | 50 個 |
| AWS | EKS Cluster / CloudFormation Stack | タグ (key-value) | 256 文字 | 50 個 |
| GCP | GKE Cluster / Project | ラベル (key-value) | 63 文字 | 64 個 |
| OCI | OKE Cluster / Compartment | Freeform Tags | 256 文字 | 制限緩い |
| K3s | — | 不要 (クラウドリソースなし) | — | — |

タグ付与先のリソースは、ドライバの命名規則から決定的に特定できるものを選ぶ（Azure なら Resource Group、AWS なら EKS Cluster 等）。

**設計上の特性**:

- タグ付与先リソースが存在する限りステートは不滅であり、IaC 履歴の自動削除や手動削除の影響を受けない
- タグの読み書きは単一の API 呼び出しで完結し、IaC テンプレートの再デプロイを必要としない
- UUID (36 文字) はすべてのクラウドの値の長さ上限に収まる
- GCP のラベルは値が 63 文字に制限されるが、GCP では識別子の多くが決定的に導出可能であり、ラベルに記録する必要性は低い

### 要件

- ドライバのメソッドはすべてステートレスであり、前回の実行結果をローカルに保存しない
- クラウドが動的に生成する値（UUID、ARN、URL 等）は、そのクラウドのリソースまたはデプロイメント記録から API 経由で取得する
- ユーザーが管理すべき状態は `kompoxops.yml`（宣言的定義）のみとする
- ドメイン層およびユースケース層はステートの保持方法に関知しない（ドライバ内部の実装詳細として閉じる）

## 設計原則: 決定的命名

### 原則

ドライバが管理するすべてのクラウドリソースの名前は、**ユーザーが定義した宣言的情報（Workspace 名、Provider 名、Cluster 名、App 名）から決定的に導出**される。乱数や自動生成名に依存しない。

### 背景

ステートレスドライバがクラウドリソースを「再発見」するには、リソース名を入力情報だけから再現できる必要がある。決定的命名はステートレス原則の前提条件であり、ステートファイルを持たないアーキテクチャを成立させる基盤である。

### 要件

- リソース名は Workspace / Provider / Cluster / App の名前とハッシュの組み合わせで構成する
- 同じ入力に対して常に同じ名前を返す（純粋関数）
- クラウドプラットフォームの命名制約（長さ上限、使用可能文字）に適合させるトランケーションとハッシュ付与を行う
- ハッシュはグローバル一意性を確保するために使用し、ヒューマンリーダブルなプレフィックスと組み合わせる
- 命名ロジックはドライバ内の専用モジュール（例: `naming.go`）に集約する

### 参考: AKS ドライバの命名パターン

| リソース | 命名パターン | 例 |
|---|---|---|
| Resource Group | `<prefix>_cls_<clusterName>` + ハッシュ | `k4x-ab12_cls_main` |
| Managed Disk | `<prefix>_<appHash>_<volName>_<diskName>` | `k4x-ab12_cd34_data_init` |
| Storage Account | `<prefix><appHash><volHash>` | `k4xab12cd34ef56` |

## 設計原則: ensure パターンによる冪等収束

### 原則

クラウドリソースの作成・更新操作は **`ensure*()` パターン**で実装する: 「あるべき状態を宣言し、現在の状態と収束させる」。これは要求事項の冪等性を実現する具体的な手法である。

### パターン

```
ensure<Resource><Action>(ctx, desiredState) error
  1. 現在の状態を取得（Get / List）
  2. 既に収束済みなら return nil（冪等）
  3. 不足があれば Create or Update
  4. 要求状態に達しない場合のみ error を返す
```

### 要件

- 命名規則: `ensure<Resource><Action>` （例: `ensureResourceGroupCreated`, `ensureRoleAssigned`, `ensureStorageAccountCreated`）
- 既存リソースが目標状態と一致する場合はスキップし、ログで記録する
- HTTP 409 Conflict / AlreadyExists は成功扱いとする
- Force オプションが指定された場合は、既存の成功状態でも再実行する

### ベストエフォート削除

削除操作の一部は**ベストエフォート**として実装する。エラーをログに記録するが、呼び出し元には返さない（戻り値が `void` または error を無視）。これにより、削除フロー全体が部分的な失敗で停止しない。

適用例:
- デプロイメントレコードの削除
- RBAC ロール割り当ての削除
- 論理削除された Key Vault のパージ

## 設計原則: タグベースのリソース所有権

### 原則

ドライバが作成するクラウドリソースには、**Kompox の論理的な所有関係を示すタグ（ラベル）を付与**する。タグは List 操作のフィルタリング、所有権の確認、ステートの記録に使用する。

### 要件

- すべてのクラウドリソースに所有権を示すタグを付与する
- タグキーは `kompox-` プレフィックスで名前空間を分離する
- 最低限の共通タグ: Workspace 名、Provider 名、`managed-by: kompox`
- リソースの粒度に応じて追加タグ（Cluster 名、App 名、Volume 名等）を付与する
- List 操作ではタグフィルタにより「このスコープに属するリソース」のみを列挙する
- リソースの論理的状態（例: ディスクの Assigned フラグ）もタグで管理してよい

### 参考: AKS ドライバのタグ体系

| タグキー | 粒度 | 用途 |
|---|---|---|
| `kompox-workspace-name` | 全リソース | Workspace への帰属 |
| `kompox-provider-name` | 全リソース | Provider への帰属 |
| `kompox-cluster-name` / `kompox-cluster-hash` | Cluster スコープ | Cluster への帰属と一意識別 |
| `kompox-app-name` / `kompox-app-id-hash` | App スコープ | App への帰属と一意識別 |
| `kompox-volume` / `kompox-disk-name` / `kompox-snapshot-name` | Volume スコープ | Volume / Disk / Snapshot の識別 |
| `kompox-disk-assigned` | Disk | Disk の Assign 状態 (`true`/`false`) |
| `managed-by` | 全リソース | `kompox` による管理を示す |

### ステート記録との関係

タグベースの所有権は「クラウドネイティブな状態保持」原則の具体的な実現手段でもある。所有権タグとステート記録タグを同じ仕組みで管理することで、リソースの帰属確認と非決定的な値の保持を統一的に実現する。

## レジストリと生成

- レジストリ: `/adapters/drivers/provider/registry.go` の `Register(name, factory)` を使用し、`init()` で自己登録。
- 生成: Usecase 側は `GetDriverFactory()` でファクトリを取得し、`factory(workspace, provider)` でドライバを生成する。

登録例(抜粋)
```go
func init() {
  providerdrv.Register("aks", func(workspace *model.Workspace, provider *model.Provider) (providerdrv.Driver, error) {
    // validate provider.Settings and workspace as needed, create credentials, return driver
    return &driver{/* ... */}, nil
  })
}
```

## 実装ガイドライン(メソッド別)

### ClusterProvision / ClusterDeprovision
- クラウド側リソースの作成/削除に限定(例: RG, Managed Cluster)。
- 入力検証: `cluster.Settings` の必須キーを先頭でチェック。エラーは具体的に。
- リトライ/バックオフ: SDK 標準のポーリング/リトライを活用。タグ付けで可観測性を向上。

### ClusterStatus
- プロビジョニング状態はクラウド SDK で取得。
- 必要に応じて `kube.Client` を使い in-cluster 確認(例: Ingress 用 Namespace の存在)。
- 認可/接続エラーは「未インストール判定を阻害しない」方針で扱いを検討。

### ClusterInstall / ClusterUninstall
- 共通処理は `adapters/kube` の `Installer` に委譲。
- 前処理/後処理のみプロバイダ固有で実装(例: IAM/CSI/LB 設定など)。
- 最小ステップ(例):
  1. `kubeconfig := d.ClusterKubeconfig(ctx, cluster)`
  2. `cli, _ := kube.NewClientFromKubeconfig(ctx, kubeconfig, &kube.Options{UserAgent: "kompoxops"})`
  3. `inst := kube.NewInstaller(cli)`
  4. インストール: `inst.EnsureIngressNamespace(ctx, cluster)` → `inst.ApplyYAML(ctx, manifests, kube.IngressNamespace(cluster))`
  5. アンインストール: マニフェスト削除(将来機能)→ `inst.DeleteIngressNamespace(ctx, cluster)`

### ClusterKubeconfig
- プロバイダ SDK で管理者/ユーザ資格情報を取得し、kubeconfig のバイト列を返す。
- 返却のみ(ファイル出力しない)。ドライバ外へはバイト配列で受け渡し。

### ClusterDNSApply
- プロバイダが管理する DNS ゾーンに DNS レコードセットを適用する。
- 冪等性を保証し、ベストエフォートで動作する(opts で厳密な処理を要求されない限り、回復可能なエラーは抑制する)。
- 不正な入力やコンテキストのキャンセルはエラーを返す。
- 実装例: Azure DNS Zone にレコードを書き込む、Route53 にレコードを設定するなど。
- DNSレコード管理は実際にデプロイされた状態(Kubernetes Ingress リソースから取得した FQDN と LoadBalancer IP)に基づいて行われます。
- `usecase/dns` 層が `kube.Client.IngressHosts()` を使用して実際のデプロイ状態を取得し、それに基づいてレコードセットを構築します。
- 詳細は [K4x-ADR-004] を参照してください。

### VolumeDiskList / VolumeDiskCreate / VolumeDiskAssign / VolumeDiskDelete

- 前提
  - ひとつの volume には複数の disks が所属する。
  - 新しい disk は空のディスク、既存の snapshot、または既存の disk から作成する(`source` で指定)。ただしバックエンドが対応している場合に限る。
  - 新しい snapshot は既存の特定の disk から作成できる(`VolumeSnapshotCreate` の `source` を使用)。
  - Name メンバはユーザーまたはドライバにより決定される。重複はエラーとする。
  - Size メンバはボリュームの指定値が使われる。
  - Handle メンバはクラウドディスクリソースの参照であり `volHASH` の生成に使われる。重複はエラーとする。
  - Assigned メンバが true の VolumeDisk が 1 個だけ存在するのが正常な状態。この VolumeDisk から Manifest が生成される。非正常状態で Manifest を生成しようとするとエラーになる。

- メソッド
  - VolumeDiskList は VolumeDisk の一覧を `CreatedAt` の降順で返す。同一時刻の場合は Name の昇順で安定化する。
  - VolumeDiskCreate は新規の VolumeDisk を作成する。diskName と source を受け取る。
    - diskName: 空文字列の場合、ドライバはデフォルトの命名規則を使用する。値が指定された場合、その名前を使用する(ユーザー指定名)。
    - source: 作成元を指定する。詳細は「Source パラメータの仕様」を参照。
    - 該当ボリュームで最初の1件のみ Assigned を true として作成し、それ以外は false とする(この操作は他ディスクの Assigned を変更しない)。
  - VolumeDiskAssign は指定した VolumeDisk の Assigned メンバを true として、それ以外のディスクの Assigned メンバを false とする。
  - VolumeDiskDelete は指定した VolumeDisk を削除する。`NotFound` は冪等性のため成功扱いとしてよい。

- ドライバ実装
  - Name/Handle の重複はエラー。`kompox-volName-idHASH` 等のタグで volume に所属する VolumeDisk を確実に識別する。
  - 個々の VolumeDisk の Name, Handle の決定方法や Assigned メンバの記録方法はドライバの実装に任される。
  - すべての外部呼び出しに `ctx` を伝播し、エラーは `%w` でラップする。
  - 同一のボリュームに属する VolumeDisk を識別するためのタグの値には `kompox-volName-idHASH` を使用する。これにより同一の VolumeDisk を維持したクラスタのフェイルオーバーが可能になる。

### VolumeSnapshotList / VolumeSnapshotCreate / VolumeSnapshotDelete

- 前提
  - ひとつの volume には複数の snapshots が所属する。
  - 新しい snapshot は既存の特定の disk、または既存の snapshot から作成する(`source` で指定)。ただしバックエンドが対応している場合に限る。
  - 新しい disk は特定の snapshot から作成できる(`VolumeDiskCreate` の `source` を使用)。
  - Name メンバはユーザーまたはドライバにより決定される。重複はエラーとする。
  - Handle メンバはクラウドスナップショットリソースの参照。重複はエラーとする。

- メソッド
  - VolumeSnapshotList は VolumeSnapshot の一覧を `CreatedAt` の降順で返す。同一時刻の場合は Name の昇順で安定化する。
  - VolumeSnapshotCreate は snapName と source を受け取る。
    - snapName: 空文字列の場合、ドライバはデフォルトの命名規則を使用する。値が指定された場合、その名前を使用する(ユーザー指定名)。
    - source: スナップショット元を指定する。詳細は「Source パラメータの仕様」を参照。
    - 該当ディスクのスナップショットを作成する。
  - VolumeSnapshotDelete は指定した VolumeSnapshot を削除する。`NotFound` は冪等性のため成功扱いとしてよい。

- ドライバ実装
  - Name/Handle の重複はエラー。`kompox-volName-idHASH` 等のタグで volume に所属する VolumeSnapshot を確実に識別する。
  - 個々の VolumeSnapshot の Name, Handle の決定方法はドライバの実装に任される。
  - すべての外部呼び出しに `ctx` を伝播し、エラーは `%w` でラップする。
  - 同一のボリュームに属する VolumeSnapshot を識別するためのタグの値には `kompox-volName-idHASH` を使用する。これにより同一の VolumeSnapshot を維持したクラスタのフェイルオーバーが可能になる。
  - スナップショット未対応のプロバイダは、未サポートを示す明確なエラー(例: ErrUnsupported)を返す。

### Source パラメータの仕様

`VolumeDiskCreate` と `VolumeSnapshotCreate` の `source` パラメータは作成元リソースを指定する不透明な文字列です。CLI/UseCase 層ではパース・検証を行わず、そのままドライバに渡します。ドライバ側で以下の規則に従って解釈します。詳細は [K4x-ADR-003] を参照してください。

- **フォーマット**: `[<type>:]<name>`
  - `<type>` には `disk` または `snapshot` を指定可能(予約語彙)。
  - `<type>` を省略した場合、コマンドによってデフォルトが決まる:
    - `disk create`: `snapshot:<name>` として解釈(スナップショットから復元)
    - `snapshot create`: `disk:<name>` として解釈(ディスクからスナップショット作成)
  - `<name>` は Kompox 管理のディスク名またはスナップショット名、またはプロバイダネイティブなリソース ID(例: Azure の `/subscriptions/.../...`)。

- **デフォルト動作**(`source` 省略時):
  - `disk create`: 空の新規ディスクを作成。
  - `snapshot create`: 現在 Assigned されているディスクから自動的にスナップショットを作成。

- **命名制約**(Kompox ベースライン):
  - ボリューム名: DNS-1123 ラベル、長さ 1..16
  - ディスク名: DNS-1123 ラベル、長さ 1..24
  - スナップショット名: DNS-1123 ラベル、長さ 1..24
  - DNS-1123 ラベルは小文字英数字とハイフン(-)のみで、先頭と末尾は英数字。
  - プロバイダドライバは、プラットフォーム固有の制約によりさらに厳しい制限を課すことがあります。

- **ドライバ実装ガイドライン**:
  - `source` の解釈・検証はドライバが担当。
  - プロバイダネイティブな Resource ID(例: `/subscriptions/...`)を検出し、リージョン一致やRBAC要件を検証。
  - エラーは具体的でアクション可能なメッセージを返す(例: "snapshot must be in the same region as the disk")。

### VolumeClass

- 目的: プロバイダ固有のボリュームクラス情報(StorageClassName, CSIDriver, AccessModes 等)を返す。
- 契約: 空フィールドは「ノーオピニオン」を表す。コール側(kube 層)はその項目をマニフェストに含めない。ドライバ側でプロバイダ固有のデフォルト値を設定してはならない。
- 推奨: ドライバは既定値の埋め込みを避け、クラスタ/アプリ/ボリューム設定から決まる最小限のみを返す。

### NodePoolList / NodePoolCreate / NodePoolUpdate / NodePoolDelete

- 前提
  - Kompox は `NodePool` をプロバイダ横断の共通語として扱う。ベンダ固有用語(AKS の Agent Pool、EKS の Node Group など)はドライバ実装側で吸収する。
  - Pod スケジューリングは Kompox ラベル(`kompox.dev/node-pool`, `kompox.dev/node-zone`)を一次契約とする。
  - zone 値の正規化・変換は provider driver の責務とし、Converter 側は入力意図の反映に専念する。
  - DTO は単一の `NodePool` を使用し、Create/Update/List の全メソッドで共通化する。

- NodeSelector ラベル規約 (共通語彙/値/フォーマット)
  - `kompox.dev/node-pool`
    - 意味: スケジューリング対象 NodePool の論理名。
    - 共通語彙: `system` / `user` を予約語彙として扱う。その他の値はユーザ定義 NodePool 名として扱う。
    - フォーマット: Kubernetes Label Value 互換の非空文字列、長さ 1..63 を推奨する。
  - `kompox.dev/node-zone`
    - 意味: スケジューリング対象ゾーンの論理識別子。
    - 共通語彙 (推奨): `<region>-<zoneIndex>` 形式 (例: `japaneast-1`, `eastus2-3`)。
    - 互換語彙: プロバイダ固有値 (例: AKS の `"1"`, `"2"`, `"3"`) も許容する。
    - フォーマット: Kubernetes Label Value 互換の非空文字列、長さ 1..63 を推奨する。
  - 正規化責務
    - converter は値を透過し、語彙変換を行わない。
    - provider driver はクラスタ実体(Node/NodePool)側ラベルとの整合を維持するため、必要に応じてプロバイダ API 値との相互変換を実装する。

- メソッド
  - NodePoolList は NodePool の一覧を返す。オプションでフィルタリング(名前など)をサポートする。
  - NodePoolCreate は新しい NodePool を作成する。`pool` パラメータで構成を指定し、必須フィールドのバリデーションを行う。
  - NodePoolUpdate は既存 NodePool の可変フィールドを更新する。`pool` の non-nil ポインタフィールドのみを適用対象とし、不変項目の変更を試みた場合は validation error を返す。
  - NodePoolDelete は指定した NodePool を削除する。`NotFound` は冪等性のため成功扱いとしてよい。

- DTO 概要(MVP)
  - `NodePool` の主要フィールドは pointer を基本とし、`Update` では non-nil のみを適用対象とする。
  - 主要フィールド例: `Name *string`, `ProviderName *string`, `Mode *string` (`system`/`user`), `Labels *map[string]string`, `Zones *[]string`, `InstanceType *string`, `OSDiskType *string`, `OSDiskSizeGiB *int`, `Priority *string` (`regular`/`spot`), `Autoscaling *NodePoolAutoscaling`, `Status *NodePoolStatus`, `Extensions map[string]any`
  - `NodePoolAutoscaling` 例: `Enabled bool`, `Min int`, `Max int`, `Desired *int`
  - ベンダ方言のパラメータ名は DTO へ持ち込まず、driver 側で変換する(例: AKS `vmSize` は Kompox `InstanceType` にマッピング)。

- ドライバ実装
  - Create の必須項目はメソッド側バリデーションで強制する。
  - Update で immutable 項目が指定された場合は validation error とする。実装がどのフィールドを mutable と扱うかはドライバに委ねられる。
  - プロバイダが機能自体を持たない場合は `not implemented` エラーを返す(詳細は「エラーモデル」セクションを参照)。
  - すべての外部呼び出しに `ctx` を伝播し、エラーは `%w` でラップする。
  - 冪等性を保証する。

## エラーモデル

Provider Driver のエラー処理は、機能の未対応と不正な入力を明確に区別します。

- **Not Implemented (機能未対応)**
  - プロバイダが機能自体を持たない場合に返すエラー。
  - 例: NodePool 管理に対応していないプロバイダで NodePoolCreate を呼び出した場合。
  - Usecase/CLI 層はこれを capability boundary として扱い、transient failure として再試行しない。
  - 実装: 専用の `ErrNotImplemented` または類似のエラー型を返す。

- **Validation Error (検証エラー)**
  - プロバイダは機能を持つが、入力パラメータが不正または不可変項目を変更しようとした場合。
  - 例: NodePoolUpdate で immutable なフィールド(InstanceType など)を変更しようとした場合。
  - 例: 必須フィールドが欠けている、値が制約を満たさないなど。
  - Usecase/CLI 層は具体的なエラーメッセージをユーザに返し、入力の修正を促す。

- **原則**
  - エラーメッセージは具体的でアクション可能な内容とする。
  - エラーは `fmt.Errorf("...: %w", err)` でラップし、可観測性を保つ。

## `adapters/kube` の利用例(ドライバ側)

```go
kc, err := d.ClusterKubeconfig(ctx, cluster)
if err != nil { return fmt.Errorf("get kubeconfig: %w", err) }
cli, err := kube.NewClientFromKubeconfig(ctx, kc, &kube.Options{UserAgent: "kompoxops"})
if err != nil { return fmt.Errorf("new kube client: %w", err) }
inst := kube.NewInstaller(cli)
if err := inst.EnsureIngressNamespace(ctx, cluster); err != nil {
    return fmt.Errorf("ensure namespace: %w", err)
}
// apply manifests if needed
// _ = inst.ApplyYAML(ctx, []byte(manifestYAML), kube.IngressNamespace(cluster))
```

## 設定キーとバリデーション(例)

- 例: AKS ドライバ
  - Provider settings(作成時必須): `AZURE_SUBSCRIPTION_ID`, `AZURE_LOCATION`, `AZURE_AUTH_METHOD` など。
  - Cluster settings(クラスタ単位): `AZURE_RESOURCE_GROUP_NAME`。
- 方針: 必須キーは定数化し、`missing: key1, key2` の形式で明確に報告。

## テスト方針

- Unit: 入力検証、分岐、SDK 呼び出しのモック化テスト。
- Kube: `adapters/kube` は client-go の fake/dynamic を利用。ドライバ側は薄く利用。
- E2E(End-to-End): `/tests/aks-e2e-*` ディレクトリに実クラウド環境での統合テストを配置。
  - 各テストディレクトリには `Makefile` と一連のシェルスクリプト(`test-setup.sh`, `test-run.sh`, `test-teardown.sh`, `test-clean.sh`)が含まれる。
  - `make all` でセットアップ、実行、クリーンアップの全フローが自動化される。
  - テスト例:
    - `aks-e2e-basic`: 基本的なクラスタプロビジョニング、アプリデプロイ、Ingress 疎通確認。
    - `aks-e2e-volume`: ボリューム操作(disk create/assign/delete, snapshot create/delete/restore)の包括的検証。
    - `aks-e2e-gitea`, `aks-e2e-gitlab`, `aks-e2e-redmine`: 実アプリケーション(Compose ベース)のデプロイと動作確認。
  - 認証情報やサブスクリプション ID などは `test.env` および `test-local.env`(非コミット)で管理。
  - CI/CD では実行コスト・時間の都合上、デフォルトではスキップし、必要時に手動トリガーまたは定期実行する方針を推奨。

## 実装チェックリスト

- [ ] `ID()` が一意の識別子を返す
- [ ] `Register()` による自己登録
- [ ] Provider/Cluster 両方の settings を検証
- [ ] `ClusterKubeconfig()` がバイト列で返す(ファイルに書かない)
- [ ] `ClusterInstall/Uninstall` は `kube.Installer` を使用
- [ ] `ClusterProvision/Deprovision/Install/Uninstall` は可変オプション引数(Force 等)を受け取り尊重
- [ ] `ClusterDNSApply` は冪等性を保証し、ベストエフォートで動作する
- [ ] Volume の各メソッド(Disk/Snapshot 系)も可変オプション引数を受け取り、将来の拡張(Force/DryRun 等)に備える(未使用でも受理)
- [ ] `VolumeClass()` を実装し、不要なフィールドは空で返す(プロバイダ固有のデフォルト値を設定しない)
- [ ] Snapshot の 3 メソッド(List/Create/Delete)を実装し、前提と契約(NotFound冪等、タグ識別)を満たす
- [ ] VolumeDiskCreate は最初の1件を Assigned=true で作成し、それ以外は false。diskName と source を受け取る(空文字列はデフォルト/新規作成、source は「Source パラメータの仕様」に従う)
- [ ] VolumeSnapshotCreate は snapName と source を受け取る(source は「Source パラメータの仕様」に従う)
- [ ] diskName と snapName は、ユーザーが指定した場合はその名前を使用し、空文字列の場合はドライバがデフォルト命名規則を適用する
- [ ] List は CreatedAt 降順(同時刻は Name 昇順)
- [ ] NodePool の 4 メソッド(List/Create/Update/Delete)を実装し、契約を満たす(未対応プロバイダは `not implemented` を返す)
- [ ] NodePoolUpdate は non-nil ポインタフィールドのみを適用し、immutable 項目の変更は validation error とする
- [ ] NodePool DTO はベンダ中立な命名(InstanceType, Priority など)を使用し、ドライバ側でマッピングする
- [ ] 外部コマンドを使用しない(kubectl/helm 禁止)
- [ ] ログと UserAgent を設定、シークレットはマスク
- [ ] 冪等性とコンテキストキャンセルに対応

## 参照

### 関連 ADR

- [K4x-ADR-002] - Unify snapshot restore into disk create
  スナップショット復元を `disk create` に統合し、単一の `-S/--source` オプションで一貫した UX を実現。
- [K4x-ADR-003] - Unify Disk/Snapshot CLI flags and adopt opaque Source contract
  Disk/Snapshot コマンドのフラグを統一(`-N/--name`, `-S/--source`)し、`source` パラメータを不透明な文字列として扱う契約を採用。CLI/UseCase 層ではパースせず、ドライバに解釈を委ねる。
- [K4x-ADR-004] - Cluster ingress endpoint DNS auto-update
  Ingress エンドポイントの DNS レコード自動更新機能。実際にデプロイされた Kubernetes Ingress リソースの状態に基づいて DNS を管理する。`ClusterDNSApply` の設計根拠と動作仕様を定義。
- [K4x-ADR-019] - Introduce NodePool abstraction for multi-provider cluster scaling and scheduling
  Kompox における NodePool 抽象の導入決定。プロバイダ横断の共通語として `NodePool` を採用し、ライフサイクル管理メソッドの契約レベルの方針を定義。

### 関連ドキュメント

- [Kompox-Arch-Implementation] - アーキテクチャガイダンス
- [Kompox-CLI] - CLI 仕様
- [Kompox-ProviderDriver-AKS] - AKS 固有の実装ガイド
- [Kompox-Logging] - ロギング仕様

[K4x-ADR-002]: ../adr/K4x-ADR-002.md
[K4x-ADR-003]: ../adr/K4x-ADR-003.md
[K4x-ADR-004]: ../adr/K4x-ADR-004.md
[K4x-ADR-019]: ../adr/K4x-ADR-019.md
[Kompox-Arch-Implementation]: ./Kompox-Arch-Implementation.ja.md
[Kompox-CLI]: ./Kompox-CLI.ja.md
[Kompox-ProviderDriver-AKS]: ./Kompox-ProviderDriver-AKS.ja.md
[Kompox-Logging]: ./Kompox-Logging.ja.md
