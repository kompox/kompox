---
id: 2025-10-27-volume-types
title: Volume Types 実装
status: active
updated: 2025-10-28
language: ja
owner: yaegashi
---
# Task: Volume Types 実装

## 目的

- [K4x-ADR-014] ("Introduce Volume Types") を実装する。
- 既存の Volume/Disk/Snapshot 契約を維持しつつ、RWX 向けの provider 管理ファイル共有 (Azure Files, EFS, Filestore など) を `Type = "files"` として扱えるようにする。
- ドメインは provider/Kubernetes 詳細から独立させ、マッピングはドライバー/アダプター層で行う。

## スコープ / 非スコープ

- In:
  - ドメインモデル拡張: `AppVolume.Type` 追加 (空は `"disk"` 扱い)
  - 定数導入: `VolumeTypeDisk = "disk"`, `VolumeTypeFiles = "files"`
  - `ErrNotSupported` エラー追加 (スナップショット非対応の provider/type で使用)
  - `VolumeDisk` の意味拡張 (`Type = "files"` 時):
    - `Name`: 共有/エクスポート名
    - `Handle`: provider ネイティブ URI (例: `smbs://{account}.file.core.windows.net/{share}`, `nfs://{host}:/{export}`)
    - `Size`: 共有クォータ (バイト; 未設定は 0)
    - `Zone`: リージョナルサービスでは空 (可用性は `Options` で表現)
    - `Options`: provider 固有属性 (`protocol`, `skuName`, `availability`, `quotaGiB` など)
  - Kube 変換: `Type = "files"` のとき RWX PVC と provider CSI パラメータ出力
  - AKS ドライバー: `files` を Azure Files で実装
    - App 単位で 1 Storage Account (Disk 初回作成時に自動作成)
    - 共有名: `{vol.name}-{disk.name}`
    - 共有を Disk として表現、URI Handle
    - SMB プロトコルのみサポート (NFS は将来拡張)
    - RWX PVC 生成
  - 既存 `VolumePort` シグネチャ維持 ("Disk" は「プロビジョン済みアーティファクト」の総称)
- Out:
  - 公開 API の破壊的変更 (CRD 以外)
  - 全 provider の即時実装 (AKS 優先; 他は別タスク)
  - Azure NetApp Files/Managed Lustre/Blob FUSE 等の最適化 (将来拡張)
  - accessModes/StorageClass のみで RWX を駆動する設計 (ADR で却下)

## 仕様サマリ

- Canonical Types とバリデーション
  - 許可値: `"disk"`, `"files"` (空は `"disk"`)
  - 不明値は早期バリデーションで拒否 (CRD 変換/ドメイン検証)
- スナップショット
  - `Type = "files"` では多くの provider で非対応
  - `Snapshot*` 操作は `ErrNotSupported` を返す
- AKS ドライバー実装
  - `Type = "disk"`: Azure Managed Disk (既存動作)
  - `Type = "files"`: Azure Files (新規)
    - ストレージアカウント名: `k4x{prv_hash}{app_hash}` (15文字、小文字英数字のみ)
    - 共有名: `{vol.name}-{disk.name}` (最大41文字)
    - 共有メタデータで管理: `kompox-volume`, `kompox-disk-assigned`
    - Handle: `smbs://{account}.file.core.windows.net/{share}`
    - 既定 SKU: `Standard_LRS`、プロトコル: `smb` (固定)
- VolumeClass 返却値 (`Type = "files"`)
  - `StorageClassName`: `"azurefile-csi"`
  - `CSIDriver`: `"file.csi.azure.com"`
  - `AccessModes`: `["ReadWriteMany"]`
  - `Attributes`: `protocol` (固定 `"smb"`), `skuName`, `availability` など
- 認証メモ
  - Azure Files: CSI ドライバーが WI/MI 経由で鍵/SAS 取得
  - データプレーン認証は OIDC 非対応 (ドメインモデルには非公開)

## 計画 (チェックリスト)

