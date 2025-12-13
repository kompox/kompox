---
id: Kompox-Logging
title: Kompox ロギング仕様
version: v1
status: synced
updated: 2025-12-13
language: ja
---

# Kompox ロギング仕様

## 概要

Kompox プロジェクトにおけるログ出力の標準仕様を定義する。

- Go 標準の `log/slog` パッケージによる構造化ログを使用
- 3 種類のログパターン (Event / Span / Step) を使い分け
- human / json 両方のフォーマットをサポート
- ファイルベースのログ出力戦略 ([K4x-ADR-016])
  - 既定ではコンソールへのログ出力を行わない
  - 構造化ログはファイルに自動出力
- 標準実装パターン
  - Named return value `(err error)` と `defer func() { cleanup(err) }()` の活用

## ログ出力

### 出力先と制御

構造化ログは既定でファイルに出力される。コンソール (stderr) への出力は明示的に設定した場合のみ行われる。

**CLI フラグ**

| フラグ | 説明 |
|--------|------|
| `--log-format <FORMAT>` | ログ形式: `json` (既定) または `human` |
| `--log-level <LEVEL>` | 最小ログレベル: `DEBUG`, `INFO` (既定), `WARN`, `ERROR` |
| `--log-output <PATH>` | ログ出力先 (下表参照) |

**`--log-output` の値**

| 値 | 動作 |
|----|------|
| (空/省略) | `$KOMPOX_LOG_DIR/kompoxops-YYYYMMDD-HHMMSS-sss.log` に出力 |
| `<path>` | 指定パスに出力 (絶対パスまたは `$KOMPOX_LOG_DIR` 相対) |
| `-` | stderr に出力 |
| `none` | ログ出力を無効化 |

**環境変数**

| 変数 | 説明 |
|------|------|
| `KOMPOX_LOG_DIR` | ログディレクトリ (既定: `$KOMPOX_DIR/logs`) |
| `KOMPOX_LOG_FORMAT` | ログ形式 (既定: `json`) |
| `KOMPOX_LOG_LEVEL` | ログレベル (既定: `INFO`) |
| `KOMPOX_LOG_OUTPUT` | ログ出力先 |

注: `-v`, `--verbose`, `--debug` フラグは構造化ログ出力に影響しない。これらは各コマンドが stdout/stderr への出力内容を制御するために独自に解釈する。

### ファイル出力

| 項目 | 仕様 |
|------|------|
| 既定ディレクトリ | `$KOMPOX_DIR/logs/` |
| 形式 | JSON Lines (機械処理可能) |
| レベル | INFO 以上 |
| ファイル命名 | `kompoxops-YYYYMMDD-HHMMSS-sss.log` |
| タイムゾーン | UTC |
| 保持期間 | 7 日間 (既定) |

ファイル名の例: `kompoxops-20251213-095105-123.log` (2025年12月13日 09:51:05.123 UTC)

各 CLI 呼び出しごとに新しいログファイルを作成する。これにより並列実行時のログ混在を防止する。

### エラー終了時の動作

異常終了時、CLI は人間が読める形式のエラーメッセージを stderr に出力し、ログファイルのフルパスを含める:

```
Error: deployment failed: manifest validation error
See log file for details: /path/to/project/.kompox/logs/kompoxops-20251213-095105-123.log
```

これにより、verbose モードを有効にしなくても詳細トレースを参照できる。

### ログファイルの保持

- 保持期間を超えたファイルは CLI 起動時に削除される
- 実行ごとにユニークなファイルを作成 (単一実行内でのローテーションはなし)
- ミリ秒精度のタイムスタンプにより並列実行時も安全

## ログフォーマット

### フォーマットオプション

`--log-format` フラグまたは `KOMPOX_LOG_FORMAT` 環境変数で指定:

- `json` (既定): 機械処理に適した JSON Lines 形式
- `human`: 人間が読みやすいテキスト形式

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

構造化ログの `msg` 属性に統一されたメッセージシンボル文字列を設定する。このシンボルにより、 Event (アプリイベント)、Span (長時間操作)、Step (短時間操作) を区別し、grep などでの検索を容易にする。

### msg 属性の形式

`msg` 属性の値は以下のいずれかの形式をとる:

