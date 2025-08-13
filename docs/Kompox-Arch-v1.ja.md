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
    /memory/             (in-memory)
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

このプロジェクトでは次の種類のリソースを管理する。
UseCase で REST API の CRUD 操作を定義する。

- Service
- Provider
- Cluster
- App

## データベース

このプロジェクトでは次のような URL でデータベースを指定できる。

```
memory:
sqlite:/path/to/database.sqlite3
postgres://<username>:<password>@<host>:<port>/<database>
mysql://<username>:<password>@<host>:<port>/<database>
```

memory はテスト目的のインメモリストレージである。
sqlite/postgres/mysql の実装は gorm を使用する。

## 命名規則 (UseCase 層)

- ディレクトリ: /usecase/<resource>/
- ファイル名: アクション動詞 (create.go / update.go / list.go など)
- 1 ユースケース = 1 ファイル（肥大化時のみ細分化）
- 型命名:
  - `type ServiceUseCase struct { Repo domain.ServiceRepository ... }`
  - `func (u *ServiceUseCase) Create(ctx context.Context, cmd CreateServiceCommand) (*service.Service, error)`
- Command/Query オブジェクト: `CreateServiceCommand` / `ListServicesQuery`
- HTTP/CLI 層は Command/Query を組み立てて呼び出す
- 重複 (service_service.go) を避けるため「リソース名をディレクトリ」「動詞をファイル」

## 主なレイヤ依存方向

api(cmd) → usecase → domain ← adapters(drivers, store, kube)

## Admin CLI (kompoxops admin)

管理用途 CLI を提供する。
一般ユーザー向け REST API とは異なり認証認可を無視した OOB の接続により
UseCase で規定する CRUD 操作を直接呼び出すことができる。

```
kompoxops admin [--db-url <URL>] <KIND> <VERB> <options...>
kompoxops admin service list
kompoxops admin service get svc-a
kompoxops admin service create -f svc-a.yml
kompoxops admin service update svc-a -f svc-a.yml
kompoxops admin service delete svc-a
```
