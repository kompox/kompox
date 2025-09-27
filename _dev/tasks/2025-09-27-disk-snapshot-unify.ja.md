---
id: 2025-09-27-disk-snapshot-unify
title: Disk/Snapshot 機能統合（disk create -S）
status: active
owner: yaegashi
updated: 2025-09-27
language: ja
references:
  - design/adr/K4x-ADR-002.md
  - design/v1/Kompox-CLI.ja.md
---
# Task: Disk/Snapshot 機能統合（`disk create -S`）

## 目的

- `snapshot restore` コマンドを廃止し、スナップショットからの復元を `disk create` に統合する。
- Kompox 管理外のクラウドリソース（例: Azure の Snapshot/Disk Resource ID）からのディスク作成もサポートし、インポート経路を確立する。

## 仕様（サマリ）

- CLI: `kompoxops disk create -V <vol> [-S <source>] [--zone <zone>] [--options <json>]`
  - `-S/--source` の自動判別:
    - 未指定/空: 空ディスク
    - 先頭が `/subscriptions/` の場合: Provider ネイティブ Resource ID
    - それ以外: Kompox 管理スナップショット名として解決
- UseCase/Model:
  - `DiskCreateInput.Source string` を追加し、`VolumeDiskCreateOption` に伝播
  - `model.WithVolumeDiskCreateSource(src string)` を追加
- Provider Driver:
  - `VolumeDiskCreate` 内で `Source` を評価し、`CreateOption=Copy + SourceResourceID`（または Empty）で作成
- 移行:
  - まず `disk create -S` を実装・確認し、その後 `snapshot restore` 関連を削除

## 実装方針（段階）

1) モデル拡張
- `model.VolumeDiskCreateParams` に `Source string` を追加
- `model.WithVolumeDiskCreateSource(src string)` を追加

2) UseCase 拡張
- `usecase/volume/disk_create.go` の入力に `Source` を追加し、オプションへ反映

3) CLI 拡張
- `cmd/kompoxops/cmd_disk.go` に `-S, --source` を追加

4) Provider Driver (AKS) 拡張
- `VolumeDiskCreate` で `Source` を判定し、Empty/Copy を切替
- Kompox 管理スナップショット名はドライバ内で Resource ID に解決

5) ドキュメント
- `design/v1/Kompox-CLI.ja.md` に `-S/--source` を追記

6) テスト
- UseCase: `Source` がオプションに伝播するユニットテスト
- ドライバ（可能ならモック）: Empty/ResourceID/内部スナップショットの3分岐
- CLI: `-S` の引数受け渡しスモーク

7) 削除フェーズ
- `usecase/volume/snapshot_restore.go` を削除
- `adapters/drivers/provider/registry.go` の `VolumeSnapshotRestore` を削除
- CLI の snapshot restore コマンドを削除
- 関連ドキュメントから snapshot restore 記述を削除

## 受け入れ条件

- `kompoxops disk create -V default` が空ディスクを作成し成功する
- `kompoxops disk create -V default -S <KompoxSnapshotName>` がスナップショットから作成できる
- `kompoxops disk create -V default -S /subscriptions/...` が外部 Resource ID から作成できる
- 作成されたディスクのタグ/Zone/Options が期待通りに反映され、`Assigned=false` で返る
- 削除フェーズ後、コードベースに SnapshotRestore のシンボルが存在しない

## 注意点

- プロバイダ制約（例: Azure ではリージョン一致、必要な RBAC 権限）
- Source の曖昧さ対策として、将来 `--source-kind` の導入余地