| 形式 | 意味 | 用途 |
|------|------|------|
| `<msgSym>` | Event (任意の記録) | アプリケーションイベントの記録 |
| `<msgSym>/S` | Span 開始 | 長時間操作の開始 |
| `<msgSym>/EOK` | Span 終了 (成功) | 長時間操作の成功終了 |
| `<msgSym>/EFAIL` | Span 終了 (失敗) | 長時間操作の失敗終了 |
| `<msgSym>/s` | Step 開始 | 短時間操作の開始 |
| `<msgSym>/eok` | Step 終了 (成功) | 短時間操作の成功終了 |
| `<msgSym>/efail` | Step 終了 (失敗) | 短時間操作の失敗終了 |

`<msgSym>` 構文規則

- **使用禁止文字**: 半角スペース (` `) とスラッシュ (`/`)
- **階層の区切り**: コロン (`:`) を使用
- **任意の文章**: `msg` には定型シンボルのみを設定し、任意の文章は `desc` 属性に記録する

各サフィックスの詳細とログレベル、必須属性、省略可否:

| サフィックス | 意味 | レベル | 必須属性 | 備考 |
|------------|------|--------|---------|---------|
| (なし) | Event 記録 | 任意 | - ||
| `/S` | Span 開始 | `DEBUG` / `INFO` | - ||
| `/EOK` | Span 成功終了 | `DEBUG` / `INFO` | `elapsed` | 先行 `/S` が必要 |
| `/EFAIL` | Span 失敗終了 | `DEBUG` / `INFO` | `err`, `elapsed` | 先行 `/S` が必要 |
| `/s` | Step 開始 | `DEBUG` / `INFO` | - ||
| `/eok` | Step 成功終了 | `DEBUG` / `INFO` | - | 先行 `/s` が不要 |
| `/efail` | Step 失敗終了 | `DEBUG` / `INFO` | `err` | 先行 `/s` が不要 |

### ログレベルの使い分け

#### Event ログパターン

- **使用可能なレベル**: `DEBUG` / `INFO` / `WARN` / `ERROR`
- Event はアプリケーションタスク視点の重要度を判断する
- 例:
  - `INFO`: 正常な進行状況
  - `WARN`: ユーザーに注意を促すべき状況 (タスクは継続可能)
  - `ERROR`: タスクが失敗した

#### Span および Step ログパターン

- **使用可能なレベル**: `DEBUG` または `INFO` のみ
- Span および Step は操作の機械的なトレースであり、アプリケーションタスク視点の判断を含まない
- `/EFAIL` や `/efail` でも `DEBUG` または `INFO` を使用する
  - 冪等性の観点から API エラーの握りつぶしなどは頻繁に発生する。リソース重複作成や非存在削除などの「失敗」をすべて `WARN` や `ERROR` でログ出力するとノイズになる
  - アプリケーションタスク視点での「重要度」の提示は Event パターンで行う

### Event ログパターン

サフィックスなしの `<msgSym>` は Event ログパターンとしてアプリケーションイベントの記録に使用。
Span や Step の中間記録、状態遷移、サマリー情報、判断結果などに適している。
文字種制限のない任意の文章を記録する場合は `desc` 属性を使用する。

#### 出力例

```
2025/10/24 09:51:11 INFO UC:app.destroy:DeletingSelector ns=basic selector=app=basic
2025/10/24 09:51:12 INFO UC:app.destroy:DeletingNamespace ns=basic
2025/10/24 09:51:14 INFO UC:app.destroy:Completed ns=basic deleted=10 nsDeleted=1
```

### Span ログパターン

長時間実行される操作や、開始・終了を明示的に記録したい操作に使用。

#### 属性

- **開始** (`/S`):
  - `level`: `DEBUG` または `INFO`
  - `runId`, `cmd`, `resourceId` などのトレース属性
- **成功終了** (`/EOK`):
  - `level`: `DEBUG` または `INFO`
  - `err`: `""` (空文字列、省略可能)
  - `elapsed`: 開始からの経過秒数 (float64、例: `8.54`)
  - トレース属性は開始時と一致
- **失敗終了** (`/EFAIL`):
  - `level`: `DEBUG` または `INFO`
  - `err`: エラーメッセージの最初の32文字 (超過時は `...` を付与)
  - `elapsed`: 開始からの経過秒数
  - トレース属性は開始時と一致

#### 出力例

