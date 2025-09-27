# メンテナー向けタスクガイド（`_dev/tasks`）

このフォルダには、メンテナーが実装に向けて動くための短いタスク文書（実装計画、受け入れ条件、テストメモなど）を置きます。ユーザー向けドキュメントではありません。ディレクトリ名がアンダースコアで始まるため、Go パッケージからも自動的に除外されます。

## ここに置くもの

- 具体的な変更の実装計画（1〜2ページ）
- 単一の成果物に紐づく受け入れ条件とテスト計画
- 設計判断（ADR）や最終仕様へのリンク

## ここに置かないもの

- 正式な設計仕様（`design/` 配下）
- ユーザードキュメント（`docs/` 配下、MkDocs）
- 長期ロードマップ（`design/` などへ）

## ファイル命名

- 形式: `YYYY-MM-DD-topic.lang.md`
  - 例: `2025-09-27-disk-snapshot-unify.ja.md`, `2025-10-05-cli-refactor.en.md`
- topic は参照しやすくするため可能な限り短く。言語サフィックス（`.ja.md` or `.en.md`）を付ける。

## ワークフロー（軽量）

1) 下記テンプレートを使って新しいタスクファイルを作る
2) 作業中は適宜更新（status, updated, チェックリスト）
3) 完了したら `status: done` にしてファイルは残す（履歴のため）
   - 置き換えられた場合は `status: superseded` にして新タスクへのリンクを追加

ヒント: 判断は ADR に記録。タスクから ADR/仕様にリンクし、重複を避ける。

## フロントマター（YAML）スキーマ

必須
- `id` (string): 一意なタスク ID（推奨: `YYYY-MM-DD-topic`）
- `title` (string): 短いタイトル
- `status` (enum): 下記「ステータス値」を参照
- `updated` (date): ISO `YYYY-MM-DD`
- `language` (enum): `ja | en`

任意
- `owner` (string): GitHub ハンドルまたは氏名
- `supersedes` (string|string[]): このタスクが置き換える旧タスク ID（複数可）
- `supersededBy` (string): このタスクを置き換える新タスク ID

ステータス値
- `draft`: 作成直後／編集中
- `active`: 作業中
- `done`: 完了（履歴として保持）
- `canceled`: 取り止め（履歴として保持）

注記: 系譜（置換関係）は `supersedes`/`supersededBy` で表現するのを基本とし、状態の語彙は最小限を推奨。待ち状態が必要なら拡張として `blocked` や `on-hold` を追加検討。

例
```yaml
---
id: 2025-09-27-disk-snapshot-unify
title: Disk/Snapshot 機能統合（disk create -S）
status: draft
updated: 2025-09-27
language: ja
owner: your-handle
---
```

## 推奨構成（本文）

- 目的: なぜこのタスクが必要か、成功の状態
- スコープ / 非スコープ: テスト可能な最小範囲に絞る
- 仕様サマリ: 最小限の仕様変更（詳細は ADR/仕様へリンク）
- 計画（チェックリスト）: 小さなステップに分割
- 進捗: 短いメモ（日時 + 簡潔な結果）
- テスト: ユニット/統合/スモークと検証ポイント
- 受け入れ条件: 簡潔かつ検証可能
- メモ: リスク、フォローアップ、移行の注意

## テンプレート（コピーして調整）

```markdown
---
id: YYYY-MM-DD-topic
title: 短いタイトル
status: draft
updated: YYYY-MM-DD
language: ja
---
# Task: <短いタイトル>

## 目的

- ...

## スコープ / 非スコープ

- In: ...
- Out: ...

## 仕様サマリ

- ...（ADR/仕様へリンク）

## 計画（チェックリスト）

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

- YYYY-MM-DD: 1〜3 を PR #<num> で実装 などの短い記述

## 参考

- design/adr/K4x-ADR-00X.md
- design/v1/<Spec>.md
```

## インデックス化

- Makefile のターゲットで `README.md`（および `README.ja.md`）を再生成できます:
- `make gen-index`