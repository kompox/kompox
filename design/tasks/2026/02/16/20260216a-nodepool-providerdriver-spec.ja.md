---
id: 20260216a-nodepool-providerdriver-spec
title: NodePool 対応に向けた ProviderDriver 仕様更新 (Phase 1)
status: active
updated: 2026-02-16T12:41:01Z
language: ja
owner: yaegashi
adrs:
  - K4x-ADR-019
plans:
  - 2026ab-k8s-node-pool-support
---
# Task: NodePool 対応に向けた ProviderDriver 仕様更新 (Phase 1)

## Goal

- [Kompox-ProviderDriver] の公開契約を、NodePool 抽象の導入方針([K4x-ADR-019])に沿って更新する。
- 本タスクでは設計文書更新のみを行い、実装コードの変更は行わない。

## Scope / Out of scope

- In:
  - [Kompox-ProviderDriver] の Driver インターフェース仕様に NodePool 管理メソッドの分類・契約概要を追加
  - 未対応ドライバの挙動(`not implemented`)と責務境界の記述追加
  - 既存メソッド説明との整合性確認と参照更新
- Out:
  - `adapters/drivers/provider/**` の実装変更
  - AKS driver 実装([Kompox-ProviderDriver-AKS] 側の実装詳細追記を含む)
  - CLI/Converter の実装変更

## Spec summary

- `NodePool` を Provider Driver の共通抽象として扱う。
- 追加対象メソッド(概略): `NodePoolList`, `NodePoolCreate`, `NodePoolUpdate`, `NodePoolDelete`。
- `NodePoolGet` は当面導入せず、`List + name filter` 方針とする。
- 未対応ドライバは `not implemented` を返す capability 境界として扱う。

### Proposed signatures (aligned with existing Driver patterns)

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

### Method and DTO mapping

| Method | Input DTO | Output DTO | Purpose |
|---|---|---|---|
| `NodePoolList` | `NodePoolListOption` | `[]*NodePool` | List pools (`Get` absorbed by list + name filter) |
| `NodePoolCreate` | `NodePool`, `NodePoolCreateOption` | `*NodePool` | Create pool |
| `NodePoolUpdate` | `NodePool`, `NodePoolUpdateOption` | `*NodePool` | Patch mutable fields (non-nil fields only) |
| `NodePoolDelete` | `NodePoolDeleteOption` | none (`error` only) | Delete pool |

### DTO outline (MVP)

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

## Policy: NodePool abstraction and vendor differences

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

## Plan (checklist)

- [ ] [Kompox-ProviderDriver] の現行契約と記述位置を確認する。
- [ ] NodePool 管理カテゴリのメソッド契約(概要)を追記する。
- [ ] エラーモデル(検証エラーと `not implemented`)の使い分け方針を追記する。
- [ ] 既存セクションとの重複・矛盾を解消し、参照リンクを更新する。
- [ ] `make gen-index` を実行してインデックスを更新する。

## Tests

- Unit: なし (docs-only)
- Smoke:
  - `make gen-index` が成功する。
  - `design/index.json` と `design/tasks/index.json` に task が反映される。

## Acceptance criteria

- [Kompox-ProviderDriver] に NodePool 管理メソッドの仕様が追加されている。
- [K4x-ADR-019] と矛盾しない契約記述になっている。
- 本タスクの範囲で実装コードが変更されていない。

## Notes

- Risks:
  - 将来の AKS 実装詳細を先に書きすぎると、設計文書の責務境界が崩れる。
- Follow-ups:
  - 次タスクで [Kompox-ProviderDriver-AKS] 側の実装方針詳細とテスト方針を追加する。

## Progress

- 2026-02-16T12:04:26Z Task file created
- 2026-02-16T12:12:41Z Added policy section for NodePool abstraction and vendor-difference handling
- 2026-02-16T12:18:12Z Added proposed method signatures and method/DTO mapping table
- 2026-02-16T12:31:26Z Updated DTO fields to vendor-neutral names (InstanceType, OSDisk*, Priority) and added AKS mapping note
- 2026-02-16T12:41:01Z Switched to single NodePool DTO policy (pointer-based fields across methods)

## References

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
