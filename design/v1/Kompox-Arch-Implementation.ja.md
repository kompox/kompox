---
id: Kompox-Arch-Implementation
title: Kompox Implementation Architecture
version: v1
status: synced
updated: 2025-10-12
language: ja
---

# Kompox Implementation Architecture

## Package / Directory Hierarchy

```
cmd/
  kompoxops/            package main: CLI エントリポイントとコマンド定義
    main.go             main()
    version.go          バージョン情報 (GoReleaser がビルド時に上書き)
    repos_builder.go    Repos ビルダー・キャッシュ
    usecase_builder.go  Usecase ビルダー
    cmd_*.go            コマンド定義 (cobra)

adapters/               外部アダプタ群
  kube/                 package kube: Kubernetes クライアントとマニフェスト変換ヘルパー
  drivers/
    provider/           package providerdrv: プロバイダドライバ共通インターフェース
      aks/              package aks: Azure AKS プロバイダドライバ実装
      k3s/              package k3s: K3s プロバイダドライバ（スタブ）
      registry.go       providerdrv.Driver インターフェースおよび登録ファクトリ
      cluster_port.go   model.ClusterPort アダプタ
      volume_port.go    model.VolumePort アダプタ
  store/                ドメインリポジトリ実装
    inmem/              package inmem: インメモリ実装
    rdb/                package rdb: gorm ベース実装

domain/                 package domain: ドメイン層
  model/                package model: エンティティとポート定義
  repository.go         リポジトリインターフェース

usecase/                ユースケース層
  app/                  package app: App リソースのユースケース
  box/                  package box: Box 開発環境ユースケース
  cluster/              package cluster: Cluster リソースのユースケース
  provider/             package provider: Provider リソースのユースケース
  secret/               package secret: Secret リソースのユースケース
  workspace/            package workspace: Workspace リソースのユースケース
  volume/               package volume: Volume リソースのユースケース

config/
  kompoxopscfg/         package kompoxopscfg: CLI 設定ローダーとコンバータ

infra/
  aks/
    infra/              AKS 向け Bicep/IaC アセット
    scripts/            Azure CLI 等の運用スクリプト
    azure.yaml          Azure Developer CLI 設定

internal/               内部ユーティリティ
  kubeconfig/           package kubeconfig: kubeconfig の読み書きヘルパー
  logging/              package logging: slog ベースのロギング設定
  naming/               package naming: リソース命名規約ユーティリティ
  terminal/             package terminal: CLI ターミナル I/O ヘルパー

tests/                  E2E テストシナリオ
  aks-e2e-basic/
  aks-e2e-gitea/
  aks-e2e-gitlab/
  aks-e2e-redmine/

docker/                 Docker ビルド

design/                 仕様・アーキテクチャドキュメント
  adr/                  ADR (Architecture Decision Records)
  v1/                   v1 (Current CLI)
  v2/                   v2 (Future PaaS/Operator)
  pub/                  登壇など公開資料
_dev/                   開発者向け階層
  tasks/                タスクトラッキング

.goreleaser.yml         GoReleaser 設定ファイル
Makefile                Make タスク定義
```

## リソース

このプロジェクトでは次の種類 (kind) のリソースを管理する。
UseCase で REST API の CRUD 操作を定義する。

- Workspace
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
// Cluster represents a Kubernetes cluster resource.
type Cluster struct { ... }

// ドメインポートインターフェース
// ClusterPort is an interface (domain port) for cluster operations.
type ClusterPort interface {
	Status(ctx context.Context, cluster *Cluster) (*ClusterStatus, error)
	Provision(ctx context.Context, cluster *Cluster, opts ...ClusterProvisionOption) error
	Deprovision(ctx context.Context, cluster *Cluster, opts ...ClusterDeprovisionOption) error
	Install(ctx context.Context, cluster *Cluster, opts ...ClusterInstallOption) error
	Uninstall(ctx context.Context, cluster *Cluster, opts ...ClusterUninstallOption) error
}

// ドメインポート入出力型
// ClusterStatus represents the status of a cluster.
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
- `Output` が 1 つの主要エンティティを返す場合は `Workspace *model.Workspace` のように明示。複数の場合は `Workspaces []*model.Workspace` 等の複数形
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
  Workspace domain.WorkspaceRepository
  Provider  domain.ProviderRepository
  Cluster   domain.ClusterRepository
  App       domain.AppRepository
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
  Workspace *model.Workspace `json:"workspace"`
}

// Create は新しい Workspace を作成する。
func (u *UseCase) Create(ctx context.Context, in *CreateInput) (*CreateOutput, error) {
  if in == nil { return nil, model.ErrInvalidArgument }
  if in.Name == "" { return nil, model.ErrInvalidArgument }
  ws := &model.Workspace{ /* ... */ }
  if err := u.Repos.Workspace.Create(ctx, ws); err != nil { return nil, err }
  return &CreateOutput{Workspace: ws}, nil
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
type DeleteInput struct { WorkspaceID string `json:"workspace_id"` }
type DeleteOutput struct{}

func (u *UseCase) Delete(ctx context.Context, in *DeleteInput) (*DeleteOutput, error) {
  if in == nil || in.WorkspaceID == "" { return nil, model.ErrInvalidArgument }
  if err := u.Repos.Workspace.Delete(ctx, in.WorkspaceID); err != nil { return nil, err }
  return &DeleteOutput{}, nil
}
```

### テスト指針

- ユースケース単体テストではポート/リポジトリをモック化し、Input バリデーションと副作用（呼び出し回数・順序）を検証
- シリアライズ互換性を壊さないため、重要 DTO の JSON スナップショットテストを追加してもよい

### 将来拡張

- Input/Output へのフィールド追加は JSON タグ互換 (追加のみ) で非互換変更を避ける
- 異なるバージョンの API を提供する場合は パッケージ分割 (`usecasev2/...`) ではなく facade 層でのアダプタを推奨

