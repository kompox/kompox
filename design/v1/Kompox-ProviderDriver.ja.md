---
id: Kompox-ProviderDriver
title: Kompox Provider Driver ガイド
version: v1
status: synced
updated: 2025-10-08
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
}
```

要求事項(横断)
- 冪等性: 同じ入力で複数回実行しても安全。`NotFound` は必要に応じて成功扱い。
- コンテキスト: `ctx` をすべての外部呼び出しに伝播。タイムアウトは上位層が制御し、ドライバ側は尊重。
- エラー: `fmt.Errorf("...: %w", err)` で原因をラップし可観測性を保つ。
- セキュリティ: kubeconfig/証明書/トークンはディスクに書かない(バイト列のまま扱う)。
- 外部コマンド禁止: `kubectl`/`helm` などの shell-out は避け、Go SDK / client-go を利用。
- ログ/UA: 構造化ログ。`UserAgent` は `kompoxops/<module>` を付与。

## レジストリと生成

- レジストリ: `/adapters/drivers/provider/registry.go` の `Register(name, factory)` を使用し、`init()` で自己登録。
- 生成: Usecase 側は `GetDriverFactory()` でファクトリを取得し、`factory(service, provider)` でドライバを生成する。

登録例(抜粋)
```go
func init() {
  providerdrv.Register("aks", func(service *model.Service, provider *model.Provider) (providerdrv.Driver, error) {
    // validate provider.Settings and service as needed, create credentials, return driver
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
- [ ] 外部コマンドを使用しない(kubectl/helm 禁止)
- [ ] ログと UserAgent を設定、シークレットはマスク
- [ ] 冪等性とコンテキストキャンセルに対応

## リファレンス

### 関連 ADR(Architecture Decision Records)

- [K4x-ADR-002]: Unify snapshot restore into disk create
  スナップショット復元を `disk create` に統合し、単一の `-S/--source` オプションで一貫した UX を実現。

- [K4x-ADR-003]: Unify Disk/Snapshot CLI flags and adopt opaque Source contract
  Disk/Snapshot コマンドのフラグを統一(`-N/--name`, `-S/--source`)し、`source` パラメータを不透明な文字列として扱う契約を採用。CLI/UseCase 層ではパースせず、ドライバに解釈を委ねる。

- [K4x-ADR-004]: Cluster ingress endpoint DNS auto-update
  Ingress エンドポイントの DNS レコード自動更新機能。実際にデプロイされた Kubernetes Ingress リソースの状態に基づいて DNS を管理する。`ClusterDNSApply` の設計根拠と動作仕様を定義。

### 関連ドキュメント

- `design/v1/Kompox-Spec-Draft.ja.md`: プロジェクト概要と目標
- `design/v1/Kompox-Arch-Implementation.ja.md`: アーキテクチャガイダンス
- `design/v1/Kompox-CLI.ja.md`: CLI 仕様
- `_dev/tasks/`: 実装タスクとタスクガイド

[K4x-ADR-002]: ../adr/K4x-ADR-002.md
[K4x-ADR-003]: ../adr/K4x-ADR-003.md
[K4x-ADR-004]: ../adr/K4x-ADR-004.md
