---
id: 2025-09-28-disk-snapshot-cli-flags
title: Disk/Snapshot CLI フラグ統一(-N/-S)
status: active
updated: 2025-09-28
language: ja
supersedes: []
---
# Task: Disk/Snapshot CLI フラグ統一(-N/-S)

## 目的

- disk/snapshot 系コマンドの UX を統一し、学習コストを下げる。
- create に `-N/--name` を導入して任意名での作成を可能にする。
- `-S/--source` を「create 専用の作成元指定」に明確化し、意味の衝突を解消する。

## スコープ / 非スコープ

- In:
  - `kompoxops` の disk/snapshot サブコマンドの CLI フラグ設計・ヘルプ整備
  - `-N/--name` の導入(create/assign/delete)と `-S/--source` の create 限定
  - `--disk-name` / `--snap-name` のロングエイリアスを公式化
  - `design/v1/Kompox-CLI.ja.md` の更新
- Out:
  - Provider Driver の解釈ロジック変更(Source の解釈は Driver 側の責務)
  - 他サブコマンド(app/box/cluster/secret 等)のフラグ整理
  - 旧フラグの互換サポート(今回「後方互換なし」)

## 仕様サマリ(最終形)

共通: `-A|--app-name`(アプリ名)、`-V|--vol-name`(ボリューム名)

- disk list:   `kompoxops disk list   -A <app> -V <vol>`
- disk create: `kompoxops disk create -A <app> -V <vol> [-N <name>] [-S <source>]`
  - `-S` 省略時は空ディスクを作成
- disk assign: `kompoxops disk assign -A <app> -V <vol> -N <name>`
- disk delete: `kompoxops disk delete -A <app> -V <vol> -N <name>`
- snapshot list:   `kompoxops snapshot list   -A <app> -V <vol>`
- snapshot create: `kompoxops snapshot create -A <app> -V <vol> [-N <name>] [-S <source>]`
  - `-S` 省略時は Assigned なディスクをソースに使用、単一の Assigned なディスクが無い場合はエラー
- snapshot delete: `kompoxops snapshot delete -A <app> -V <vol> -N <name>`

命名フラグ(対象指定)
- disk コマンド: `-N | --name | --disk-name` で操作対象の Disk を指定
- snapshot コマンド: `-N | --name | --snap-name` で操作対象の Snapshot を指定

`-S/--source`(create 専用)
- 形式: `-S [<type>:]<name>`
- Opaque(不透明): CLI/UseCase は解釈・検証・正規化を行わず、そのまま Driver に渡す。
- 共通の type の語彙として `disk:` と `snapshot:` を予約する。

`-N/--name`(create 時の任意指定)
- 省略時は自動命名(例: ULID 等)。重複時はエラー。

## 設計原則(再掲)

- Source は CLI/UseCase でパース・検証・正規化しない。
- 解釈ルールや受理フォーマットは Provider Driver の契約に委譲する。
- ただし上記の共通語彙(`disk:`/`snapshot:`)は全 Driver で予約する。

## 破壊的変更 / 非互換

- 旧来の個別フラグ(例: `-D/--disk-name` の短縮 `-D`、`--snapshot-name` の短縮 `-S` など)を廃止。
- `snapshot delete` において `-S` が「対象名」を意味する挙動は撤廃(`-S` は create 専用の Source)。

## 計画(チェックリスト)

- [ ] Domain を調整(Driver 主導のシグネチャへ移行)
  - [ ] `domain/model/volume_port.go`
    - [ ] `VolumeDiskCreateOptions` の `Source` メンバを削除(Source は Driver メソッドの引数に移譲)。
    - [ ] `VolumeDiskCreateOptions` に `Name` は追加しない(作成名は Driver の引数で受ける)。
    - [ ] `VolumeSnapshotCreateOptions` は更新不要(現状のまま)。
    - [ ] Option ヘルパーを整理:
      - [ ] (削除)`WithVolumeDiskCreateSource` は提供しない。
      - [ ] Name 系ヘルパーは提供しない(Name は Driver 引数で受ける)。
    - [ ] コメントは ADR `design/adr/K4x-ADR-003.md` に整合(Source はオペーク、Driver 引数で受ける)。

- [ ] UseCase を更新(Driver シグネチャへ引数を直渡し)
  - [ ] `usecase/volume/disk_create.go`
  - [ ] CLI の `-N` は Driver の `diskName` 引数へ直接渡す(Domain Options へは反映しない)。
  - [ ] `-S` の値はそのまま Driver の `VolumeDiskCreate(ctx, ..., diskName, source, opts...)` へ渡す(完全オペーク)。
  - [ ] `-S` 省略時は `source=""` を渡し、既定は Driver に委任。
  - [ ] `usecase/volume/snapshot_create.go`
  - [ ] CLI の `-N` は Driver の `snapName` 引数へ直接渡す(Domain Options へは反映しない)。
  - [ ] `-S` の値はそのまま Driver の `VolumeSnapshotCreate(ctx, ..., snapName, source, opts...)` へ渡す(完全オペーク)。
  - [ ] `-S` 省略時は `source=""` を渡し、既定は Driver に委任。

