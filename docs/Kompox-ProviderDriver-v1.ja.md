# Kompox Provider Driver ガイド v1

## 概要

本書は Kompox のクラウドプロバイダ用ドライバ（以下、プロバイダドライバ）の設計と公開契約を解説します。usecase はオーケストレーション、adapters は I/O 実装という責務分離に基づきます。

## 目的と範囲

- 目的: クラウドプロバイダ依存の操作（プロビジョニング/認証/前後処理）を担う。
- 非対象: Kubernetes API の共通操作（Namespace 作成、マニフェスト適用、待機など）は `adapters/kube` に委譲する。

## 配置と命名

- ディレクトリ: `/adapters/drivers/provider/`
- パッケージ名: `providerdrv`
- 各プロバイダの配置: `/adapters/drivers/provider/<id>/`（例: `aks/`, `k3s/`）
- 依存関係の原則: `api(cmd) → usecase → domain ← adapters(drivers, store, kube)`
  - adapters は domain に依存してよいが、usecase には依存しない。
  - usecase は adapters の抽象（ポート/ドライバ）を経由して操作を指示する。

## 公開契約（Driver インターフェース）

> 実体は `/adapters/drivers/provider/registry.go` を参照。

```go
// Driver abstracts provider-specific behavior (identifier, hooks, etc.).
// Implementations live under adapters/drivers/provider/<name> and should return a
// provider driver identifier such as "aks" via ID().
type Driver interface {
    // ID returns the provider driver identifier (e.g., "aks").
    ID() string

    // ServiceName returns the service name associated with this driver instance.
    // May return "(nil)" if no service is associated (e.g., for testing).
    ServiceName() string

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

    // VolumeInstanceList returns a list of volume instances of the specified volume.
    VolumeInstanceList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, opts ...model.VolumeInstanceListOption) ([]*model.VolumeInstance, error)

    // VolumeInstanceCreate creates a volume instance of the specified volume.
    VolumeInstanceCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, opts ...model.VolumeInstanceCreateOption) (*model.VolumeInstance, error)

    // VolumeInstanceAssign assigns a volume instance to the specified volume.
    VolumeInstanceAssign(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, volInstName string, opts ...model.VolumeInstanceAssignOption) error

    // VolumeInstanceDelete deletes a volume instance of the specified volume.
    VolumeInstanceDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, volInstName string, opts ...model.VolumeInstanceDeleteOption) error

    // VolumeSnapshotList returns a list of snapshots of the specified volume.
    VolumeSnapshotList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, opts ...model.VolumeSnapshotListOption) ([]*model.VolumeSnapshot, error)

    // VolumeSnapshotCreate creates a snapshot from the specified volume instance.
    VolumeSnapshotCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, volInstName string, opts ...model.VolumeSnapshotCreateOption) (*model.VolumeSnapshot, error)

    // VolumeSnapshotDelete deletes the specified snapshot.
    VolumeSnapshotDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, opts ...model.VolumeSnapshotDeleteOption) error

    // VolumeSnapshotRestore creates a new volume instance from the specified snapshot.
    // The returned instance should have Assigned=false; switching is handled by VolumeInstanceAssign.
    VolumeSnapshotRestore(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, opts ...model.VolumeSnapshotRestoreOption) (*model.VolumeInstance, error)

    // VolumeClass returns provider specific volume provisioning parameters for the given logical volume.
    // Empty fields mean "no opinion" and callers should omit them from manifests.
    VolumeClass(ctx context.Context, cluster *model.Cluster, app *model.App, vol model.AppVolume) (model.VolumeClass, error)
}
```

要求事項（横断）
- 冪等性: 同じ入力で複数回実行しても安全。`NotFound` は必要に応じて成功扱い。
- コンテキスト: `ctx` をすべての外部呼び出しに伝播。タイムアウトは上位層が制御し、ドライバ側は尊重。
- エラー: `fmt.Errorf("...: %w", err)` で原因をラップし可観測性を保つ。
- セキュリティ: kubeconfig/証明書/トークンはディスクに書かない（バイト列のまま扱う）。
- 外部コマンド禁止: `kubectl`/`helm` などの shell-out は避け、Go SDK / client-go を利用。
- ログ/UA: 構造化ログ。`UserAgent` は `kompoxops/<module>` を付与。

