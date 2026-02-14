# Plans (design/plans)

English: [GUIDE.md]

このディレクトリには、複数の design docs の更新を伴う大規模な改修を行う際に、各 design docs の差分や更新の段取りを計画するための plan files を置きます。

計画が完了し design docs が更新されたあとも、plan files は参考のため残します。

事例:
- [2026aa-kompox-box-update]

## 目的と使いどころ

plan files は次を目的とします:
- 目的/非目的を明確化する
- 影響を受ける design docs と差分の意図を俯瞰できるようにする
- design docs を更新する順序 (段取り) を定義する
- 依存関係、リスク、未解決点を記録する
- ADR/specs/tasks へリンクし、重複を避ける

plan files で避けること:
- 実装手順のチェックリスト化 (タスクファイルを使う)
- 仕様本文の丸写し (必要なら参照リンクで示す)

## 言語

Plans は日本語/英語のどちらでもよいです。拡張子で言語を示します:
- `.ja.md`: 日本語
- `.en.md`: 英語

## ファイル名と ID

- ディレクトリ: `design/plans/<year>/`
- ファイル名 (および id): `YYYYaa-<short-slug>.<lang>.md`
  - `YYYY`: 年
  - `aa`, `ab`, ...: 同一年の識別子
  - `<short-slug>`: 短い slug (kebab-case)

ルール: front matter の `id` は、ファイル名から拡張子を除いたものと一致しなければなりません。

## front matter (YAML) スキーマ

必須
- id (string): 一意な plan id (ファイル名 stem と一致)
- title (string): Plan のタイトル
- status (enum): `draft | active | done | canceled | superseded`
- updated (timestamp): UTC の ISO 8601 `YYYY-MM-DDTHH:MM:SSZ`
- language (enum): `ja | en`

任意
- version (string): 版ラベル (例: `v1`)
- tasks (string[]): この plan で実装するタスクの doc-id のリスト (例: `20260214a-new-design-docs`)

相互参照ルール
- plan 文書では、実装するタスクの doc-id を `tasks` に列挙する。
- 値は doc-id を使い、ファイル名拡張子は含めない。

## status の目安

- draft: 作成中
- active: 実行中の計画として参照される
- done: 完了 (参考のため保持)
- canceled: 中止 (参考のため保持)
- superseded: 新しい plan に置き換え (参考のため保持)

## 記述ガイド

- どの design docs がどう変わるかを見やすく書く
- 暗黙の推論より、明示ルールと具体例を優先する
- 互換性や移行が必要なら Migration notes に書く
- 完了後も読みやすい形を保つ
- このルールの対象は、front-matter に `id:` を持つ markdown ドキュメントのみ。
- `README.md`、`README.ja.md`、`GUIDE.md`、`GUIDE.ja.md` は対象外。

## テンプレート

```markdown
---
id: YYYYaa-short-slug
title: 短いタイトル
version: v1
status: draft
updated: 2026-02-14T00:00:00Z
language: ja
tasks: []
---

# Plan: 短いタイトル

## 目的

- ...

## 非目的

- ...

## 背景

- ...

## 対象となる design docs

- In scope:
  - design/v1/<doc>.md: ...
  - design/v2/<doc>.md: ...
  - design/adr/<adr>.md: ...
- Out of scope:
  - ...

## 差分サマリ

- Doc A: ...
- Doc B: ...

## 更新の段取り

- Step 1: ...
- Step 2: ...

## リスク/未解決点

- ...

## Migration notes

- ...

## 参照

- [design/plans/README.md]

[design/plans/README.md]: ./README.md
```

## インデックス

- Makefile のターゲットで [README.md] を再生成できます:
- `make gen-index`

## 参照

- [GUIDE.md]
- [README.md]
- [2026aa-kompox-box-update]

[GUIDE.md]: ./GUIDE.md
[README.md]: ./README.md
[2026aa-kompox-box-update]: ./2026/2026aa-kompox-box-update.ja.md
