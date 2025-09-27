---
id: 2025-09-27-disk-snapshot-unify
title: Disk/Snapshot 機能統合(disk create -S)
status: active
owner: yaegashi
updated: 2025-09-27
language: ja
references:
  - design/adr/K4x-ADR-002.md
  - design/v1/Kompox-CLI.ja.md
---
# Task: Disk/Snapshot 機能統合(`disk create -S`)

## 目的

- `snapshot restore` コマンドを廃止し、スナップショットからの復元を `disk create` に統合する。
- Kompox 管理外のクラウドリソース(例: Azure の Snapshot/Disk Resource ID)からのディスク作成もサポートし、インポート経路を確立する。

## 設計原則(解釈の委譲)

- Source 文字列について CLI や UseCase でなにか解釈することはしない。
  - Source 文字列は不透明(opaque)な値として扱い、パース・バリデーション・正規化を行わない。
  - 受理フォーマットや曖昧性解消のルールは Provider Driver の契約(ドライバ間で異なることを許容)。
  - ただし、最低限の共通語彙として `disk:` と `snapshot:` は全ドライバで予約される。
    - `disk:<name>` は Kompox 管理ディスク名の参照を意味し、各ドライバはこれを解決できること。
    - `snapshot:<name>` は Kompox 管理スナップショット名の参照を意味し、各ドライバはこれを解決できること。
    - 省略時は `snapshot:` と解釈すること。

## 仕様（サマリ）

- CLI: `kompoxops disk create -V <vol> [-S <source>] [--zone <zone>] [--options <json>]`
  - `-S/--source`: 任意の文字列を受け取り、そのまま UseCase→Provider Driver に透過的に渡す。
  - CLI では `source` の解釈・検証・正規化を行わない(ヘルプには「受理形式は Provider Driver の仕様に従う」と明記)。
- UseCase/Model:
  - `DiskCreateInput.Source string` を追加し、`VolumeDiskCreateOption` にそのまま伝播する(opaque contract)。
  - `model.WithVolumeDiskCreateSource(src string)` を追加(値を格納するだけのセッタ)。
- Provider Driver:
  - `VolumeDiskCreate` 内で `Source` を評価し、受理フォーマットに従って Copy/Import/Empty を判定。
  - 例: AKS ドライバであれば、Kompox 管理名や Azure ARM Resource ID などの形式を受理し、必要な解決・検証・正規化・エラー化を行う。
- 移行:
  - まず `disk create -S` のパススルー経路(CLI→UseCase→Driver)を実装・確認し、その後 `snapshot restore` 関連を削除

## 実装方針（段階）

1) モデル拡張
- `model.VolumeDiskCreateParams` に `Source string` を追加
- `model.WithVolumeDiskCreateSource(src string)` を追加

2) UseCase 拡張
- `usecase/volume/disk_create.go` の入力に `Source` を追加し、オプションへ反映

3) CLI 拡張（パススルー実装）
- `cmd/kompoxops/cmd_disk.go` に `-S, --source` を追加
  - 入力値はパースせず、そのまま UseCase に渡す。
  - フォーマットの列挙や自動補完は行わない。
  - ヘルプ文言のみ「入力形式は Provider Driver の仕様を参照」と案内。

4) Provider Driver (AKS) 拡張
- `VolumeDiskCreate` で `Source` を判定し、Empty/Copy/Import を切替
- Kompox 管理ディスク/スナップショット名や外部 ID(例: Azure ARM Resource ID)はドライバ内で解決
- 受理する入力例や解釈ルールは AKS ドライバのドキュメント/テストとして明示
- 以下は AKS ドライバが受理する想定の一例。CLI はこれらの文字列を解釈せず、そのまま Driver に渡す。
  - `kompoxops disk create -V default` → 空ディスク(Source 未指定)
  - `kompoxops disk create -V default -S snapshot:backup-20250927` → 管理スナップショット名をドライバが解釈
  - `kompoxops disk create -V default -S disk:gold-master` → 管理ディスク名をドライバが解釈
  - `kompoxops disk create -V default -S arm:/subscriptions/.../providers/Microsoft.Compute/disks/d1` → ARM ディスク ID をドライバが解釈
  - `kompoxops disk create -V default -S resourceId:/subscriptions/.../providers/Microsoft.Compute/snapshots/s1` → ARM スナップショット ID をドライバが解釈
  - `kompoxops disk create -V default -S /subscriptions/.../providers/Microsoft.Compute/disks/d2` → 先頭 `/subscriptions/` を持つ値をドライバが ARM と認識する実装例
  - `kompoxops disk create -V default -S daily-20250927` → `snapshot:` 省略をドライバが許容する実装例

5) ドキュメント
- `design/v1/Kompox-CLI.ja.md` に `-S/--source` を追記

6) テスト
- CLI: `-S` で受け取った値が変換されずに UseCase→Driver へ到達することのスモーク/ユニット
- UseCase: `Source` がオプションにそのまま伝播するユニットテスト
- ドライバ(可能ならモック): Empty/ResourceID/内部スナップショット等の分岐、バリデーション、エラーの単体テスト

7) 削除フェーズ
- `usecase/volume/snapshot_restore.go` を削除
- `adapters/drivers/provider/registry.go` の `VolumeSnapshotRestore` を削除
- CLI の snapshot restore コマンドを削除
- 関連ドキュメントから snapshot restore 記述を削除

## 受け入れ条件

- CLI/UseCase に `Source` の解釈(パース・バリデーション・正規化)が存在しない(コードリーディング/テストで確認)
- `kompoxops disk create -V default` が空ディスクを作成し成功する
- `kompoxops disk create -V default -S snapshot:<KompoxSnapshotName>` でスナップショットから作成できる(Driver 解釈)
- `kompoxops disk create -V default -S disk:<KompoxDiskName>` で管理ディスクから作成できる(Driver 解釈)
- `kompoxops disk create -V default -S /subscriptions/...` や `arm:/subscriptions/...`、`resourceId:/subscriptions/...` で外部 Resource ID から作成できる(Driver 解釈)
- 作成されたディスクのタグ/Zone/Options が期待通りに反映され、`Assigned=false` で返る
- 削除フェーズ後、コードベースに SnapshotRestore のシンボルが存在しない

## 注意点

- プロバイダ制約(例: Azure ではリージョン一致、必要な RBAC 権限)
- 受理フォーマットは Driver の契約であり、ドライバ間で差異があり得る。各 Driver ドキュメント/テストで明示する。
- 将来の拡張として、曖昧性低減のため `--source-kind` を導入可能だが、解釈は依然として Driver 側の責務。
