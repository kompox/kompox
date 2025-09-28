---
id: 2025-09-28-disk-snapshot-cli-flags
title: Disk/Snapshot CLI フラグ統一（-N/-S）
status: active
updated: 2025-09-28
language: ja
supersedes: []
---
# Task: Disk/Snapshot CLI フラグ統一（-N/-S）

## 目的

- disk/snapshot 系コマンドの UX を統一し、学習コストを下げる。
- create に `-N/--name` を導入して任意名での作成を可能にする。
- `-S/--source` を「create 専用の作成元指定」に明確化し、意味の衝突を解消する。

## スコープ / 非スコープ

- In:
  - `kompoxops` の disk/snapshot サブコマンドの CLI フラグ設計・ヘルプ整備
  - `-N/--name` の導入（create/assign/delete）と `-S/--source` の create 限定
  - `--disk-name` / `--snap-name` のロングエイリアスを公式化
  - `design/v1/Kompox-CLI.ja.md` の更新
- Out:
  - Provider Driver の解釈ロジック変更（Source の解釈は Driver 側の責務）
  - 他サブコマンド（app/box/cluster/secret 等）のフラグ整理
  - 旧フラグの互換サポート（今回「後方互換なし」）

## 仕様サマリ（最終形）

共通: `-A|--app-name`（アプリ名）、`-V|--vol-name`（ボリューム名）

- disk list:   `kompoxops disk list   -A <app> -V <vol>`
- disk create: `kompoxops disk create -A <app> -V <vol> [-N <name>] [-S <source>]`
  - `-S` 省略時は「空ディスク（プロバイダ既定）」を作成
- disk assign: `kompoxops disk assign -A <app> -V <vol> -N <name>`
- disk delete: `kompoxops disk delete -A <app> -V <vol> -N <name>`

- snapshot list:   `kompoxops snapshot list   -A <app> -V <vol>`
- snapshot create: `kompoxops snapshot create -A <app> -V <vol> [-N <name>] [-S <source>]`
  - `-S` 省略時は「現在 Assigned なディスク」を作成元に使用
- snapshot delete: `kompoxops snapshot delete -A <app> -V <vol> -N <name>`

命名フラグ（対象指定）
- disk コマンド: `-N | --name | --disk-name` で操作対象の Disk を指定
- snapshot コマンド: `-N | --name | --snap-name` で操作対象の Snapshot を指定

`-S/--source`（create 専用）
- Opaque（不透明）: CLI/UseCase は解釈・検証・正規化を行わず、そのまま Driver に渡す。
- 形式: `-S [<type>:]<name>`（共通 type: `disk` | `snapshot`）
  - 値で type を省略した場合: `disk create` → `snapshot:<name>`／`snapshot create` → `disk:<name>`
  - `-S` 自体を省略した場合: `disk create` → 空ディスク作成／`snapshot create` → Assigned なディスクを解決して `disk:<name>` を使用

`-N/--name`（create 時の任意指定）
- 省略時は自動命名（例: ULID 等）。重複時はエラー。

## 設計原則（再掲）

- Source は CLI/UseCase でパース・検証・正規化しない。
- 解釈ルールや受理フォーマットは Provider Driver の契約に委譲する。
- ただし上記の共通語彙（`disk:`/`snapshot:`）は全 Driver で予約する。

## 破壊的変更 / 非互換

- 旧来の個別フラグ（例: `-D/--disk-name` の短縮 `-D`、`--snapshot-name` の短縮 `-S` など）を廃止。
- `snapshot delete` において `-S` が「対象名」を意味する挙動は撤廃（`-S` は create 専用の Source）。

## 計画（チェックリスト）

- [ ] CLI 実装を更新
  - [ ] `cmd/kompoxops/cmd_disk.go`: `-N|--name|--disk-name` を追加・統一、`disk create` に `-S|--source` を維持（省略時 empty）。
  - [ ] `cmd/kompoxops/cmd_snapshot.go`: `-N|--name|--snap-name` を追加・統一、`snapshot create` に `-S|--source`（省略時 Assigned disk）。
  - [ ] ヘルプ文言（Short/Long/Example）を更新。
- [ ] ドキュメント更新
  - [ ] `design/v1/Kompox-CLI.ja.md` を本仕様に合わせて改訂。
  - [ ] 例示コマンド（README 含む）を `-N/-S` 方針へ統一。
- [ ] テスト整備
  - [ ] CLI: フラグ受理と UseCase への値伝播（-N/-S）をユニットで検証。
  - [ ] UseCase: `snapshot create` で `-S` 省略時に「Assigned なディスク」を自動選択できること（存在しない場合は明示エラー）。
  - [ ] スモーク: `disk create` 省略（空ディスク）、`disk create -S snapshot:...`、`snapshot create` 省略/明示の基本系が通ること。

## テスト

- ユニット
  - `cmd_disk`: `--name`, `--disk-name` の同義受理、`--source` の透過（文字列一致）を確認。
  - `cmd_snapshot`: `--name`, `--snap-name` の同義受理、`--source` の透過と省略時の既定設定（UseCase 側）を確認。
- スモーク
  - `kompoxops disk create -A app -V vol` が成功し、Assigned=false の新規ディスクが 1 件返る。
  - `kompoxops disk create -A app -V vol -S snapshot:<snap>` が成功する（Driver 解釈）。
  - `kompoxops snapshot create -A app -V vol` が成功し、作成元が現在の Assigned ディスクとなる。

## 受け入れ条件

- 新しいヘルプ/Usage が `-N/--name` と `-S/--source` の意味を正しく反映している。
- 上記 7 コマンドの挙動が仕様どおりである（-S の既定含む）。
- `source` は CLI/UseCase で解釈されず、そのまま Driver に渡る。
- 旧フラグ（短縮 `-D` 等）は受理されない。

## メモ

- `snapshot create` の `-S` 省略既定は UseCase 層で「現在 Assigned のディスク名を `disk:<name>` として Source に設定」する想定。
- `disk create` の `-S` 省略既定は Empty（Source 空文字を許容、Driver が新規作成経路に分岐）を想定。
  - 一方で `-S` を指定しつつ値で type を省略した場合は、`disk create`→`snapshot:<name>`、`snapshot create`→`disk:<name>` と解釈する（共通語彙の既定）。
- Provider の制約（ゾーン/リージョン、RBAC）はドライバ側の責務。

## 進捗

- 2025-09-28: タスク作成（本ファイル）。前提: 2025-09-27 の Source パススルー実装が完了済み。

## 参考

- `_dev/tasks/2025-09-27-disk-snapshot-unify.ja.md`
- `design/adr/K4x-ADR-003.md`
- `design/v1/Kompox-CLI.ja.md`
