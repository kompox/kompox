# Kompox Kube Client ガイド v1

## 概要

本書は Kompox の Kubernetes クライアント `kube.Client`（`adapters/kube`）の設計と公開契約を解説します。Helm/YAML 適用や Ingress の拡張ポイントを中心に、現行実装の振る舞いを明確にします。

- 本書で扱う主な事項:
  - Ingress(Traefik) のインストール/アンインストールと既定値
  - `HelmValuesMutator` による Helm values 拡張
  - `additionalConfigFiles` を用いた Traefik File Provider 連携

## 目的と範囲

- 目的: Kubernetes への標準的な操作を抽象化し、プロバイダドライバから安全かつ冪等に利用できる API を提供する。
- 非対象: クラウドプロバイダ固有のリソース操作はプロバイダドライバ `providerdrv` で実装。

## 主要 API

- Client 構築・基本

```go
// クライアント本体
type Client struct {
  RESTConfig *rest.Config
  Clientset  kubernetes.Interface
}

// 構築オプション
type Options struct {
  UserAgent string
  QPS       float32
  Burst     int
}

// 構築関数
func NewClientFromKubeconfig(ctx context.Context, kubeconfig []byte, opts *Options) (*Client, error)
func NewClientFromKubeconfigPath(ctx context.Context, path string, opts *Options) (*Client, error)
func NewClientFromRESTConfig(cfg *rest.Config, opts *Options) (*Client, error)

// 付帯: 保持している kubeconfig の取得（保持していない場合は nil）
func (c *Client) Kubeconfig() []byte
```

- Namespace 管理

```go
func (c *Client) CreateNamespace(ctx context.Context, name string) error
func (c *Client) DeleteNamespace(ctx context.Context, name string) error
```

- YAML/オブジェクト適用（Server-Side Apply）

```go
// 適用オプション
type ApplyOptions struct {
  DefaultNamespace string
  FieldManager     string
  ForceConflicts   bool
}

func (c *Client) ApplyYAML(ctx context.Context, data []byte, opts *ApplyOptions) error
func (c *Client) ApplyObjects(ctx context.Context, objs []runtime.Object, opts *ApplyOptions) error
```

- Traefik Helm Chart 管理

```go
// Helm values 型とミューテータ
type HelmValues map[string]any
type HelmValuesMutator func(ctx context.Context, cluster *model.Cluster, release string, values HelmValues)

func (c *Client) InstallIngressTraefik(ctx context.Context, cluster *model.Cluster, mutators ...HelmValuesMutator) error
func (c *Client) UninstallIngressTraefik(ctx context.Context, cluster *model.Cluster) error
```

## Traefik Helm Chart

### InstallIngressTraefik()

- 目的: 最小構成の Traefik Ingress Controller を Ingress Namespace にインストール/アップグレード（冪等）。
- 実装の要点:
  - Helm SDK を使用（外部コマンド不使用）。一時 kubeconfig を生成して接続。
  - Namespace は `IngressNamespace(cluster)` を事前に作成。
  - 既存リリースがない場合は Upgrade→Install の順に実行。
  - チャート: `https://helm.traefik.io/traefik`、リリース名: `TraefikReleaseName`。
- 既定の Helm values（抜粋）:
  - `service.type = LoadBalancer`
  - `updateStrategy.type = Recreate`
  - `serviceAccount.name = IngressServiceAccountName(cluster)`（Helm で作成しない）
  - `persistence.enabled = true`（`/data` に ACME ストレージを保持）
  - `logs.access.enabled = true`
  - `podSecurityContext.fsGroup = 65532`, `fsGroupChangePolicy = OnRootMismatch`
  - `deployment.podLabels.azure.workload.identity/use = true`
  - `additionalArguments` で ACME の production/staging を有効化し、File Provider を `/config/traefik` で監視
- File Provider 連携:
  - `additionalConfigFiles`（後述）に与えられたファイル群を ConfigMap `traefik` の `data` に転写、`/config/traefik` へ読み取り専用マウント。
  - ConfigMap を適用後、Helm values から `additionalConfigFiles` を削除（チャートへは渡さない）。
- ログ: 生成 values と ConfigMap data を YAML で Debug 出力。
- アンインストール: `UninstallIngressTraefik()` は未存在時も成功扱いで冪等。

### HelmValuesMutator

- 役割: プロバイダが Helm values をリリース単位で上書き/追記するためのフック。
- 呼び出し: `InstallIngressTraefik()` が既定値を構築した直後、Helm 実行前に順次適用。
- ベストプラクティス:
  - 同一入力で決定的に。
  - `values` 内で完結（外部 I/O を避ける）。
  - ネストの存在確認（例: `deployment` マップの生成）を行ってから編集。
- values 設定の代表例:
  - `deployment.additionalVolumes` への CSI ボリューム追加
  - `additionalVolumeMounts` へのマウント追記
  - `additionalArguments` の追記
  - `additionalConfigFiles` の注入

### values: additionalConfigFiles

- 型: `map[string]string`
  - キー: ファイル名（例: `certs.yaml`, `middlewares.yaml`）
  - 値: ファイル本文（テキスト、通常は Traefik の動的設定 YAML）
- 挙動:
  - ConfigMap `traefik` の `data` に転写され、Pod に `/config/traefik` としてマウント。
  - File Provider の監視により動的に反映（`--providers.file.directory` と `--providers.file.watch` が既定有効）。
  - Helm 実行前に `values` から削除され、チャートには渡らない。
- 注意点:
  - 同一ファイル名は最後の値が有効。
  - ConfigMap のサイズ上限に留意（大きな設定は分割/他手段の検討）。
- サンプル（TLS 設定ファイルの注入）
```yaml
# certs.yaml
tls:
  certificates:
    - certFile: /var/run/secrets/traefik/tls/example.crt
      keyFile:  /var/run/secrets/traefik/tls/example.key
      stores:
        - default
```

### AKS ドライバでの利用例（Key Vault 連携）

- 目的: Key Vault の証明書を CSI Secrets Store で Pod にマウントし、Traefik に `additionalConfigFiles` 経由で `tls.certificates` を教える。
- 手順（概略）:
  1. `SecretProviderClass` を作成し、必要な CSI ボリューム/マウント情報と証明書リスト（`configs`）を生成。
  2. Mutator で `deployment.additionalVolumes` と `additionalVolumeMounts` を追記。
  3. `configs` を `tls.certificates` 形式の YAML に Marshal し、`certs.yaml` を `additionalConfigFiles` に格納。
- 実装の要所:
  - `deployment.podLabels.azure.workload.identity/use = true` は kube 側で既定付与。
  - `cluster.Ingress.CertEmail` が ACME 用に利用されるため、運用時は必ず設定。

---

最終更新: v1（初版）