- [ ] Provider Driver を対応(シグネチャ変更／後方互換なし)
  - [ ] `platformdev.Driver` のメソッドを変更:
    - [ ] `VolumeDiskCreate(ctx, cluster, app, volName, diskName string, source string, opts ...model.VolumeDiskCreateOption) (*model.VolumeDisk, error)`
    - [ ] `VolumeSnapshotCreate(ctx, cluster, app, volName, snapName string, source string, opts ...model.VolumeSnapshotCreateOption) (*model.VolumeSnapshot, error)`
  - [ ] `diskName`/`snapName`/`source` は省略時にゼロ値(空文字)を許容し、既定動作に分岐。
  - [ ] Source はオペークで受け取り、少なくとも予約 `disk:` / `snapshot:` 接頭辞を理解する。
  - [ ] 重複名・無効 Source は明示エラー。
  - [ ] AKS Driver 実装 (`adapters/drivers/provider/aks`)
    - [ ] `VolumeDiskCreate` で `source==""` の場合には新規ディスクを作成する。
    - [ ] `VolumeSnapshotCreate` で `source==""` の場合には Assigned ディスクを取得し `disk:<name>` として処理する。単一の Assigned ディスクが見つからない場合はエラー。

- [ ] CLI 実装を更新
  - [ ] `cmd/kompoxops/cmd_disk.go`: `-N|--name|--disk-name` を追加・統一、`disk create` に `-S|--source` を維持(省略時 empty)。
  - [ ] `cmd/kompoxops/cmd_snapshot.go`: `-N|--name|--snap-name` を追加・統一、`snapshot create` に `-S|--source`(省略時は空文字を渡し Driver に委任)。
  - [ ] ヘルプ文言(Short/Long/Example)を更新。

- [ ] ドキュメント更新
  - [ ] `design/v1/Kompox-CLI.ja.md` を本仕様に合わせて改訂。
  - [ ] 例示コマンド(README 含む)を `-N/-S` 方針へ統一。
  - [ ] ADR `design/adr/K4x-ADR-003.md` への参照と整合コメント(Domain Option の Name/Source 追記)を反映。

- [ ] テスト整備
  - [ ] CLI: フラグ受理と UseCase への値伝播(-N/-S)をユニットで検証。
  - [ ] UseCase: `-S` 値は文字列を一切加工せずに Driver へ透過し、`-S` 省略時は空文字を渡すことを確認。
  - [ ] Driver: `diskName`/`snapName`/`source`(省略時は空文字)引数が期待どおりに受け取れること(パススルー)を確認。
  - [ ] Driver: `snapshot create` にて `source==""` の場合に Assigned ディスクを解決して使用し、Assigned が無ければエラーとなることをユニット/インテグレーションで確認(AKS Driver での実装含む)。
  - [ ] スモーク: `disk create` 省略(空ディスク)、`disk create -S snapshot:...`、`snapshot create` 省略/明示の基本系が通ること。

## テスト

- ユニット
  - `cmd_disk`: `--name`, `--disk-name` の同義受理、`--source` の透過(文字列一致)を確認。
  - `cmd_snapshot`: `--name`, `--snap-name` の同義受理、`--source` の透過と省略時の既定設定(UseCase 側)を確認。
- スモーク
  - `kompoxops disk create -A app -V vol` が成功し、Assigned=false の新規ディスクが 1 件返る。
  - `kompoxops disk create -A app -V vol -S snapshot:<snap>` が成功する(Driver 解釈)。
  - `kompoxops snapshot create -A app -V vol` が成功し、作成元が現在の Assigned ディスクとなる。

## 受け入れ条件

- 新しいヘルプ/Usage が `-N/--name` と `-S/--source` の意味を正しく反映している。
- 上記 7 コマンドの挙動が仕様どおりである(-S の既定含む)。
- `source` は CLI/UseCase で解釈・補完されず、そのまま Driver に渡る(完全オペーク)。
- 旧フラグ(短縮 `-D` 等)は受理されない。
- Domain のオプション型が調整されている:
  - `VolumeDiskCreateOptions` から `Source` が削除されている(`Name` は追加しない)。
  - `VolumeSnapshotCreateOptions` は変更なし。
 UseCase は `-S` の加工や解決を行わない:
  - `disk create` は `-S` 省略時に `source=""` を Driver へ渡す。
  - `snapshot create` も `-S` 省略時に `source=""` を Driver へ渡す。
  - `-S` 値は与えられた文字列を一切加工しない(type 有無も判別しない)。
- Driver は新シグネチャを実装し、`diskName`/`snapName`/`source` のゼロ値を正しく既定動作にマップし、無効 Source や重複名で適切にエラーを返す。

## 進捗

- 2025-09-28: タスク作成(本ファイル)。前提: 2025-09-27 の Source パススルー実装が完了済み。

## 参考

- `_dev/tasks/2025-09-27-disk-snapshot-unify.ja.md`
- `design/adr/K4x-ADR-003.md`
- `design/v1/Kompox-CLI.ja.md`