## レジストリと生成

- レジストリ: `/adapters/drivers/provider/registry.go` の `Register(name, factory)` を使用し、`init()` で自己登録。
- 生成: Usecase 側は `GetDriverFactory()` でファクトリを取得し、`factory(service, provider)` でドライバを生成する。

登録例（抜粋）
```go
func init() {
  providerdrv.Register("aks", func(service *model.Service, provider *model.Provider) (providerdrv.Driver, error) {
    // validate provider.Settings and service as needed, create credentials, return driver
    return &driver{/* ... */}, nil
  })
}
```

## 実装ガイドライン（メソッド別）

### ClusterProvision / ClusterDeprovision
- クラウド側リソースの作成/削除に限定（例: RG, Managed Cluster）。
- 入力検証: `cluster.Settings` の必須キーを先頭でチェック。エラーは具体的に。
- リトライ/バックオフ: SDK 標準のポーリング/リトライを活用。タグ付けで可観測性を向上。

### ClusterStatus
- プロビジョニング状態はクラウド SDK で取得。
- 必要に応じて `kube.Client` を使い in-cluster 確認（例: Ingress 用 Namespace の存在）。
- 認可/接続エラーは「未インストール判定を阻害しない」方針で扱いを検討。

### ClusterInstall / ClusterUninstall
- 共通処理は `adapters/kube` の `Installer` に委譲。
- 前処理/後処理のみプロバイダ固有で実装（例: IAM/CSI/LB 設定など）。
- 最小ステップ（例）:
  1. `kubeconfig := d.ClusterKubeconfig(ctx, cluster)`
  2. `cli, _ := kube.NewClientFromKubeconfig(ctx, kubeconfig, &kube.Options{UserAgent: "kompoxops"})`
  3. `inst := kube.NewInstaller(cli)`
  4. インストール: `inst.EnsureIngressNamespace(ctx, cluster)` → `inst.ApplyYAML(ctx, manifests, kube.IngressNamespace(cluster))`
  5. アンインストール: マニフェスト削除（将来機能）→ `inst.DeleteIngressNamespace(ctx, cluster)`

### ClusterKubeconfig
- プロバイダ SDK で管理者/ユーザ資格情報を取得し、kubeconfig のバイト列を返す。
- 返却のみ（ファイル出力しない）。ドライバ外へはバイト配列で受け渡し。

### VolumeInstanceList / VolumeInstanceCreate / VolumeInstanceAssign / VolumeInstanceDelete

- app.volumes で定義された各ボリュームに対する操作。
- app.volumes で定義された各ボリュームにつき、
  - 複数の VolumeInstance が存在する。
  - Name メンバはドライバにより決定される。重複はエラーとする。
  - Size メンバはボリュームの指定値が使われる。
  - Handle メンバはクラウドディスクリソースの参照であり `volHASH` の生成に使われる。重複はエラーとする。
  - Assigned メンバが true の VolumeInstance が 1 個だけ存在するのが正常な状態。この VolumeInstance から Manifest が生成される。非正常状態で Manifest を生成しようとするとエラーになる。
- メソッド
  - VolumeInstanceList は VolumeInstance の一覧を CreatedAt メンバの降順で取得する。同一時刻の場合は Name の昇順で安定化する。
  - VolumeInstanceCreate は新規の VolumeInstance を作成する。該当ボリュームで最初の1件のみ Assigned を true として作成し、それ以外は false とする（この操作は他インスタンスの Assigned を変更しない）。
  - VolumeInstanceAssign は指定した VolumeInstance の Assigned メンバを true として、それ以外のインスタンスの Assigned メンバを false とする。
  - VolumeInstanceDelete は指定した VolumeInstance を削除する。
- ドライバ実装
  - 個々の VolumeInstance の Name, Handle の決定方法や Assigned メンバの記録方法はドライバの実装に任される。
  - 同一のボリュームに属する VolumeInstance を識別するためのタグの値には `kompox-volName-idHASH` を使用する。これにより同一の VolumeInstance を維持したクラスタのフェイルオーバーが可能になる。

### VolumeSnapshotList / VolumeSnapshotCreate / VolumeSnapshotDelete / VolumeSnapshotRestore

