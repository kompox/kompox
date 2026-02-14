# Design Docs (v1) ガイド

English: [GUIDE.md]

このディレクトリには Kompox v1 (現行) の design docs を置きます。タスクよりも長く参照される文書であり、実装完了後も仕様・契約の参照として有用であることを重視します。

## 対象範囲と目的

design docs (v1) では次を扱います:
- 期待される挙動や契約 (CLI UX, config 形式, domain model, driver interface)
- 非自明な制約、トレードオフ
- 実装・レビューに必要な設計詳細
- ADR/Tasks への参照リンク (重複を避ける)

design docs (v1) で避けること:
- 日々の進捗ログ (tasks に書く)
- 大量のコード貼り付けやベンダードキュメントの転載

## 言語

design docs (v1) は日本語/英語のどちらでもよいです。

- 日本語: `.ja.md` / `language: ja`
- 英語: `.md` (言語サフィックスなし) / `language: en`

## ファイル名と ID

- ディレクトリ: `design/v1/`
- ファイル名 (および id): `Kompox-<short-slug>.<lang>.md`
  - `<short-slug>`: 既存文書に合わせて短い slug を付ける
  - `<lang>`: 日本語は `.ja.md`、英語は `.md`

ルール: front matter の `id` は、ファイル名から拡張子を除いた stem と一致しなければなりません。

## front matter (YAML) スキーマ

必須
- id (string): 一意な doc id (ファイル名 stem と一致)
- title (string): タイトル
- version (string): `v1`
- status (enum): `draft | synced | out-of-sync | archived`
- updated (timestamp): UTC の ISO 8601 `YYYY-MM-DDTHH:MM:SSZ`
- language (enum): `ja | en`

## status の目安

- draft: 検討中/未実装
- synced: 実装済みで、この文書が現状を反映している
- out-of-sync: 実装はあるが、この文書の更新が必要
- archived: 履歴として保持するが、メンテ対象外

## 記述ガイド

- 仕様・契約・不変条件・例を優先し、内部実装メモに寄りすぎない
- 見出し/表を活用し、後から探しやすい構成にする
- 判断が ADR にある場合は ADR へリンクし、本文の重複を避ける

## テンプレート

```markdown
---
id: Kompox-ShortSlug
title: 短いタイトル
version: v1
status: draft
updated: 2026-02-14T00:00:00Z
language: ja
---

# 短いタイトル

## Overview

- ...

## スコープ / 非スコープ

- In: ...
- Out: ...

## 設計

- ...

## インターフェイス / 契約

- ...

## 制約

- ...

## Migration notes

- ...

## 参考

- [design/v1/README.md]

[design/v1/README.md]: ./README.md
```

## インデックス

- [README.md] は生成物です。次で再生成できます:
- `make gen-index`

## 参考

- [GUIDE.md]
- [README.md]

[GUIDE.md]: ./GUIDE.md
[README.md]: ./README.md
