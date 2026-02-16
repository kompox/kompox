---
id: 20260216a-nodepool-providerdriver-spec
title: NodePool 対応に向けた ProviderDriver 仕様更新 (Phase 1)
status: done
updated: 2026-02-16T15:59:31Z
language: ja
owner: yaegashi
adrs:
  - K4x-ADR-019
plans:
  - 2026ab-k8s-node-pool-support
---
# タスク: NodePool 対応に向けた ProviderDriver 仕様更新 (Phase 1)

本タスクは、Plan [2026ab-k8s-node-pool-support] の Phase 1 を具体化する作業項目です。

## 目的

- [Kompox-ProviderDriver] の公開契約を、NodePool 抽象の導入方針([K4x-ADR-019])に沿って更新する。
- 本タスクでは設計文書更新のみを行い、実装コードの変更は行わない。

## スコープ / 非スコープ

- 対象:
  - [Kompox-ProviderDriver] の Driver インターフェース仕様に NodePool 管理メソッドの分類・契約概要を追加
  - 未対応ドライバの挙動(`not implemented`)と責務境界の記述追加
  - 既存メソッド説明との整合性確認と参照更新
- 非対象:
  - `adapters/drivers/provider/**` の実装変更
  - AKS driver 実装([Kompox-ProviderDriver-AKS] 側の実装詳細追記を含む)
  - CLI/Converter の実装変更

## 仕様サマリ

- `NodePool` を Provider Driver の共通抽象として扱う。
- 追加対象メソッド(概略): `NodePoolList`, `NodePoolCreate`, `NodePoolUpdate`, `NodePoolDelete`。
- `NodePoolGet` は当面導入せず、`List + name filter` 方針とする。
- 未対応ドライバは `not implemented` を返す capability 境界として扱う。
- `要求事項(横断)` を MVP 必須項目と将来検討項目に分割し、各要求を 1 行の簡潔な解説で記述する。

### 提案シグネチャ (既存 Driver パターン準拠)

```go
NodePoolList(
    ctx context.Context,
    cluster *model.Cluster,
    opts ...model.NodePoolListOption,
) ([]*model.NodePool, error)

NodePoolCreate(
    ctx context.Context,
    cluster *model.Cluster,
    pool model.NodePool,
    opts ...model.NodePoolCreateOption,
) (*model.NodePool, error)

NodePoolUpdate(
    ctx context.Context,
    cluster *model.Cluster,
    pool model.NodePool,
    opts ...model.NodePoolUpdateOption,
) (*model.NodePool, error)

NodePoolDelete(
    ctx context.Context,
    cluster *model.Cluster,
    poolName string,
    opts ...model.NodePoolDeleteOption,
) error
```

### メソッドと DTO の対応

| Method | Input DTO | Output DTO | Purpose |
|---|---|---|---|
| `NodePoolList` | `NodePoolListOption` | `[]*NodePool` | List pools (`Get` absorbed by list + name filter) |
| `NodePoolCreate` | `NodePool`, `NodePoolCreateOption` | `*NodePool` | Create pool |
| `NodePoolUpdate` | `NodePool`, `NodePoolUpdateOption` | `*NodePool` | Patch mutable fields (non-nil fields only) |
| `NodePoolDelete` | `NodePoolDeleteOption` | none (`error` only) | Delete pool |

### DTO 概要 (MVP)

- DTO は単一の `NodePool` を使用し、Create/Update/List の全メソッドで共通化する。
- `NodePool` の主要フィールドは pointer を基本とし、`Update` では non-nil のみを適用対象とする。
  - `Name *string`
  - `ProviderName *string`
  - `Mode *string` (`system`/`user`)
  - `Labels *map[string]string` (includes `kompox.dev/node-pool`, `kompox.dev/node-zone`)
  - `Zones *[]string`
  - `InstanceType *string`
  - `OSDiskType *string`
  - `OSDiskSizeGiB *int`
  - `Priority *string` (`regular`/`spot`)
  - `Autoscaling *NodePoolAutoscaling`
  - `Status *NodePoolStatus`
  - `Extensions map[string]any`
