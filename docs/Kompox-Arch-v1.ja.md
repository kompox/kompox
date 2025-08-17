# Kompox PaaS Architecture

## Package / Directory Hierarchy

```
cmd/
  kompoxops/            main for CLI: wiring + DI

api/
  server.go             HTTP server bootstrap
  router.go             mux/router setup
  handlers/
    app_handler.go
    cluster_handler.go
    provider_handler.go
    service_handler.go

usecase/                Use case layer
  service/              Service resource
    list.go             List method
    get.go              Get method
    create.go           Create method
    update.go           Update method
    delete.go           Delete method
  cluster/              Cluster resource
  provider/             Provider resource
  app/                  App resource

domain/                 Domain layer
  model/                Domain models: entity definitions, port interfaces
  repository.go         Dmain model repository interfaces

adapters/
  kube/
    kompose/            Converter implementation (kompose)
    client.go           Kubernetes client
    installer.go        Kubernetes infrastructure installer
    convert.go          Compose to Manifest converter
  drivers/              Drivers
    provider/           Provider drivers (providerdrv)
      aks/
      k3s/
      cluster_port.go   model.ClusterPort implementation
      registry.go       Driver interface and registration factory
    ingress/            ingress drivers (ingressdrv)
      traefik/
      nginx/
      registry.go
  store/                Domain model repository implementations
    inmem/              in-memory
    rdb/                gorm

internal/
  logging/              Logging infrastructure (slog)
  compose/              Compose loader and validator (compose-go)

config/
  kompoxopscfg/         komposops.cfg loader

infra/
  aks/
    infra/              Azure Bicep IaC
      main.json         AKS ARM template output
    azure.yaml          Azure Developer CLI config

docs/
  *.ja.md               日本語可
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
  - 上位層 api/cmd で構造体メンバをワイヤリングする
- ユースケースメソッド
  - 1 ユースケースメソッド = 1 ファイルで定義（肥大化時のみ細分化）
  - メソッドの引数型名・戻値型名の接尾語は `Input` / `Output` を使用する

```go
// リポジトリ構造体 (types.go)
type Repos struct {
	Cluster  domain.ClusterRepository
	Provider domain.ProviderRepository
}

// ユースケース構造体 (types.go)
type UseCase struct {
	Repos       *Repos
	ClusterPort model.ClusterPort
}

// ユースケースメソッド (create.go)
type CreateInput struct { ... }
func (u *UseCase) Create(ctx context.Context, in CreateInput) (*model.Service, error)

// ユースケースメソッド (status.go)
type StatusInput struct { ... }
type StatusOutput struct { ... }
func (u *UseCase) Status(ctx context.Context, in StatusInput) (*StatusOutput, error)
```
