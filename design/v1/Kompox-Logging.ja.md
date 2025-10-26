---
id: Kompox-Logging
title: Kompox ロギング仕様
version: v1
status: draft
updated: 2025-10-24
language: ja
---

# Kompox ロギング仕様

## 概要

Kompox プロジェクトにおけるログ出力の標準仕様を定義する。

- Go 標準の `log/slog` パッケージによる構造化ログを使用
- 2 種類のログパターン (Span / Step) を使い分け
- human / json 両方のフォーマットをサポート
- 標準実装パターン
  - Named return value `(err error)` と `defer func() { cleanup(err) }()` の活用

## ログフォーマット

### フォーマットオプション

`--log-format` フラグまたは `KOMPOX_LOG_FORMAT` 環境変数で指定:

- `human` (既定): 人間が読みやすいテキスト形式
- `json`: 機械処理に適した JSON Lines 形式

### human フォーマット

```
YYYY/MM/DD HH:MM:SS LEVEL MSG key1=val1 key2=val2 ...
```

例:
```
2025/10/24 09:51:05 INFO CMD:app.deploy:START runId=0t4mrd528u3s resourceId=/ws/w1/prv/p1/cls/c1/app/a1
2025/10/24 09:51:14 INFO CMD:app.deploy:END:OK runId=0t4mrd528u3s resourceId=/ws/w1/prv/p1/cls/c1/app/a1 err="" elapsed=8.54
```

### json フォーマット

```json
{"time":"YYYY-MM-DDTHH:MM:SSZ","level":"LEVEL","msg":"MSG","key1":"val1","key1":"val2",...}
```

例:
```json
{"time":"2025-10-24T09:51:05Z","level":"INFO","msg":"CMD:app.deploy:START","runId":"0t4mrd528u3s","resourceId":"/ws/w1/prv/p1/cls/c1/app/a1"}
{"time":"2025-10-24T09:51:14Z","level":"INFO","msg":"CMD:app.deploy:END:OK","runId":"0t4mrd528u3s","resourceId":"/ws/w1/prv/p1/cls/c1/app/a1","err":"","elapsed":8.54}
```

## ログ属性

構造化ログの属性名は原則としてキャメルケース (camelCase) を使用する。

一般的な属性名と属性値の説明:

| 属性名 | 属性値の例 | 説明 |
|-|-|-|
| `time` | `2025-10-24T09:51:05Z` | ログ出力時刻 (RFC3339 形式) |
| `level` | `DEBUG` `INFO` `WARN` `ERROR` | ログレベル |
| `msg` | `CMD:app.deploy:START` `CMD:app.deploy:END:OK` | メッセージシンボル文字列 (必須) |
| `desc` | `Deploy application a1 to cluster c1` | メッセージ詳細文字列 (任意) |
| `err` | `""` `manifest validation failed: ...` | メッセージエラー文字列 (成功時は空文字列) |
| `elapsed` | `8.54` | 経過時間 (秒数、float64) |
| `runId` | `0t4mrd528u3s` | トレース: 実行単位の一意識別子 |
| `cmd` | `app.deploy` | トレース: 実行コマンド名 |
| `resourceId` | `/ws/w1/prv/p1/cls/c1/app/a1` | トレース: 対象リソースFQN |

## ログパターン

### Span パターン

長時間実行される操作や、開始・終了を明示的に記録したい操作に使用。

#### 属性

msg 属性には定型のメッセージシンボル文字列を設定する

- 開始: `[<prefix>:]<operation>:START`
- 成功終了: `[<prefix>:]<operation>:END:OK`
- 失敗終了: `[<prefix>:]<operation>:END:FAILED`

その他の属性

- 開始:
  - `level`: `INFO`
  - `desc` (任意) 操作の詳細説明
  - `runId` `cmd` `resourceId` などのトレース属性
- 終了:
  - `level`:
    - 成功 `INFO`
    - 失敗 `WARN` または `ERROR`
  - `err` メッセージエラー文字列
  - `elapsed` 経過秒数
  - `desc` (任意) 結果の詳細説明
  - トレース属性は開始時と一致させる

