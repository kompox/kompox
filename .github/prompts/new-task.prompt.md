---
description: Create a new task file
mode: agent
tools: ['runCommands', 'edit', 'search', 'todos', 'changes', 'fetch']
---
## Your Task

- Create a new task file in [_dev/tasks].
  - Standard path: `YYYY-MM-DD-short-description.ja.md`
  - Follow user's instructions if provided.
- Honor the general instructions: [md] and [ja].
- Follow the guidelines: [GUIDE.ja.md].
- Refer the existing task files as examples for structure and style.
- Use the same set of front-matter fields as in the provided example.
- Maintain a markdown link list in the end of the file.
  - Use the label reference style like `[K4x-ADR-014]` in the content.
  - Do not mention meta instruction files like [GUIDE.ja.md].
- Run make gen-index after creating the file to update the task index.

## Task File Example

Task file path: `_dev/tasks/2025-10-24-volume-types.ja.md`

```markdown
---
id: 2025-10-24-volume-types
title: Volume Type 実装
status: active
updated: 2025-10-24
language: ja
owner:
---
# Task: Volume Type 実装

## 目的

- [K4x-ADR-014] ("Introduce Volume Types") を実装する。
- ...

## スコープ / 非スコープ

- In:
  - ...
- Out:
  - ...

## 仕様サマリ

## 計画 (チェックリスト)

## 受け入れ条件

## メモ

## 進捗

- 2025-10-24: タスク作成

## 参考

- [K4x-ADR-014]

[K4x-ADR-014]: ../../design/adr/K4x-ADR-014.md
```

[_dev/tasks]: ../../_dev/tasks
[GUIDE.ja.md]: ../../_dev/tasks/GUIDE.ja.md
[md]: ../instructions/md.instructions.md
[ja]: ../instructions/ja.instructions.md
