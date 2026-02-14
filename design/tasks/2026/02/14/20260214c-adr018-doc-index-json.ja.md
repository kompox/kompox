---
id: 20260214c-adr018-doc-index-json
title: K4x-ADR-018 実装（design index JSON 生成）
status: active
updated: 2026-02-14T17:36:29Z
language: ja
owner: yaegashi
plans: []
---
# Task: K4x-ADR-018 実装（design index JSON 生成）

## 目的

- [K4x-ADR-018] で決定した「front matter を source of truth とした dual index（Markdown + JSON）運用」を、既存 `design/gen` に実装する。
- 既存の Markdown index 生成を維持しつつ、機械可読な `design/index.json` と `design/<category>/index.json` を生成できる状態にする。
- 併せて `_dev/tasks/*.ja.md` を収集対象に含め、`_dev/tasks/index.json` を生成する。

## スコープ/非スコープ

- In:
  - `design/gen/main.go` の出力を拡張し、カテゴリ別 JSON と集約 JSON を生成
  - `_dev/tasks/*.ja.md` を JSON 収集対象として扱う
  - `_dev/tasks/index.json` を生成する（旧タスクの機械可読インデックス）
  - JSON スキーマ（最小必須フィールド）を定義し、生成を deterministic にする
  - `make gen-index` 実行で Markdown/JSON の両方が更新されるようにする
  - 生成仕様を [design/gen/README.md] に追記
- Out:
  - front matter 自体の項目追加・既存ドキュメントの大規模書き換え
  - JSON コンシューマ（外部ツール）側の実装
  - CI の全面再設計（必要最小限の追記は可）

## 仕様サマリ

- source of truth は各ドキュメントの front matter。
- 出力物:
  - `design/<category>/index.json`
  - `design/index.json`
  - `_dev/tasks/index.json`
- 最小フィールド:
  - `id`, `title`, `status`, `updated`, `language`, `version`, `category`, `relPath`, `references`
- 既存の Markdown index 生成仕様（対象ファイル、除外ルール、並び順）と整合させる。
- `_dev/tasks` 配下は `.ja.md` を対象にし、`_dev/tasks/README.md` の有無に依存せず JSON を生成できるようにする。

## 計画 (チェックリスト)

- [ ] 現行 `design/gen/main.go` のデータモデルと出力フローを整理
- [ ] category index JSON 出力を実装
- [ ] hub 集約 JSON 出力を実装
- [ ] `_dev/tasks/*.ja.md` 収集と `_dev/tasks/index.json` 出力を実装
- [ ] 生成順序・キー順・日付整形を確認し deterministic 性を担保
- [ ] [design/gen/README.md] に JSON 出力仕様を反映
- [ ] `make gen-index` と関連テスト（必要最小限）を実行

## テスト

- ユニット:
  - `design/gen` の JSON 出力に対するスナップショット/期待値比較（必要に応じて追加）
- スモーク:
  - `make gen-index` 成功
  - 生成後に `git diff` が期待どおり（余計な差分なし）

## 受け入れ条件

- `make gen-index` 1回で Markdown index と JSON index が両方更新される。
- JSON 出力は前述の最小フィールドを満たし、同一入力で同一出力になる。
- 既存の `design/<category>/README.md` 生成結果に意図しない破壊的変更がない。
- `_dev/tasks/index.json` が生成され、`_dev/tasks/*.ja.md` のメタデータを参照できる。
- 仕様文書（[design/gen/README.md]）が実装内容と一致する。

## メモ

- リスク:
  - references 抽出方法を過度に複雑化すると生成器の責務が肥大化する。
- 方針:
  - 初期実装では「利用価値が高い最小 JSON」に絞り、拡張は後方互換で行う。

## 進捗

- 2026-02-14: タスク作成
- 2026-02-14: 要件更新（`_dev/tasks/*.ja.md` 収集、`_dev/tasks/index.json` 生成）

## 参照

- [K4x-ADR-018]
- [design/gen/README.md]
- [design/tasks/README.md]

[K4x-ADR-018]: ../../../../adr/K4x-ADR-018.md
[design/gen/README.md]: ../../../../gen/README.md
[design/tasks/README.md]: ../../README.md
