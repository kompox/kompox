# Kompox PaaS Architecture

## Package / Directory Hierarchy

```
/cmd/
  kompoxops/            (main for CLI: wiring + DI)

/api/
  server.go             (HTTP server bootstrap)
  router.go             (mux/router setup)
  handlers/
    app_handler.go
    cluster_handler.go
    provider_handler.go
    service_handler.go

/usecase/               (ユースケース層。I/O 依存なし)
  /service/
    list.go
    get.go
    create.go
    update.go
    delete.go
  /cluster/
    create.go
    provision.go
  /provider/
    create.go
    validate.go
  /app/
    register.go
  errors.go             (ユースケース層エラーマッピング)

/domain/                (ドメイン層。ビジネスロジック)
  /model/               (ドメインモデル)
  repository.go         (interfaces: AppRepository, ClusterRepository, ProviderRepository, ServiceRepository, UnitOfWork)
  errors.go             (domain errors)

/adapters/
  /kube/
    runtime.go          (Kubernetes client runtime implementing cluster.Runtime if必要)
  /drivers/
    /provider/          (package providerdrv)
      registry.go
      types.go
      /aks/
      /k3s/
    /ingress/           (package ingressdrv)
      registry.go
      types.go
      /traefik/
      /nginx/
  /store/               (persistent storage)
    /inmem/             (in-memory)
      service.go
      provider.go
      cluster.go
      app.go
    /rdb/               (gorm)
      uow.go            (UnitOfWork + Tx)
      service.go
      provider.go
      cluster.go
      app.go
  /logging/
    logger.go           (adapter; optional)

/models/
  cfgops/               (設定読み込みモデル。domain へ直接持ち込まないラッパ層)

/docs/
  *.ja.md               (日本語可)

/infra/                 (Azure Developer CLI IaC)

azure.yaml              (Azure Developer CLI configuration)
```

## リソース

このプロジェクトでは次の種類 (kind) のリソースを管理する。
UseCase で REST API の CRUD 操作を定義する。

- Service
- Provider
- Cluster
- App

## 永続化層

このプロジェクトでは次のような URL で永続化データベースを指定できる。

```
file:/path/to/kompoxops.yml
sqlite:/path/to/database.sqlite3
postgres://<username>:<password>@<host>:<port>/<database>
mysql://<username>:<password>@<host>:<port>/<database>
```

file は指定ファイルのリソース定義をインメモリストレージに読み込む。

sqlite/postgres/mysql の実装には gorm を使用する。

## ドメイン層

- ディレクトリ `/domain/model/` でドメインモデルとドメインポートを定義する
  - ドメインモデルはビジネスロジックのデータを保持する構造体
  - ドメインポートはビジネスロジックのコードを実装するためのインターフェースであり、ユースケース層から呼び出される
- 例: ファイル `/domain/model/cluster.go`
  - ドメインモデル構造体 `model.Cluster`
  - ドメインポートインターフェース `model.ClusterPort`
  - 入出力型 `model.ClusterStatus`
- ファイル `/adapters/drivers/provider/<resource>_port.go` で次を定義する
  - ドメインポート取得関数 例: `providerdrv.GetClusterPort()`

```go
// ドメインモデル構造体
type Cluster struct { ... }

// ドメインポートインターフェース
type ClusterPort interface {
	Status(ctx context.Context, cluster *Cluster) (*ClusterStatus, error)
	Provision(ctx context.Context, cluster *Cluster) error
	Deprovision(ctx context.Context, cluster *Cluster) error
	Install(ctx context.Context, cluster *Cluster) error
	Uninstall(ctx context.Context, cluster *Cluster) error
}

// ドメインポート入出力型
type ClusterStatus struct { ... }
```

## ユースケース層

- ディレクトリ `/usecase/<resource>/` でユースケース構造体とユースケースメソッドを定義する
- ユースケース構造体
  - 構造体 `UseCase` を `types.go` で定義する
  - api/cmd など上位層で構造体メンバをワイヤリングする
- ユースケースメソッド
  - 1 ユースケースメソッド = 1 ファイルで定義（肥大化時のみ細分化）
  - メソッドの引数型名・戻値型名の接尾語は `Input` / `Output` を使用する

```go
// ユースケース構造体 (types.go)
type UseCase struct {
	Clusters    domain.ClusterRepository
	Providers   domain.ProviderRepository
	ClusterPort model.ClusterPort
}

// ユースケースメソッド (create.go)
func (u *UseCase) Create(ctx context.Context, in CreateInput) (*model.Service, error)

// ユースケースメソッド (status.go)
func (u *UseCase) Status(ctx context.Context, in StatusInput) (*StatusOutput, error)
```

## 主なレイヤ依存方向

api(cmd) → usecase → domain ← adapters(drivers, store, kube)
