---
id: 20260215d-app-kubectl-command
title: "kompoxops app kubectl コマンド実装"
status: active
updated: 2026-02-15T12:41:59Z
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
- merge 実行のスキップ条件:
  - 既存 kubeconfig に対象 context が存在し、かつその context の default namespace が対象 App namespace と一致する場合は skip
- merge 実行:
  - skip できない場合、または `--refresh-kubeconfig` 指定時は
  - `kompoxops cluster kubeconfig --merge --kubeconfig $KOMPOX_DIR/kubeconfig --context <appName>-<inHASH> --namespace <appNamespace> --force`
- `--set-current`:
  - 使用しない
- 失敗時挙動:
  - `cluster kubeconfig` / `kubectl` いずれも失敗時はそのまま失敗終了
  - 終了コードはそのまま返す

## 計画 (チェックリスト)

- [ ] `cmd/kompoxops` に `app kubectl` サブコマンドを追加し、`--` 必須の引数パースを実装する。
- [ ] App 解決処理から `<appName>-<inHASH>` と App namespace を導出する。
- [ ] kubeconfig 読み取りで skip 判定 (context 存在 + namespace 一致) を実装する。
- [ ] `--refresh-kubeconfig` フラグを追加し、強制 merge を実装する。
- [ ] `kubectl --context <appName>-<inHASH>` 実行と exit code 透過を実装する。
- [ ] `design/v1/Kompox-CLI.ja.md` に `app kubectl` の仕様・使用例を追記する。
- [ ] `cmd/kompoxops` のテストを追加し、主要分岐 (skip/refresh/failure) を検証する。

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
- `--set-current` を変更しない。
- `kubectl` 失敗時に同じ終了コードを返す。

## メモ

- リスク:
  - kubeconfig 解析と context 判定ロジックの差異により、意図せず merge が走る可能性がある。
- フォローアップ:
  - 必要であれば将来 `app kubectl` 向けに dry-run 相当の診断オプションを追加検討する。

## 進捗

- 2026-02-15T12:41:59Z タスクファイル作成

## 参照

- [Kompox-CLI]
- [Kompox-KubeConverter]

[Kompox-CLI]: ../../../../v1/Kompox-CLI.ja.md
[Kompox-KubeConverter]: ../../../../v1/Kompox-KubeConverter.ja.md
