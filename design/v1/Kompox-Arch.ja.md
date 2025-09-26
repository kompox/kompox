---
title: Kompox PaaS Architecture
version: v1
status: out-of-sync
updated: 2025-09-26
language: ja
---

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
    client.go           Kubernetes client
    installer.go        Kubernetes infrastructure installer
    compose_convert.go  Compose to Manifest converter
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

ユースケース層はアプリケーション固有の操作（シナリオ）をまとめ、ドメインモデル・ドメインポート・外部アダプタをオーケストレーションする。I/O DTO を明示し、安定したインターフェースを提供する。

### 配置とファイル構成
- ディレクトリ: `/usecase/<resource>/`
- `types.go`: `UseCase` 構造体と共有補助型のみを定義（リポジトリ束 `Repos` やポートフィールドなど）
- 1 ユースケースメソッド = 1 ファイル（`create.go`, `list.go`, `update.go`, ...）。肥大化/共通化が必要になった場合のみ更に分割
- 各メソッドファイル内でそのメソッド専用の `XxxInput` / `XxxOutput` 型を定義する（他メソッドと共有しない）

### 命名・シグネチャ規約
- 公開メソッドシグネチャ: `func (u *UseCase) Xxx(ctx context.Context, in *XxxInput) (*XxxOutput, error)`
  - `in` は必ず **ポインタ**。nil チェックによるバリデーション、将来のフィールド追加、構造体サイズ増大時のコピーコスト抑制を狙う
  - 戻り値は *Output と error の 2 値固定。従来 error のみだった操作でも空 `XxxOutput` を返す（インターフェース一貫性を確保）
- DTO 型名接尾語は `Input` / `Output`。他の接尾語は付与しない
- メソッド名は動詞（UseCase の操作）を基本とし、状態取得は `Get` / 集合取得は `List` / 状態監視は `Status` など統一

### DTO (Input/Output) 規約
- フィールド名はコンテキストによる省略を禁止（例: AppID を ID としない）。`AppID`, `ProviderID`, `ClusterID` 等の完全形を維持
- 直列化要件: すべての公開フィールドに JSON タグ（snake_case）を付与し、安定したストレージ/ログ出力互換性を確保
- `Output` が 1 つの主要エンティティを返す場合は `Service *model.Service` のように明示。複数の場合は `Services []*model.Service` 等の複数形
- 追加情報（メタデータや集計値）が必要な場合も `Output` にフィールドを追加し、戻り値シグネチャは変えない
- 空出力は `type DeleteOutput struct{}` のように空 struct を許可

### ドキュメントコメント
- すべての公開メソッド、公開 DTO 型、および公開フィールドに GoDoc 形式のコメントを付与（why / what を簡潔に）
- コメントは実装理由や副作用、重要なバリデーションルールを含める。プロンプトや運用手順等のメタ情報は含めない

### エラーハンドリング
- `Input` が nil の場合は即座にバリデーションエラー（ドメイン定義のエラー型）を返す
- ID / Name など必須フィールドの空文字列はバリデーションで弾き、ドメイン層へ渡さない

### 例: `types.go`
```go
// Repos はユースケースが利用するリポジトリ依存関係の束。
type Repos struct {
  Service  domain.ServiceRepository
  Provider domain.ProviderRepository
  Cluster  domain.ClusterRepository
  App      domain.AppRepository
}

// UseCase は <resource> に対するアプリケーションサービスを提供する。
type UseCase struct {
  Repos       *Repos
  ClusterPort model.ClusterPort // 必要なドメインポートを保持
}
```

### 例: 作成メソッド (`create.go`)
```go
// CreateInput はリソース作成パラメータ。
type CreateInput struct {
  Name string `json:"name"`
}

// CreateOutput は作成結果。
type CreateOutput struct {
  Service *model.Service `json:"service"`
}

// Create は新しい Service を作成する。
func (u *UseCase) Create(ctx context.Context, in *CreateInput) (*CreateOutput, error) {
  if in == nil { return nil, model.ErrInvalidArgument }
  if in.Name == "" { return nil, model.ErrInvalidArgument }
  svc := &model.Service{ /* ... */ }
  if err := u.Repos.Service.Create(ctx, svc); err != nil { return nil, err }
  return &CreateOutput{Service: svc}, nil
}
```

### 例: 状態取得メソッド (`status.go`)
```go
// StatusInput はクラスタ状態取得パラメータ。
type StatusInput struct {
  ClusterID string `json:"cluster_id"`
}

// StatusOutput は状態情報を保持。
type StatusOutput struct {
  Status *model.ClusterStatus `json:"status"`
}

// Status はクラスタの最新状態を取得する。
func (u *UseCase) Status(ctx context.Context, in *StatusInput) (*StatusOutput, error) {
  if in == nil || in.ClusterID == "" { return nil, model.ErrInvalidArgument }
  cluster, err := u.Repos.Cluster.Get(ctx, in.ClusterID)
  if err != nil { return nil, err }
  st, err := u.ClusterPort.Status(ctx, cluster)
  if err != nil { return nil, err }
  return &StatusOutput{Status: st}, nil
}
```

### 例: 削除のみのメソッド (`delete.go`)
```go
type DeleteInput struct { ServiceID string `json:"service_id"` }
type DeleteOutput struct{}

func (u *UseCase) Delete(ctx context.Context, in *DeleteInput) (*DeleteOutput, error) {
  if in == nil || in.ServiceID == "" { return nil, model.ErrInvalidArgument }
  if err := u.Repos.Service.Delete(ctx, in.ServiceID); err != nil { return nil, err }
  return &DeleteOutput{}, nil
}
```

### テスト指針
- ユースケース単体テストではポート/リポジトリをモック化し、Input バリデーションと副作用（呼び出し回数・順序）を検証
- シリアライズ互換性を壊さないため、重要 DTO の JSON スナップショットテストを追加してもよい

### 将来拡張
- Input/Output へのフィールド追加は JSON タグ互換 (追加のみ) で非互換変更を避ける
- 異なるバージョンの API を提供する場合は パッケージ分割 (`usecasev2/...`) ではなく facade 層でのアダプタを推奨

