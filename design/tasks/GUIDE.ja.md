# メンテナー向けタスクガイド (design/tasks)

English: [GUIDE.md]

このフォルダには、メンテナーが実装に向けて動くための短いタスク文書 (実装計画、受け入れ条件、テストメモなど) を置きます。ユーザー向けドキュメントではありません。

## ここに置くもの

- 具体的な変更の実装計画 (1〜2ページ)
- 単一の成果物に紐づく受け入れ条件とテスト計画
- 設計判断 (ADR) や仕様へのリンク (必要な範囲)

## ここに置かないもの

- 正式な設計仕様 (design/v1, design/v2 など)
- ユーザードキュメント (docs/ 配下)
- 長期ロードマップ (別途 design/ 配下に置く)

## ファイル命名

- 形式: design/tasks/YYYY/MM/DD/YYYYMMDDa-short-description.ja.md
  - 例: design/tasks/2026/02/14/20260214a-new-design-docs.ja.md
- short-description は参照しやすくするため可能な限り短くします。

## ワークフロー (軽量)

1) 下記テンプレートを使って新しいタスクファイルを作る
2) 作業中は適宜更新 (status, updated, チェックリスト)
3) 完了したら status: done にしてファイルは残す (履歴のため)
   - 置き換えられた場合は status: superseded にして置換先タスクへのリンクを追加

ヒント: 判断は ADR に記録し、タスクから ADR/仕様へリンクして重複を避けます。

References ルール
- このルールの対象は、front-matter に `id:` を持つ markdown ドキュメントのみ。
- `README.md`、`README.ja.md`、`GUIDE.md`、`GUIDE.ja.md` は対象外。

## フロントマター (YAML) スキーマ

必須
- id (string): 一意なタスク ID (推奨: YYYYMMDDa-short-description)
- title (string): 短いタイトル
- status (enum): 下記「ステータス値」を参照
- updated (timestamp): UTC の ISO 8601 `YYYY-MM-DDTHH:MM:SSZ`
- language (enum): ja | en

任意
- owner (string): GitHub ハンドルまたは氏名
- plans (string[]): このタスクが参照する plan の doc-id のリスト (例: `2026aa-kompox-box-update`)
- supersedes (string|string[]): このタスクが置き換える旧タスク ID (複数可)
- supersededBy (string): このタスクを置き換える新タスク ID

相互参照ルール
- task 文書では、参照する plan の doc-id を `plans` に列挙する。
- 値は doc-id を使い、ファイル名拡張子は含めない。

ステータス値 (推奨)
- draft: 作成直後/編集中
- active: 作業中
- blocked: 依存待ち
- done: 完了 (履歴として保持)
- canceled: 取り止め (履歴として保持)
- superseded: 置換済み (履歴として保持)

## テンプレート (コピーして調整)

```markdown
---
id: YYYYMMDDa-short-description
title: 短いタイトル
status: draft
updated: 2026-02-14T00:00:00Z
language: ja
owner:
plans: []
supersedes: []
supersededBy:
---
# Task: <短いタイトル>

## 目的

- ...

## スコープ/非スコープ

- In: ...
- Out: ...

## 仕様サマリ

- ... (詳細は ADR/仕様へリンク)

## 計画 (チェックリスト)

- [ ] Step 1
- [ ] Step 2

## テスト

- ユニット: ...
- スモーク: ...

## 受け入れ条件

- ...

## メモ

- リスク: ...
- フォローアップ: ...

## 進捗

- YYYY-MM-DD: ...

## 参照

- ...
```

## インデックス化

- Makefile のターゲットで design/tasks/README.md を再生成できます:
- make gen-index

## 参照

- [GUIDE.md]

[GUIDE.md]: ./GUIDE.md