- Create の必須項目はメソッド側バリデーションで強制する。
- Update で immutable 項目が指定された場合は validation error とする。
- `NodePoolAutoscaling`
  - `Enabled bool`
  - `Min int`
  - `Max int`
  - `Desired *int`

## 方針: NodePool 抽象とベンダ差異

- Kompox の公開契約では `NodePool` を共通語として扱い、ベンダ固有用語は driver 実装側で吸収する。
  - AKS: Agent Pool
  - EKS: Node Group
  - GKE/OKE: Node Pool
- Pod スケジューリングは Kompox ラベルを一次契約とする。
  - `kompox.dev/node-pool`
  - `kompox.dev/node-zone`
- zone 値の正規化・変換は provider driver の責務とし、Converter 側は入力意図の反映に専念する。
- ベンダ方言のパラメータ名は DTO へ持ち込まず、driver 側で変換する。
  - 例: AKS `vmSize` は Kompox `InstanceType` にマッピングする。
- DTO 種類は増やさず、単一 `NodePool` DTO で Create/Update/List を表現する。
- 未対応機能の扱いは次で統一する。
  - provider が機能自体を持たない: `not implemented`
  - provider は機能を持つが対象項目が不正/不可変: validation error

## 計画 (チェックリスト)

- [x] [Kompox-ProviderDriver] の現行契約と記述位置を確認する。
- [x] NodePool 管理カテゴリのメソッド契約(概要)を追記する。
- [x] エラーモデル(検証エラーと `not implemented`)の使い分け方針を追記する。
- [x] `要求事項(横断)` を MVP 必須/将来検討に分割し、各項目を簡潔に整形する。
- [x] 既存セクションとの重複・矛盾を解消し、参照リンクを更新する。
- [x] `make gen-index` を実行してインデックスを更新する。

## テスト

- ユニット: なし (docs-only)
- スモーク:
  - `make gen-index` が成功する。
  - `design/index.json` と `design/tasks/index.json` に task が反映される。

## 受け入れ条件

- [Kompox-ProviderDriver] に NodePool 管理メソッドの仕様が追加されている。
- [K4x-ADR-019] と矛盾しない契約記述になっている。
- 本タスクの範囲で実装コードが変更されていない。

## 備考

- リスク:
  - 将来の AKS 実装詳細を先に書きすぎると、設計文書の責務境界が崩れる。
- フォローアップ:
  - 次タスクで [Kompox-ProviderDriver-AKS] 側の実装方針詳細とテスト方針を追加する。

## 進捗

- 2026-02-16T12:04:26Z タスクファイルを作成
- 2026-02-16T12:12:41Z NodePool 抽象とベンダ差異吸収方針のセクションを追加
- 2026-02-16T12:18:12Z 提案メソッドシグネチャとメソッド/DTO 対応表を追加
- 2026-02-16T12:31:26Z DTO フィールド名をベンダ中立に更新し、AKS マッピング注記を追加
- 2026-02-16T12:41:01Z 単一 NodePool DTO 方針へ変更 (ポインタベース適用)
- 2026-02-16T14:49:38Z cloud agent コミット `668752b` を確認し、タスクを done に更新
- 2026-02-16T15:43:42Z `要求事項(横断)` を MVP 必須/将来検討に再構成し、各項目を 1 行で簡潔化

## 参照

- [2026ab-k8s-node-pool-support]
- [K4x-ADR-019]
- [Kompox-ProviderDriver]
- [Kompox-ProviderDriver-AKS]
- [design/tasks/README.md]

[2026ab-k8s-node-pool-support]: ../../../plans/2026/2026ab-k8s-node-pool-support.ja.md
[K4x-ADR-019]: ../../../adr/K4x-ADR-019.md
[Kompox-ProviderDriver]: ../../../v1/Kompox-ProviderDriver.ja.md
[Kompox-ProviderDriver-AKS]: ../../../v1/Kompox-ProviderDriver-AKS.ja.md
[design/tasks/README.md]: ../../README.md
