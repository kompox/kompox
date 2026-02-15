---
id: 20260215d-app-kubectl-command
title: "kompoxops app kubectl コマンド実装"
status: done
updated: 2026-02-15T18:03:18Z
language: ja
owner: yaegashi
adrs: []
plans: []
supersedes: []
supersededBy:
---
# Task: kompoxops app kubectl コマンド実装

## 目的

- `kompoxops app deploy` 後のクラスタ調査を簡素化するため、`kompoxops app kubectl` を追加する。
- App コンテキスト(対象 App / namespace / kubeconfig)の決定を CLI 側で自動化し、`kubectl` 実行の手間を減らす。

## スコープ/非スコープ

- In:
  - `kompoxops app kubectl -- <kubectl args...>` サブコマンドを追加する。
  - `KUBECONFIG=$KOMPOX_DIR/kubeconfig` を使用して `kubectl` を実行する。
  - context 名 `<appName>-<inHASH>` を使って App 単位の context を管理する。
  - 必要時のみ `kompoxops cluster kubeconfig --merge` を実行し、`--refresh-kubeconfig` で強制更新できるようにする。
- Out:
  - `kompoxops cluster kubeconfig` 本体の機能拡張。
  - `kubectl` の引数・出力フォーマットの独自拡張。

## 仕様サマリ

- コマンド形式:
  - `kompoxops app kubectl -- <kubectl args...>` (`--` は必須)
- context 名:
  - `<appName>-<inHASH>` (`app.kubernetes.io/instance` と同値)
- kubeconfig:
  - 常に `$KOMPOX_DIR/kubeconfig` を使用
- default namespace:
  - `app kubectl` の default namespace は常に App namespace に固定する
  - App namespace 以外を対象にする場合は `kubectl` 引数で `-n <namespace>` を明示する
- merge 実行のスキップ条件:
  - 既存 kubeconfig に対象 context が存在し、かつその context の default namespace が対象 App namespace と一致する場合は skip
- merge 実行:
  - skip できない場合、または `--refresh-kubeconfig` 指定時は
  - `kompoxops cluster --cluster-id <clusterFQN> kubeconfig --merge --kubeconfig $KOMPOX_DIR/kubeconfig --context <appName>-<inHASH> --namespace <appNamespace> --force`
  - short option: `-R` (`--refresh-kubeconfig` のエイリアス)
- `--set-current`:
  - 使用しない
- ログ出力:
  - `app kubectl` の診断ログは `msg="CMD:app.kubectl"` を使用する
  - 任意の診断情報は `desc` 属性に記録する
- 失敗時挙動:
  - `cluster kubeconfig` / `kubectl` いずれも失敗時はそのまま失敗終了
  - 終了コードはそのまま返す
- `--namespace`:
  - `app kubectl` では提供しない（廃止）

## 計画 (チェックリスト)

- [x] `cmd/kompoxops` に `app kubectl` サブコマンドを追加し、`--` 必須の引数パースを実装する。
- [x] App 解決処理から `<appName>-<inHASH>` と App namespace を導出する。
- [x] kubeconfig 読み取りで skip 判定 (context 存在 + namespace 一致) を実装する。
- [x] `--refresh-kubeconfig` フラグを追加し、強制 merge を実装する。
  - [x] short option `-R` を追加する。
- [x] `kubectl --context <appName>-<inHASH>` 実行と exit code 透過を実装する。
- [x] `app kubectl` の診断ログを `msg=CMD:app.kubectl` / `desc=<detail>` へ統一する。
- [x] `design/v1/Kompox-CLI.ja.md` に `app kubectl` の仕様・使用例を追記する。
- [x] `cmd/kompoxops` のテストを追加し、主要分岐 (skip/refresh/failure) を検証する。
  - [x] kubeconfig skip 判定のユニットテストを追加する。
  - [x] refresh/failure 分岐のユニットテストを追加する。

## テスト

- ユニット:
  - `cmd/kompoxops` の新規/更新テストで以下を検証
    - `--` 必須
    - skip 判定
    - `--refresh-kubeconfig` の強制実行
    - 失敗時 exit code 透過
- スモーク:
  - 初回実行で merge 実行、2回目で skip、`--refresh-kubeconfig` で再実行を確認

## 受け入れ条件

- `kompoxops app kubectl -- get pod -o wide` が実行できる。
- context 名が `<appName>-<inHASH>` で作成/更新される。
- context+namespace 一致時は `cluster kubeconfig --merge` を実行しない。
- `--refresh-kubeconfig` 指定時は一致していても merge を実行する。
- `cluster kubeconfig` 起動時は必ず `--cluster-id <clusterFQN>` を付与し、対象クラスタの取り違えを防ぐ。
- `--set-current` を変更しない。
- `kubectl` 失敗時に同じ終了コードを返す。
- `app kubectl` に `--namespace` フラグは存在しない。
- `app kubectl` の default namespace は常に App namespace である。

## メモ

- リスク:
  - kubeconfig 解析と context 判定ロジックの差異により、意図せず merge が走る可能性がある。
- フォローアップ:
  - 必要であれば将来 `app kubectl` 向けに dry-run 相当の診断オプションを追加検討する。

## 進捗

- 2026-02-15T12:41:59Z タスクファイル作成
- 2026-02-15T17:25:00Z 追加仕様と実装状況を反映
  - `app kubectl` 実装完了（`--` 必須、context/namespace 自動導出、skip 判定、`--refresh-kubeconfig`）
  - short option `-R` を追加
  - ログ仕様を `msg=CMD:app.kubectl` + `desc` 属性へ統一
  - `cmd/kompoxops/cmd_app_kubectl_test.go` を追加（skip 判定の検証）
  - 未完了: CLI ドキュメント追記、refresh/failure 分岐の追加テスト
- 2026-02-15T17:30:11Z 仕様補足を追記
  - `cluster kubeconfig` 起動コマンド例に `--cluster-id <clusterFQN>` を追加
  - 受け入れ条件に `--cluster-id` 必須を明記
- 2026-02-15T17:39:36Z 残タスク完了
  - `design/v1/Kompox-CLI.ja.md` に `app kubectl` の仕様・オプション・挙動・使用例を追記
  - `cmd/kompoxops/cmd_app_kubectl_test.go` に refresh/skip 判定と failure(終了コード透過)のユニットテストを追加
  - タスク状態を `done` に更新
- 2026-02-15T18:03:18Z 安全性方針に合わせて仕様を更新
  - `app kubectl` の `--namespace` を廃止
  - default namespace を App namespace 固定に明記
  - App namespace 以外は `kubectl -n <namespace>` 明示運用に統一

## 参照

- [Kompox-CLI]
- [Kompox-KubeConverter]

[Kompox-CLI]: ../../../../v1/Kompox-CLI.ja.md
[Kompox-KubeConverter]: ../../../../v1/Kompox-KubeConverter.ja.md