- 前提
  - ひとつの volume には複数の snapshots が所属する。
  - 新しい snapshot は既存の特定の instance からのみ作成できる（`volInstName` で指定）。
  - 新しい instance は特定の snapshot から作成できる（`VolumeSnapshotRestore`）。

- メソッド
  - VolumeSnapshotList は Snapshot の一覧を `CreatedAt` の降順で返す。同一時刻の場合は Name の昇順で安定化する。
  - VolumeSnapshotCreate は `volInstName` を必須とし、該当インスタンスのスナップショットを作成する。
  - VolumeSnapshotDelete は指定した Snapshot を削除する。`NotFound` は冪等性のため成功扱いとしてよい。
  - VolumeSnapshotRestore は指定した Snapshot から新規の VolumeInstance を作成して返す。返値の `Assigned` は `false` とし、切替は `VolumeInstanceAssign` に委ねる。

- ドライバ実装
  - Name/Handle の重複はエラー。`kompox-volName-idHASH` 等のタグで volume に所属する Snapshot を確実に識別する。
  - プロバイダが Kubernetes External Snapshotter を利用できる場合はそれを優先。利用できない場合はクラウドネイティブ API（例: ディスクスナップショット）で実装する。
  - すべての外部呼び出しに `ctx` を伝播し、エラーは `%w` でラップする。
  - スナップショット未対応のプロバイダは、未サポートを示す明確なエラー（例: ErrUnsupported）を返す。

### VolumeClass

- 目的: プロバイダ固有のボリュームクラス情報（StorageClassName, CSIDriver, AccessModes 等）を返す。
- 契約: 空フィールドは「ノーオピニオン」を表す。コール側（kube 層）はその項目をマニフェストに含めない。
- 推奨: ドライバは既定値の埋め込みを避け、クラスタ/アプリ/ボリューム設定から決まる最小限のみを返す。

## `adapters/kube` の利用例（ドライバ側）

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

## 設定キーとバリデーション（例）

- 例: AKS ドライバ
  - Provider settings（作成時必須）: `AZURE_SUBSCRIPTION_ID`, `AZURE_LOCATION`, `AZURE_AUTH_METHOD` など。
  - Cluster settings（クラスタ単位）: `AZURE_RESOURCE_GROUP_NAME`。
- 方針: 必須キーは定数化し、`missing: key1, key2` の形式で明確に報告。

## テスト方針

- Unit: 入力検証、分岐、SDK 呼び出しのモック化テスト。
- Kube: `adapters/kube` は client-go の fake/dynamic を利用。ドライバ側は薄く利用。
- Integration（任意）: 実クラウドへのスモークはオプション。CI ではスキップ可能に。

## 実装チェックリスト

- [ ] `ID()` が一意の識別子を返す
- [ ] `Register()` による自己登録
- [ ] Provider/Cluster 両方の settings を検証
- [ ] `ClusterKubeconfig()` がバイト列で返す（ファイルに書かない）
- [ ] `ClusterInstall/Uninstall` は `kube.Installer` を使用
- [ ] `ClusterProvision/Deprovision/Install/Uninstall` は可変オプション引数（Force 等）を受け取り尊重
- [ ] Volume の各メソッド（Instance/Snapshot 系）も可変オプション引数を受け取り、将来の拡張（Force/DryRun 等）に備える（未使用でも受理）
- [ ] `VolumeClass()` を実装し、不要なフィールドは空で返す
- [ ] Snapshot の 4 メソッド（List/Create/Delete/Restore）を実装し、前提と契約（Assigned=false, NotFound冪等, タグ識別）を満たす
- [ ] VolumeInstanceCreate は最初の1件を Assigned=true で作成し、それ以外は false。List は CreatedAt 降順（同時刻は Name 昇順）
- [ ] 外部コマンドを使用しない（kubectl/helm 禁止）
- [ ] ログと UserAgent を設定、シークレットはマスク
- [ ] 冪等性とコンテキストキャンセルに対応

---

最終更新: v1（初版）

注記: Ingress コントローラ（Traefik）のインストールや Helm values の拡張ポイントの詳細は kube クライアントのガイドに移動しました。`docs/Kompox-KubeClient-v1.ja.md` を参照してください。