単純な出力例:
```
2025/10/24 09:51:05 INFO CMD:cluster.install/S runId=0t4mrd528u3s resourceId=/ws/w1/prv/p1/cls/c1
2025/10/24 09:51:40 INFO CMD:cluster.install/EOK runId=0t4mrd528u3s resourceId=/ws/w1/prv/p1/cls/c1 err="" elapsed=34.67
```

ネストした出力例 (トレース属性の継承):
```
2025/10/24 09:51:05 INFO CMD:app.deploy/S runId=0t4mrd528u3s cmd=app.deploy resourceId=/ws/w1/prv/p1/cls/c1/app/a1
2025/10/24 09:51:06 INFO AKS:ClusterProvision/S runId=0t4mrd528u3s cmd=app.deploy resourceId=/ws/w1/prv/p1/cls/c1/app/a1
2025/10/24 09:51:18 INFO AKS:ClusterProvision/EFAIL runId=0t4mrd528u3s cmd=app.deploy resourceId=/ws/w1/prv/p1/cls/c1/app/a1 err="resource group not found: rg-foo" elapsed=12.34
2025/10/24 09:51:19 INFO CMD:app.deploy/EFAIL runId=0t4mrd528u3s cmd=app.deploy resourceId=/ws/w1/prv/p1/cls/c1/app/a1 err="AKS operation failed: resourc..." elapsed=14.2
2025/10/24 09:51:19 ERROR CMD:app.deploy:Failed runId=0t4mrd528u3s cmd=app.deploy resourceId=/ws/w1/prv/p1/cls/c1/app/a1 err="FAILED: AKS operation failed: resource group not found: rg-foo"
```

注: すべての Span (`/S`, `/EOK`, `/EFAIL`) は `INFO` レベルで機械的に記録される。Span の `/EFAIL` では `err` を32文字で省略する。アプリケーションとしての失敗判断は最後の Event ログ (`CMD:app.deploy:Failed`) で `ERROR` レベルで出力され、Span から返されたエラーメッセージを省略なしで `err` 属性に記録する (プレフィックス `FAILED:` を付与)。

### Step ログパターン

短時間の操作や、成功が通常パスで失敗のみ注目すべき操作に使用。

開始・成功終了・失敗終了の 3 種類があり、単独でも組み合わせても使用可能。

#### 属性

- **開始** (`/s`):
  - 出力タイミング: **操作実行前**
  - `level`: `DEBUG` または `INFO`
  - `runId`, `cmd`, `resourceId` などのトレース属性
- **成功終了** (`/eok`):
  - 出力タイミング: **操作実行後**
  - `level`: `DEBUG` または `INFO`
  - `err`: `""` (空文字列、省略可能)
  - 先行の開始ログ `/s` が存在する場合はトレース属性を一致させる
- **失敗終了** (`/efail`):
  - 出力タイミング: **操作実行後**
  - `level`: `DEBUG` または `INFO`
  - `err`: エラーメッセージの最初の32文字 (超過時は `...` を付与)
  - 先行の開始ログ `/s` が存在する場合はトレース属性を一致させる

#### ログ出力パターン

| 操作結果 | ログ出力 | 説明 |
|-|-|-|
| 成功 | `/s` | 開始のみ出力 (`/efail` やエラー終了が続かなければ成功とみなす) |
| 成功 | `/s` →  `/eok` | 開始と成功終了を出力 |
| 成功 | `/eok` | 成功終了のみ出力 |
| 失敗 | `/s` → `/efail` | 開始と失敗終了を出力 |
| 失敗 | `/efail` | 失敗終了のみ出力 |

#### 出力例

```
2025/10/24 09:51:11 INFO KubeClient:Apply/s ns=k4x-app kind=Namespace name=k4x-4p4y0a-basic-e126hy
2025/10/24 09:51:12 INFO KubeClient:Apply/s ns=ingress-k4x-4p4y0a kind=ServiceAccount name=traefik
2025/10/24 09:51:13 INFO KubeClient:Apply/s ns=k4x-app kind=Secret name=db-creds
2025/10/24 09:51:13 INFO KubeClient:Apply/efail ns=k4x-app kind=Secret name=db-creds err="already exists"
2025/10/24 09:51:14 INFO AKS:RoleKV/s principalId=abc123 scope=/subscriptions/.../secrets/cert1
```

注: `KubeClient:Apply/efail` は `INFO` レベル。リソースの重複作成は冪等性の観点で正常なパス。

