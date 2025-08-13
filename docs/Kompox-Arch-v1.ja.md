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
    create.go
    update.go
    delete.go
    list.go
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
    /inmem/
      service.go
      provider.go
      cluster.go
      app.go
    /postgres/
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