- [x] ドメイン
  - [x] `domain/model`: `AppVolume` に `Type` フィールド追加
  - [x] `domain`: `VolumeTypeDisk`, `VolumeTypeFiles`, `ErrNotSupported` 定数追加
  - [x] バリデーション: 不明 `Type` を拒否、空は `disk` 解釈
  - [x] `VolumeDisk` コメント更新 (`files` の意味明記)
- [x] UseCase/ポート
  - [x] `VolumePort` 契約不変を確認
  - [x] `Snapshot*` は `Type` に関わらずドライバーに委譲 (UseCase では判断しない)
- [x] Kube 変換
  - [x] `Type = "files"` 時に RWX PVC 生成
  - [x] `Options` から CSI パラメータ反映 (`skuName` など、`protocol` は固定 `"smb"`)
  - [x] [Kompox-KubeConverter.ja.md] 仕様追記
- [x] AKS ドライバー - リファクタリング (`adapters/drivers/provider/aks`)
  - [x] ファイル構成整理 (`volume.go`, `volume_driver.go`, `volume_driver_disk.go`, `volume_driver_files.go`)
  - [x] 型名統一: `driverVolume` (interface), `driverVolumeDisk`, `driverVolumeFiles`
  - [x] インターフェース定義: `DiskList`, `DiskCreate`, `DiskDelete`, `DiskAssign`, `SnapshotList`, `SnapshotCreate`, `SnapshotDelete`, `Class`
  - [x] ヘルパー関数のメソッド化:
    - [x] `newVolumeDisk` → `(vd *driverVolumeDisk) newDisk`
    - [x] `azureZones` → `(vd *driverVolumeDisk) zones`
    - [x] `setAzureDiskOptions` → `(vd *driverVolumeDisk) setDiskOptions`
    - [x] `azureDiskOptions` → `(vd *driverVolumeDisk) diskOptions`
    - [x] `newVolumeSnapshot` → `(vd *driverVolumeDisk) newSnapshot` (移動元: `volume_snapshot.go` 削除)
  - [x] ディスパッチ処理共通化: `(d *driver) resolveVolumeDriver(vol *model.AppVolume)` メソッド追加
  - [x] 全7つのディスパッチメソッド簡潔化 (重複ロジック削除、合計76行削減)
  - [x] ビルド・テスト成功確認
- [x] AKS ドライバー - Azure Files 実装 (`adapters/drivers/provider/aks`)
  - [x] `driverVolumeFiles` 構造体と `newDriverVolumeFiles()` 実装
  - [x] `appStorageAccountName()`: ストレージアカウント名生成 (`k4x{prv_hash}{app_hash}` 形式、15文字)
  - [x] `ensureStorageAccountExists()`: Storage Account 自動作成ロジック (App 単位、初回 Disk 作成時)
  - [x] `DiskList()`: 共有一覧取得、メタデータフィルタリング
  - [x] `DiskCreate()`: 共有作成、メタデータ設定、`source` 指定時エラー
  - [x] `DiskAssign()`: 共有メタデータ更新 (`kompox-files-share-assigned`)
  - [x] `DiskDelete()`: 共有削除 (冪等)
  - [x] `Class()`: `Type = "files"` 時の StorageClass/CSI 情報返却 (`protocol="smb"` 固定)
  - [x] `SnapshotList/Create/Delete()`: `ErrNotSupported` 返却
  - [x] `newVolumeFilesDisk()`: 共有 Handle URI 生成 (`smb://...`)
  - [x] SKU 選択 (`Options` から取得、既定値設定)、プロトコル固定 (SMB)
  - [x] `Options.protocol` が `"smb"` 以外の場合エラー処理
- [ ] E2E テスト
  - [ ] `tests/aks-e2e-volume/` または新規テストケース作成
  - [ ] `Type = "files"` での共有作成/割当/削除
  - [ ] RWX PVC 生成確認
  - [ ] スナップショット非対応確認 (`ErrNotSupported`)
- [ ] ドキュメント
  - [ ] [Kompox-ProviderDriver-AKS.ja.md] 更新 (完了済み)
  - [ ] [Kompox-KubeConverter.ja.md] 更新
  - [ ] `kompoxops.yml` サンプル追加 (`Type: files`, `Options` 例)
  - [ ] リリースノート項目