err 属性

- 成功時: `err=""` (空文字列)
- 失敗時: エラーメッセージの最初の32文字 (32文字超の場合は `...` を付与)

elapsed 属性

- 開始から終了までの経過時間(秒)を float で設定する
- 例: `8.54`, `120.3`

#### 出力例

human フォーマット:
```
2025/10/24 09:51:05 INFO CMD:cluster.install:START runId=0t4mrd528u3s resourceId=/ws/w1/prv/p1/cls/c1
2025/10/24 09:51:40 INFO CMD:cluster.install:END:OK runId=0t4mrd528u3s resourceId=/ws/w1/prv/p1/cls/c1 err="" elapsed=34.67
```

json フォーマット:
```json
{"time":"2025-10-24T09:51:05Z","level":"INFO","msg":"CMD:cluster.install:START","runId":"0t4mrd528u3s","resourceId":"/ws/w1/prv/p1/cls/c1"}
{"time":"2025-10-24T09:51:40Z","level":"INFO","msg":"CMD:cluster.install:END:OK","runId":"0t4mrd528u3s","resourceId":"/ws/w1/prv/p1/cls/c1","err":"","elapsed":34.67}
```

ネストした出力例 (トレース属性の継承):
```
2025/10/24 09:51:05 INFO CMD:app.deploy:START runId=0t4mrd528u3s cmd=app.deploy resourceId=/ws/w1/prv/p1/cls/c1/app/a1
2025/10/24 09:51:06 INFO AKS:ClusterProvision:START runId=0t4mrd528u3s cmd=app.deploy resourceId=/ws/w1/prv/p1/cls/c1/app/a1
2025/10/24 09:51:18 WARN AKS:ClusterProvision:END:FAILED runId=0t4mrd528u3s cmd=app.deploy resourceId=/ws/w1/prv/p1/cls/c1/app/a1 err="resource group not found: rg-foo" elapsed=12.34
2025/10/24 09:51:19 WARN CMD:app.deploy:END:FAILED runId=0t4mrd528u3s cmd=app.deploy resourceId=/ws/w1/prv/p1/cls/c1/app/a1 err="AKS operation failed: resourc..." elapsed=14.2
```

### Step パターン

短時間の操作や、成功が通常パスで失敗のみ注目すべき操作に使用。

#### 属性

msg 属性には定型のメッセージシンボル文字列を設定する

- 開始: `[<prefix>:]<operation>`
- 成功終了: ログ出力なし
- 失敗終了: `[<prefix>:]<operation>:FAILED`

その他の属性

- 開始:
  - `level`: `INFO`
  - `desc` (任意) 操作の詳細説明
  - `runId` `cmd` `resourceId` などのトレース属性
- 失敗終了:
  - `level`: `WARN` または `ERROR`
  - `err` メッセージエラー文字列
  - `desc` (任意) 結果の詳細説明
  - トレース属性は開始時と一致させる

err 属性

- 失敗時: エラーメッセージの最初の32文字 (32文字超の場合は `...` を付与)

#### 出力例

human フォーマット:
```
2025/10/24 09:51:11 INFO APPLY kind=Namespace name=k4x-4p4y0a-basic-e126hy
2025/10/24 09:51:12 INFO APPLY kind=ServiceAccount name=traefik namespace=ingress-k4x-4p4y0a
2025/10/24 09:51:13 INFO APPLY kind=Secret name=db-creds namespace=k4x-app
2025/10/24 09:51:13 WARN APPLY:FAILED kind=Secret name=db-creds namespace=k4x-app err="already exists"
2025/10/24 09:51:14 INFO ROLE keyVault=kv-prod principalId=abc123

```

