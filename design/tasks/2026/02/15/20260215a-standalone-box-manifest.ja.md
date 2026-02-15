---
id: 20260215a-standalone-box-manifest
title: Standalone Box の K8s Manifest 化 (Phase 4)
status: done
updated: 2026-02-15T05:59:42Z
language: ja
owner: yaegashi
adrs:
  - K4x-ADR-017
plans:
  - 2026aa-kompox-box-update
---
# Task: Standalone Box の K8s Manifest 化 (Phase 4)

## Goal

- [2026aa-kompox-box-update] の Phase 4 を実装し、Standalone Box (`spec.image` あり) を K8s Manifest として生成できる土台を整える。
- Compose Box 実装 (Phase 7 以降) に影響を出さず、段階的に進められる実装境界を確立する。

## Scope / Out of scope

- In:
  - Box を domain model/repository で扱えるようにする。
  - `spec.image` を持つ Box を converter 入力へ反映できる構造を導入する。
  - 既存挙動 (`Box` 未定義時の component=`app`) を維持するための回帰確認を行う。
- Out:
  - `kompoxops app` 側の component 選択 UI/CLI 拡張 (Phase 5/6)。
  - Compose Box の service 割り当てロジック (Phase 7)。
  - Ingress 配賦および NetworkPolicy 全面適用の本実装 (Phase 8/9)。

## Spec summary

- Standalone Box のライフサイクル操作 (`deploy`/`destroy`/`status`) は `kompoxops box` を正規経路として維持する。
- 本タスクでは「K8s Manifest 化できる状態」に焦点を当て、アプリ運用系 CLI の統合は次フェーズへ分離する。

## Plan (checklist)

- [x] domain に Box モデル/Repository を追加し、KOM `Sink.ToModels` から取り出せるようにする。
- [x] Standalone Box を converter 入力に渡すためのデータ経路を追加する。
- [x] Standalone Box の Manifest 生成 (最低限 Deployment + 必要な周辺リソース) を実装する。
- [x] Box 未定義 App の既存出力が変わらないことを回帰テストで確認する。
- [x] 失敗時エラー (不正 Box 参照など) が再現性あるメッセージになるよう整える。

## Tests

- Unit:
  - Box model/repository 変換と converter 入力構築のテストを追加。
- Smoke:
  - `go test ./config/crd/ops/v1alpha1 ./usecase/... ./adapters/kube/...`
  - 既存の `app` 単体経路で回帰がないことを確認。

## Acceptance criteria

- Standalone Box を含む KOM から、Phase 4 の範囲で必要な Manifest 生成経路が機能する。
- Box 未定義 App の生成物・挙動が従来から変化しない。
- 本タスクが完了しても、Compose Box 実装に未着手であることが明確である。

## Notes

- Risks:
  - domain/repository 拡張の影響範囲が広く、既存 usecase の依存更新が必要になる可能性がある。
- Follow-ups:
  - Phase 5 で `kompoxops app` 側の component 単位運用へ接続する。

## Progress

- 2026-02-15: タスク作成
- 2026-02-15: Cloud agent による実装
  - https://github.com/kompox/kompox/pull/6
- 2026-02-15: 実装完了
  - Box の domain model/repository と in-memory/RDB 実装を追加
  - CRD sink から Box を model へ変換する経路を追加
  - converter に Standalone Box 生成経路を追加
  - app 側の経路で Standalone Box を生成しない境界を再確認・修正
- 2026-02-15: 完了反映
  - 計画チェックリストを完了に更新
  - front-matter を `status: done` に更新

## References

- [K4x-ADR-017]
- [2026aa-kompox-box-update]
- [20260214b-box-kom-loader]

[K4x-ADR-017]: ../../../../adr/K4x-ADR-017.md
[2026aa-kompox-box-update]: ../../../../plans/2026/2026aa-kompox-box-update.ja.md
[20260214b-box-kom-loader]: ../14/20260214b-box-kom-loader.ja.md
