---
id: 20260214c-adr018-doc-index-json
title: K4x-ADR-018 実装（design index JSON 生成）
status: done
updated: 2026-02-18T01:30:40Z
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
- archived タスクの `category` は `old-tasks` とする（`_dev/tasks` はパスとして扱う）。
- 最小フィールド:
  - `id`, `title`, `status`, `updated`, `language`, `version`, `category`, `relPath`, `references`
- `docs[]` は front matter を含む単一 map とし、`frontMatter` の独立フィールドは持たない。
- 全ての index JSON の最上位に `docCount` を出力する。
- Markdown index のリンクはカテゴリ相対 `./...` を維持する。
- JSON の `relPath` は repo-root 相対（`./` なし）を維持する。
- 既存の Markdown index 生成仕様（対象ファイル、除外ルール、並び順）と整合させる。
- `_dev/tasks` 配下は `.ja.md` を対象にし、`_dev/tasks/README.md` の有無に依存せず JSON を生成できるようにする。

## 計画 (チェックリスト)

- [x] 現行 `design/gen/main.go` のデータモデルと出力フローを整理
- [x] category index JSON 出力を実装
- [x] hub 集約 JSON 出力を実装
- [x] `_dev/tasks/*.ja.md` 収集と `_dev/tasks/index.json` 出力を実装
- [x] 生成順序・キー順・日付整形を確認し deterministic 性を担保
- [x] [design/gen/README.md] に JSON 出力仕様を反映
- [x] `make gen-index` と関連テスト（必要最小限）を実行

## テスト

- ユニット:
  - `design/gen` の JSON 出力に対するスナップショット/期待値比較（必要に応じて追加）
- スモーク:
  - `make gen-index` 成功
  - 生成後に `git diff` が期待どおり（余計な差分なし）

## 受け入れ条件

- `make gen-index` 1回で Markdown index と JSON index が両方更新される。
- JSON 出力は前述の最小フィールドを満たし、同一入力で同一出力になる。
- 各 `index.json` の最上位に `docCount` が出力される。
- `_dev/tasks/*.ja.md` の `docs[].category` は `old-tasks` となる。
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
- 2026-02-14: 実装開始
  - `design/gen/main.go` に category/hub JSON 生成を追加
  - `_dev/tasks/*.ja.md` 収集と `_dev/tasks/index.json` 出力を追加
  - 生成仕様を `design/gen/README.md` に反映
  - `make gen-index` 実行で生成物更新を確認
- 2026-02-14: 仕様変更反映
  - archived タスクの category を `old-tasks` に変更
  - `docs[]` を front matter 含む単一 map 出力に統一（`frontMatter` 分離を廃止）
  - 全 index JSON の最上位に `docCount` を追加
- 2026-02-14: Markdown index リンク修正
  - `design/gen/main.go` で Markdown 用 `RelPath` と JSON 用 `relPath` を分離
  - Markdown はカテゴリ相対 `./...`、JSON は repo-root 相対を維持
  - `design/pub/README.md` など壊れたリンクの復旧を確認
- 2026-02-14: 完了判定
  - 受け入れ条件を満たすことを確認し、status を `done` に更新

## 参照

- [K4x-ADR-018]
- [design/gen/README.md]
- [design/tasks/README.md]

[K4x-ADR-018]: ../../../../adr/K4x-ADR-018.md
[design/gen/README.md]: ../../../../gen/README.md
[design/tasks/README.md]: ../../../README.md