json フォーマット:
```json
{"time":"2025-10-24T09:51:11Z","level":"INFO","msg":"APPLY","kind":"Namespace","name":"k4x-4p4y0a-basic-e126hy"}
{"time":"2025-10-24T09:51:12Z","level":"INFO","msg":"APPLY","kind":"ServiceAccount","name":"traefik","namespace":"ingress-k4x-4p4y0a"}
{"time":"2025-10-24T09:51:13Z","level":"INFO","msg":"APPLY","kind":"Secret","name":"db-creds","namespace":"k4x-app"}
{"time":"2025-10-24T09:51:13Z","level":"WARN","msg":"APPLY:FAILED","kind":"Secret","name":"db-creds","namespace":"k4x-app","err":"already exists"}
{"time":"2025-10-24T09:51:14Z","level":"INFO","msg":"ROLE","keyVault":"kv-prod","principalId":"abc123"}
```

## 実装例

### Span パターン: withCmdRunLogger

[cmd/kompoxops/logging.go] の `withCmdRunLogger` 関数は Span パターンを実装している。

```go
// withCmdRunLogger implements the Span pattern for CLI command logging.
// It emits a START log line and returns a context with logger attributes attached,
// plus a cleanup function to emit the END:OK or END:FAILED log line.
//
// Usage:
//
//	ctx, cleanup := withCmdRunLogger(ctx, "cluster.provision", resourceID)
//	defer func() { cleanup(err) }()
//
// Log message format:
// - START:  CMD:<operation>:START (with runId, resourceId in logger attributes)
// - END:    CMD:<operation>:END:OK or CMD:<operation>:END:FAILED (with err, elapsed in logger attributes)
//
// See design/v1/Kompox-Logging.ja.md for the full Span pattern specification.
func withCmdRunLogger(ctx context.Context, operation, resourceID string) (context.Context, func(err error)) {
	runID, err := naming.NewCompactID()
	if err != nil {
		// Fallback to a fixed value if ID generation fails
		runID = "error"
	}

	startAt := time.Now()

	// Attach runId, resourceId to logger and return new context
	logger := logging.FromContext(ctx).With("runId", runID, "resourceId", resourceID)
	ctx = logging.WithLogger(ctx, logger)

	// Emit START log line
	logger.Info(ctx, "CMD:"+operation+":START")

	cleanup := func(err error) {
		elapsed := time.Since(startAt).Seconds()
		var msg, errStr string
		if err == nil {
			msg = "CMD:" + operation + ":END:OK"
			errStr = ""
		} else {
			msg = "CMD:" + operation + ":END:FAILED"
			errMsg := err.Error()
			if len(errMsg) > 32 {
				errStr = errMsg[:32] + "..."
			} else {
				errStr = errMsg
			}
		}

		if err == nil {
			logger.Info(ctx, msg, "err", errStr, "elapsed", elapsed)
		} else {
			logger.Warn(ctx, msg, "err", errStr, "elapsed", elapsed)
		}
	}

	return ctx, cleanup
}
```

## ベストプラクティス

### 属性名

- キャメルケース (`camelCase`) を使用
- 簡潔で明確な名前を付ける
- 予約語との衝突を避ける

### ログメッセージ

- `msg` (必須) には定型のメッセージシンボル文字列のみを設定する
- `desc` (任意) に自由形式のメッセージ詳細文字列を設定する

### エラーハンドリング

- Named return value `(err error)` を使用
- `defer func() { cleanup(err) }()` で自動キャプチャ

### コンテキスト伝搬

- `logger := logging.FromContext(ctx)` で追加属性ロガーを取得
- `ctx = logging.WithLogger(ctx, logger)` として下位層に追加属性ロガーつきの ctx を伝搬させる

## 参考

- [2025-10-24-log.ja.md] - 実装タスク
- [cmd/kompoxops/logging.go] - コマンドレイヤー実装
- [adapters/drivers/provider/aks/logging.go] - ドライバーレイヤー実装
- [internal/logging] - ロギングパッケージ

[2025-10-24-log.ja.md]: ../../_dev/tasks/2025-10-24-log.ja.md
[cmd/kompoxops/logging.go]: ../../cmd/kompoxops/logging.go
[adapters/drivers/provider/aks/logging.go]: ../../adapters/drivers/provider/aks/logging.go
[internal/logging]: ../../internal/logging
