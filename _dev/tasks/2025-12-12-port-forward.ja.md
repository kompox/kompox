---
id: 2025-12-12-port-forward
title: kompoxops app port-forward コマンド実装
status: done
updated: 2025-12-12
language: ja
owner: yaegashi
---
# Task: kompoxops app port-forward コマンド実装

## 目的

- `kompoxops app port-forward` コマンドを実装し、アプリの Pod に対するポートフォワードを提供する。
- 既存の `usecase/box/port_forward.go` および `adapters/kube/client_port_forward.go` を参考に、アプリ Pod 向けの実装を追加する。
- `--component` オプションにより `app` (既定) または `box` の Pod を選択可能にする。

## スコープ / 非スコープ

- In:
  - `usecase/app/port_forward.go` の新規実装
  - `cmd/kompoxops` への `app port-forward` サブコマンド追加
  - エイリアス `app pf` の追加
  - `--component`、`-S/--service`、`--address` オプションの実装
  - 複数ポート同時フォワードのサポート
  - ポート指定形式 (`LOCAL:REMOTE`、`PORT`、`:REMOTE`) のパース
  - Ready 状態 Pod 優先選択ロジック
  - SIGINT (Ctrl+C) による終了処理
- Out:
  - `box port-forward` サブコマンド (削除済み: `app port-forward --component=box` で代替)
  - SSH トンネリング (-L/-R) 機能 (既存 `box ssh` で対応)

## 仕様サマリ

[Kompox-CLI.ja.md] の `kompoxops app port-forward` セクションに準拠する。

### 使用法

```
kompoxops app port-forward -A <appName> [--component COMPONENT] [-S SERVICE] [--address ADDR] [LOCAL_PORT:]REMOTE_PORT...
```

### オプション

| オプション | 説明 |
|------------|------|
| `--component` | 対象の component ラベル値 (既定: `app`、`box` で Kompox Box に接続) |
| `-S, --service` | 対象の Compose service 名 (`--component=app` 時のみ有効) |
| `--address` | 待ち受けアドレス (既定: `localhost`、カンマ区切りで複数指定可) |

### ポート指定形式

| 形式 | 説明 |
|------|------|
| `8080:80` | ローカル 8080 → リモート 80 |
| `8080` | ローカル 8080 → リモート 8080 (同じポート) |
| `:80` | ローカル自動割当 → リモート 80 |

### 挙動

- アプリの Namespace 内で `app.kubernetes.io/component=<COMPONENT>` ラベルを持つ Pod を対象とする。
- Ready 状態の Pod を優先し、無ければ非終了中の Pod を選択する。
- 対象 Pod が終了すると自動的に接続が終了する。
- Ctrl+C (SIGINT) で終了できる。

## 計画 (チェックリスト)

- [x] `usecase/app/port_forward.go` を新規作成
  - [x] `PortForwardInput` 構造体定義 (AppID, Component, Service, Address, Ports)
  - [x] `PortForwardOutput` 構造体定義 (現在は空の構造体として維持)
  - [x] ポート指定文字列のパース関数 (`parsePortSpec`)
  - [x] アドレス指定のパース関数 (`splitAddresses`)
  - [x] `PortForward` メソッド実装 (Pod 選択、複数ポートフォワード設定、リトライ)
- [x] `usecase/app/types.go` に必要な型を追加 (不要だったため追加なし)
- [x] `cmd/kompoxops` への `app port-forward` サブコマンド追加
  - [x] cobra コマンド定義 (`cmd/kompoxops/cmd_app.go` に統合)
  - [x] エイリアス `pf` 登録
  - [x] フラグ定義 (`--component`, `-S/--service`, `--address`)
  - [x] ポート引数パース
  - [x] SIGINT ハンドラ実装
- [x] 単体テスト追加 (`usecase/app/port_forward_test.go`)
  - [x] ポート指定パースのテスト
  - [ ] Pod 選択ロジックのテスト (モック使用)
- [x] 統合テスト (手動確認)

## 受け入れ条件

- `kompoxops app port-forward -A <app> 8080:80` でアプリ Pod にポートフォワードできる。
- `kompoxops app pf 8080:80` (エイリアス) が動作する。
- `--component=box` 指定で Kompox Box Pod にフォワードできる。
- `-S web` 指定で特定サービスの Pod を選択できる。
- `--address 0.0.0.0` で全インターフェースにバインドできる。
- 複数ポート (`8022:22 8080:80`) を同時にフォワードできる。
- `:3000` 形式でローカルポートが自動割当される。
- Ready 状態の Pod が優先選択される。
- Ctrl+C で正常終了する。
- Pod 終了時に接続が自動終了する。

## メモ

- 既存の `usecase/box/port_forward.go` は `setupPortForward` として内部利用のみ。今回は `usecase/app` に CLI 向けの完全な実装を追加する。
- `adapters/kube/client_port_forward.go` の `PortForward` と `PortForwardMulti` を活用する。
- `--service` オプションは `--component=app` の場合のみ意味を持つ。`--component=box` では無視される。

## 進捗

- 2025-12-12: タスク作成
- 2025-12-12: `kompoxops app port-forward` を実装 (エイリアス `pf`、複数ポート、`--component`、`--service`、`--address`)
- 2025-12-12: `cmd/kompoxops/cmd_app.go` に統合し、専用ファイルは作成しない構成に変更
- 2025-12-12: 誤ったリモートポート等による `connection refused` を警告扱いとして継続 (リトライとバックオフ)
- 2025-12-12: ログを `design/v1/Kompox-Logging.ja.md` に準拠するよう調整 (msg シンボル、key 順序の安定化)
- 2025-12-12: 終了後に出る "Forwarding from ..." の出力は削除 (UseCase の戻り値に意味のあるメッセージを持たせない)
- 2025-12-12: [2025-12-12-tunnel] タスクにより `tunnel` コマンドへ発展・統合

## 参考

- [Kompox-CLI.ja.md]
- [usecase/box/port_forward.go]
- [adapters/kube/client_port_forward.go]
- [usecase/app/exec.go]
- [2025-12-12-tunnel]

[Kompox-CLI.ja.md]: ../../design/v1/Kompox-CLI.ja.md
[usecase/box/port_forward.go]: ../../usecase/box/port_forward.go
[adapters/kube/client_port_forward.go]: ../../adapters/kube/client_port_forward.go
[usecase/app/exec.go]: ../../usecase/app/exec.go
[2025-12-12-tunnel]: ./2025-12-12-tunnel.ja.md
