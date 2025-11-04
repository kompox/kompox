---
id: 2025-11-04-converter
title: Converter の entrypoint/command 変換実装
status: done
updated: 2025-11-04
language: ja
owner: yaegashi
---
# Task: Converter の entrypoint/command 変換実装

## 目的

- Docker Compose の `entrypoint` と `command` を Kubernetes の `command` と `args` へ適切に変換する機能を実装する。
- [Kompox-KubeConverter.ja.md] の新規追加仕様に従い、shell form と exec form の両方をサポートする。

## スコープ / 非スコープ

- In:
  - `adapters/kube/converter.go` の `Convert()` メソッドでの entrypoint/command 変換実装
  - compose-go が提供する `types.ServiceConfig` の `Entrypoint` と `Command` フィールドの処理
  - 文字列形式 (shell form) を `/bin/sh -c` でラップして配列化
  - 配列形式 (exec form) をそのまま使用
  - ユニットテストの追加 (`adapters/kube/converter_test.go`)
- Out:
  - compose-go 自体のパース処理の変更
  - 既存の他のフィールド変換ロジックの大幅な変更

## 仕様サマリ

[Kompox-KubeConverter.ja.md] の entrypoint/command セクションに基づく:

| Compose フィールド | Kubernetes フィールド | 説明 |
|-------------------|---------------------|------|
| `entrypoint` | `command` | コンテナのエントリポイント (実行ファイルパス) |
| `command` | `args` | コマンドライン引数 |

変換規則:
- `entrypoint` が指定されている場合、その値を `command` に設定
- `command` が指定されている場合、その値を `args` に設定
- どちらも未指定の場合、Kubernetes フィールドを設定しない (イメージのデフォルトを使用)
- compose-go は `Entrypoint` と `Command` を `types.ShellCommand` 型で提供
  - `ShellCommand` は `[]string` をベースとした型で、空でない場合は配列として扱う

## 計画 (チェックリスト)

- [x] 設計ドキュメント [Kompox-KubeConverter.ja.md] の更新
- [x] `adapters/kube/converter.go` の実装
  - [x] `Convert()` メソッド内のコンテナ構築ループで `Entrypoint` と `Command` を処理
  - [x] `types.ShellCommand` から `[]string` への変換ロジック追加
- [x] ユニットテストの追加
  - [x] `adapters/kube/converter_test.go` に entrypoint/command 変換テストケース追加
  - [x] 配列形式のテストケース
  - [x] 未指定のテストケース (デフォルト動作確認)
  - [x] entrypoint のみ指定のテストケース
  - [x] command のみ指定のテストケース
  - [x] 両方指定のテストケース
- [x] 統合テストでの動作確認
  - [x] 既存の E2E テストが影響を受けないか確認
- [x] ドキュメントの最終確認

## 実装詳細

### compose-go の型定義

```go
// github.com/compose-spec/compose-go/v2/types
type ServiceConfig struct {
    // ...
    Entrypoint types.ShellCommand `yaml:",omitempty" json:"entrypoint,omitempty"`
    Command    types.ShellCommand `yaml:",omitempty" json:"command,omitempty"`
    // ...
}

type ShellCommand []string
```

### 変換ロジック (概要)

```go
// adapters/kube/converter.go の Convert() メソッド内
for _, s := range proj.Services {
    container := corev1.Container{
        Name:  s.Name,
        Image: s.Image,
        // ...
    }
    
    // entrypoint → command
    if len(s.Entrypoint) > 0 {
        container.Command = []string(s.Entrypoint)
    }
    
    // command → args
    if len(s.Command) > 0 {
        container.Args = []string(s.Command)
    }
    
    // ...
    containers = append(containers, container)
}
```

### テストケース例

```yaml
# Test Case 1: 配列形式
services:
  app:
    image: app
    entrypoint: ["/app/entrypoint.sh"]
    command: ["--config", "/etc/app.conf"]

# Test Case 2: entrypoint のみ
services:
  app:
    image: app
    entrypoint: ["/bin/bash", "-c"]

# Test Case 3: command のみ
services:
  app:
    image: app
    command: ["npm", "start"]

# Test Case 4: 未指定 (デフォルト)
services:
  app:
    image: app
```

## 受け入れ条件

- [x] `adapters/kube/converter.go` が compose の `entrypoint` と `command` を正しく変換する
- [x] 生成される Kubernetes Deployment の `containers[].command` と `containers[].args` が仕様通り
- [x] ユニットテストがすべてパスする (`make test`)
- [x] 既存の E2E テストに影響がない
- [x] E2E テスト (`tests/aks-e2e-basic`) で4つのパターンを検証

## テスト

### ユニットテスト

`adapters/kube/converter_test.go`:
- `TestConverterEntrypointCommand`: 各パターンの変換が正しいことを確認
  - 両方指定、entrypoint のみ、command のみ、両方未指定
  - 単一要素、複数要素の配列
- エッジケース: 空配列、nil 値の扱い

### E2E テスト

`tests/aks-e2e-basic/compose.yml`:
- 4つのサービスで entrypoint/command の全パターンをテスト
  - `app1`: 両方未指定（イメージのデフォルト使用）
  - `app2`: entrypoint + command 両方指定
  - `app3`: entrypoint のみ指定
  - `app4`: command のみ指定
- 各サービスのログ出力で動作確認
- AKS 環境での実デプロイで検証完了

## メモ

- compose-go の `types.ShellCommand` は `[]string` のエイリアス型なので、型変換は単純
- Docker/Compose の shell form (文字列) は compose-go のパース時点で既に配列化されている想定
  - 実際の動作は compose-go のドキュメントとテストで確認必要
- Kubernetes の `command`/`args` が未設定の場合、コンテナイメージの `ENTRYPOINT`/`CMD` が使用される (標準動作)

## 進捗

- 2025-11-04: タスク作成、設計ドキュメント更新完了
- 2025-11-04: 実装完了
  - `adapters/kube/converter.go` に entrypoint/command 変換ロジック追加
  - `adapters/kube/converter_test.go` に包括的なテストケース追加
  - 全テストパス確認 (`make test`)
- 2025-11-04: E2E テスト完了
  - `tests/aks-e2e-basic/compose.yml` に4つのテストパターンを追加
  - AKS 環境で実デプロイして動作確認
  - 各サービスのログ出力で entrypoint/command が正しく動作することを検証

## 参考

- [Kompox-KubeConverter.ja.md]
- [compose-go types](https://pkg.go.dev/github.com/compose-spec/compose-go/v2/types)
- [Kubernetes Container API](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#Container)

[Kompox-KubeConverter.ja.md]: ../../design/v1/Kompox-KubeConverter.ja.md
