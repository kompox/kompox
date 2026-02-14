---
id: 20260214a-new-design-docs
title: Design doc indexes 再設計
status: done
updated: 2026-02-14
language: ja
owner:
---
# Task: Design doc indexes 再設計

## 目的

- `design/` 配下のドキュメントを adr/plans/tasks/v1/v2/pub に分割した上で、各ディレクトリに英語のインデックス `README.md` を生成できるようにする。
- ルートの `design/README.md` は静的なハブとして、各インデックスへのリンクのみを掲載する。
- `README.ja.md` は今回以降、生成しない。

## スコープ / 非スコープ

- In:
  - インデックス生成仕様の確定と実装(ジェネレーター改修)
  - `make gen-index` から新しいインデックス生成を実行できること
  - 各インデックス `README.md` を英語で出力すること
  - `README.ja.md` が生成されないこと
- Out:
  - 既存ドキュメントの移設(大量の move/rename)
  - 既存リンクの一括修正(別タスク)
  - 既存 `_dev/tasks/` の廃止/移設の実作業(別タスク)

## 仕様サマリ

### 対象パス

- ADR: `design/adr/K4x-ADR-NNN.md`
- Plans: `design/plans/2026/K4x-Plan-2026aa.ja.md`
- Tasks: `design/tasks/2026/02/14/20260214a-slug.ja.md`
- Specs:
  - `design/v1/Kompox-slug.ja.md`
  - `design/v2/Kompox-slug.ja.md`
- Public: `design/pub/Kompox-Pub-slug.md`

### インデックスファイル（README.md）

- 生成するインデックス
  - `design/adr/README.md`
  - `design/plans/README.md`
  - `design/tasks/README.md`
  - `design/v1/README.md`
  - `design/v2/README.md`
  - `design/pub/README.md`
- 静的なインデックス
  - `design/README.md` (各インデックスへのリンクのみ)
  - `design/gen/README.md` (インデックス生成仕様)

### テンプレートファイル

- テンプレートファイルは `design/gen/<category>/README.md` に配置し、カテゴリごとに独立してカスタマイズできるようにする。
- 配置パス:
  - `design/gen/adr/README.md`
  - `design/gen/plans/README.md`
  - `design/gen/tasks/README.md`
  - `design/gen/v1/README.md`
  - `design/gen/v2/README.md`
  - `design/gen/pub/README.md`

### design/gen/main.go の動作

- `design/gen/*/README.md` の存在をスキャンし、処理すべきカテゴリ(=出力先ディレクトリ)とテンプレート(=入力)のリストを得る。
- 各カテゴリについて、出力先の `design/<category>/README.md` を生成する。
- 生成時は、対応する `design/<category>/` 配下の `*.md` を再帰的にスキャンしてエントリを収集する。
- `design/README.md` は生成しない(静的ファイル)。
- 完全な仕様は `design/gen/README.md` に記載。

### 言語

- 生成するインデックスは `README.md` のみ。
- インデックス本文は英語で固定する。
  - 掲載するドキュメントのタイトルは front matter `title` をそのまま表示してよい(日本語タイトルを含む可能性を許容)。

### 収集範囲

- それぞれのインデックスは、対応ディレクトリ配下の `**/*.md` を再帰的に収集する。
- 生成先の `README.md` と `README.*` は除外する。
- `index.*` は除外する。
- `GUIDE.md` と `GUIDE.ja.md` は除外する。
- front matter の `version: meta` は除外する。

### ソート

- `design/adr/README.md`: ADR番号(NNN)の昇順に統一する。
- `design/plans/README.md`: ファイル名(=ID)の辞書順でよい。
- `design/tasks/README.md`: パス順(YYYY/MM/DD) + ファイル名順(20260214a, 20260214b, ...)。
- `design/v1/README.md`, `design/v2/README.md`, `design/pub/README.md`: ファイル名の辞書順でよい。

## 計画(チェックリスト)

- [x] 現行の `design/gen` の入出力仕様を整理する
- [x] `design/README.md` (静的 hub) の内容を確定する
- [x] `design/adr|plans|tasks|v1|v2|pub` それぞれの README.md 出力を実装する
- [x] `README.ja.md` 生成停止(既存生成がある場合は削除)を実装する
- [x] `make gen-index` から新仕様のインデックス生成を呼び出す
- [x] スモークとして `make gen-index` を実行し、生成物が仕様通りであることを確認する

## 追加実施事項(当初チェックリスト外)

- [x] 旧インデックス `design/adr/README.ja.md` を削除し、`design/` 配下に `README.ja.md` が残存しない状態にする
- [x] `design/adr/GUIDE.md` と `design/adr/GUIDE.ja.md` を追加し、インデックス収集対象から除外する
- [x] 各カテゴリのテンプレートで、ID 列をリンク化し Link 列を廃止する
- [x] インデックス生成仕様を `design/gen/README.md` に英語で記載する
- [x] `design/README.md` を静的ファイルに切り替え、ジェネレーターが上書きしないようにする

## テスト

- スモーク:
  - `make gen-index` が成功する
  - 各カテゴリの `README.md` が生成される
  - `design/README.md` が `make gen-index` で上書きされない
  - `design/**/README.ja.md` が生成されない

## 受け入れ条件

- `design/README.md` が静的ハブとして存在し、各カテゴリインデックスへのリンクのみを持つ。
- `design/adr|plans|tasks|v1|v2|pub/README.md` が生成される。
- 生成されるインデックスは英語である。
- `README.ja.md` が生成されない。

## メモ

- 既存の `design/README.md` / `design/README.ja.md` と互換がなくなるため、移行期はリンク切れが発生し得る。移設と参照修正は別タスクで段階的に行う。

## 進捗

- 2026-02-14: タスク作成
- 2026-02-14: `design/gen/main.go` をカテゴリテンプレート走査方式に改修
- 2026-02-14: `design/gen/{adr,plans,tasks,v1,v2,pub}/README.md` テンプレートを追加
- 2026-02-14: `make gen-index` 実行で `design/README.md` と各カテゴリ `README.md` 生成を確認
- 2026-02-14: `design/` 配下の `README.ja.md` 残存を削除し、生成されないことを確認
- 2026-02-14: `design/gen/README.md` にインデックス生成仕様を英語で記載
- 2026-02-14: `design/README.md` を静的ハブに切り替え、ジェネレーターが上書きしないことを確認

## 参考

- [design/gen/main.go]
- [design/gen/README.md]
- [design/README.md]
- [K4x-ADR-017]

[design/gen/main.go]: ../../../../gen/main.go
[design/gen/README.md]: ../../../../gen/README.md
[design/README.md]: ../../../../README.md
[K4x-ADR-017]: ../../../../adr/K4x-ADR-017.md