## コマンド実行ログ

### CMD 開始・終了ログ

CLI の各コマンド実行は開始ログと終了ログで囲まれる。これらは `runId` を含み、コマンド実行全体のトレースを可能にする。

**開始ログ**

```json
{"time":"2025-12-13T10:06:21Z","level":"INFO","msg":"CMD","runId":"0t77del56dte","args":["kompoxops","app","deploy"]}
```

| 属性 | 説明 |
|------|------|
| `msg` | `CMD` (固定) |
| `runId` | このコマンド実行の一意識別子 |
| `args` | コマンドライン引数の配列 (`os.Args`) |

**終了ログ**

```json
{"time":"2025-12-13T10:06:30Z","level":"INFO","msg":"CMD","runId":"0t77del56dte","exitCode":0}
```

| 属性 | 説明 |
|------|------|
| `msg` | `CMD` (固定) |
| `runId` | 開始ログと同じ識別子 |
| `exitCode` | 終了コード (0: 成功、1: 失敗) |

### ExitCodeError と終了コード伝播

サブプロセスを実行するコマンド (例: `kompoxops app tunnel`) では、サブプロセスの終了コードを CLI の終了コードとして伝播する必要がある。この場合、`ExitCodeError` 型を使用する。

**ExitCodeError の特性**

- サブプロセスの終了コードを格納するエラー型
- コマンド自体の失敗ではなく、終了コードの伝播手段
- Span パターンでは **EOK** として扱う (コマンドは正常に完了した)
- `exitCode` フィールドをログに出力

**Span 終了ログでの扱い**

```json
{"time":"...","level":"INFO","msg":"CMD:app.tunnel/EOK","resourceId":"...","err":"","exitCode":42,"elapsed":0.5}
```

| 属性 | 説明 |
|------|------|
| `msg` | `/EOK` (成功終了) |
| `err` | `""` (空文字列) |
| `exitCode` | サブプロセスの終了コード |

注: `ExitCodeError` でない通常の成功終了では `exitCode` フィールドは出力されない。

### runId の生成と伝搬

`runId` は CLI 起動時に `PersistentPreRunE` で一度だけ生成され、ロガーに付与される。以降のすべてのログ出力はこの `runId` を継承する。

```go
// PersistentPreRunE 内
runID, _ := naming.NewCompactID()
logger = logger.With("runId", runID)
logger.Info(ctx, "CMD", "args", os.Args)
ctx = logging.WithLogger(ctx, logger)
```

これにより:
- 1 回のコマンド実行内のすべてのログが同じ `runId` を持つ
- `withCmdRunLogger` や下位レイヤーのロガーは `runId` を自動的に継承する
- ログファイル内で特定のコマンド実行に関連するログを `runId` でフィルタリングできる

## 実装例

### Span ログパターン: withCmdRunLogger

[cmd/kompoxops/logging.go] の `withCmdRunLogger` 関数は Span パターンを実装している。

```go
func withCmdRunLogger(ctx context.Context, operation, resourceID string) (context.Context, func(err error))
```

この関数では `resourceId` をコンテキストに追加したロガーとクリーンアップ関数を返す。
クリーンアップ関数には `ExitCodeError` の特別処理も含まれている。

withCmdRunLogger 使用法:

```go
func cmdFoo(ctx context.Context, resourceID string) (err error) {
  ctx, cleanup := withCmdRunLogger(ctx, "foo", resourceID)
  defer func() { cleanup(err) }()
  // foo 処理
  if err = fooBar(ctx); err != nil {
    return err
  }
  return nil
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

- [K4x-ADR-016] - CLI ロギング戦略
- [2025-10-24-logging.ja.md] - 実装タスク
- [cmd/kompoxops/logging.go] - コマンドレイヤー実装
- [adapters/drivers/provider/aks/logging.go] - ドライバーレイヤー実装
- [internal/logging] - ロギングパッケージ

[K4x-ADR-016]: ../adr/K4x-ADR-016.md
[2025-10-24-logging.ja.md]: ../../_dev/tasks/2025-10-24-logging.ja.md
[cmd/kompoxops/logging.go]: ../../cmd/kompoxops/logging.go
[adapters/drivers/provider/aks/logging.go]: ../../adapters/drivers/provider/aks/logging.go
[internal/logging]: ../../internal/logging