## 受け入れ条件

- ドメイン
  - `AppVolume.Type` が空でも `disk` として動作
  - 不明な `Type` 値は明確なエラー
  - `VolumeTypeDisk`, `VolumeTypeFiles`, `ErrNotSupported` が公開
- AKS ドライバー (`Type = "files"`)
  - Azure Files Storage Account が App 単位で作成される
  - 共有名が `{vol.name}-{disk.name}` 形式
  - 共有メタデータで `kompox-volume`, `kompox-disk-assigned` 管理
  - `VolumeDisk.Handle` が `smbs://...` 形式
  - `Snapshot*` が `ErrNotSupported` 返却
  - プロトコルは SMB 固定 (`Options.protocol` が `"smb"` 以外はエラー)
  - `Options.skuName` で SKU 指定可能 (既定: `Standard_LRS`)
- Kube 変換
  - `Type = "files"` 時に `accessModes: [ReadWriteMany]` 出力
  - `Options` から CSI パラメータ設定
- 回帰
  - `Type` 未指定 (既存) で従来通り動作 (RWO, Managed Disk)
- テスト
  - 単体: バリデーション、変換、AKS ドライバー CRUD、スナップショット非対応
  - E2E: `files` での作成/割当/削除、PVC 生成、マウント動作

## メモ

- ストレージアカウントは最後の共有削除後も残る (手動削除が必要)
- 当面は SMB プロトコルのみサポート、NFS は将来拡張として検討
- Azure Files のネイティブスナップショット機能は将来拡張で検討
- `backend` オプションで将来的に ANF/Managed Lustre/Blob FUSE 対応を想定
- K3s は `Type = "disk"` で `backend=localpath` (RWO のみ)

## 進捗

- 2025-10-27: タスク作成
- 2025-10-28: ADR と AKS 仕様更新に基づきタスク書き直し
- 2025-10-28: AKS ドライバー リファクタリング完了
  - ファイル構成整理 (`volume*.go` 分割)
  - 型名・メソッド名統一 (`driverVolume*`, メソッドから `Volume` プレフィックス削除)
  - ヘルパー関数のメソッド化 (5関数 → メソッド化、カプセル化改善)
  - ディスパッチ処理共通化 (`resolveVolumeDriver` メソッド追加、76行削減)
  - 全テスト成功、コードレビュー準備完了
- 2025-10-28: AKS ドライバー Azure Files 実装完了
  - `driverVolumeFiles` 構造体と全メソッド実装
  - Storage Account 自動作成、共有 CRUD 操作
  - メタデータベースの管理 (`kompox-files-share-*` タグ)
  - Handle URI 生成 (`smb://...`)
  - スナップショット非対応 (`ErrNotSupported`)
  - SMB プロトコル固定、SKU 選択サポート
- 2025-10-28: AKS ドライバー バグ修正とリファクタリング
  - リソースグループ未作成時の `DiskList`/`SnapshotList` エラー修正 (404 → 空リスト)
  - `DiskCreate` でリソースグループ自動作成を追加
  - エラーハンドリングを `errors.As` + `azcore.ResponseError` に統一
  - `DiskCreate` で `resolveSourceSnapshotResourceID` を使用 (スナップショット優先解決)
  - ソース解決メソッド (`resolveSource*`) を `volume_source.go` から `driverVolumeDisk` に移動
  - `volume_source.go` 削除 (ディスク固有ロジックの凝集度向上)

## 参考

- [K4x-ADR-014]
- [Kompox-KubeConverter.ja.md]
- [Kompox-ProviderDriver-AKS.ja.md]

[K4x-ADR-014]: ../../design/adr/K4x-ADR-014.md
[Kompox-KubeConverter.ja.md]: ../../design/v1/Kompox-KubeConverter.ja.md
[Kompox-ProviderDriver-AKS.ja.md]: ../../design/v1/Kompox-ProviderDriver-AKS.ja.md
