---
id: 20260227a-oke-driver-design-doc
title: OKE Driver 設計基準ドキュメント作成
status: active
updated: 2026-02-27T12:34:00Z
language: ja
owner:
adrs: []
plans:
  - 2026ad-oke-driver-implementation
supersedes: []
supersededBy:
---
# タスク: OKE Driver 設計基準ドキュメント作成

本タスクは Plan [2026ad-oke-driver-implementation] の Phase 1 として、OKE Provider Driver の実装基準となる設計ドキュメントを新規作成する。

## 目的

- `design/v1/Kompox-ProviderDriver-OKE.ja.md` を作成し、OKE Driver 実装時の参照先を一本化する。
- [Kompox-ProviderDriver-OKE-DesignStudy] に分散している検討内容を、MVP 実装向けの実装規約として再編する。
- MVP 範囲と未対応境界を明示し、以降の実装タスクで解釈差分が出ない状態を作る。

## スコープ/非スコープ

- In:
  - `design/v1/Kompox-ProviderDriver-OKE.ja.md` の新規作成。
  - 認証、ステート管理、`ensure*()` 方針、破壊順序、`not implemented` 境界の明文化。
  - MVP 対象（`instance_principal` / `user_principal`、CA 非対応、OCIR 詳細後続）の明記。
- Out:
  - OKE Driver 実装コードの追加。
  - CLI / usecase / domain 契約の変更。
  - OKE 以外の provider 向け設計更新。

## 仕様サマリ

- 新規ドキュメントは [Kompox-ProviderDriver] の契約を前提に、OKE 固有マッピングのみを追加する。
- 参照元は [Kompox-ProviderDriver-OKE-DesignStudy] を一次情報源とし、実装判断に必要な内容へ圧縮する。
- 既存参照実装として [Kompox-ProviderDriver-AKS] を併記し、責務分割・命名・ログ方針の比較軸を明示する。
- 未決事項は「MVP対象外」または「後続判断」のどちらかに分類して記載する。

## 計画 (チェックリスト)

- [ ] `design/v1/Kompox-ProviderDriver-OKE.ja.md` を新規作成する。
- [ ] MVP 対象機能と対象外機能を章立てで明確化する。
- [ ] OKE 固有の実装規約（認証/ステート/ライフサイクル順序）を記述する。
- [ ] 参照リンク（plan/design docs）を整理し、doc-id ルールを満たす。
- [ ] `make gen-index` 実行後、index 反映を確認する。

## テスト

- スモーク:
  - `make gen-index`
- ドキュメント確認:
  - front matter の `id` とファイル名 stem が一致している。
  - 参照リンクが doc-id ベースで定義されている。

## 受け入れ条件

- `design/v1/Kompox-ProviderDriver-OKE.ja.md` が作成され、MVP 実装判断に必要な方針が記載されている。
- [2026ad-oke-driver-implementation] の Phase 1 の内容と整合している。
- 本タスクが index に取り込まれ、task/doc 間リンクが解決できる。

## メモ

- リスク:
  - 設計検討文書からの転載が過剰になると、MVP の判断軸が不明瞭になる。
- フォローアップ:
  - Phase 2 で [Kompox-ProviderDriver] との契約整合レビューを実施する。

## 進捗

- 2026-02-27T12:34:00Z タスクファイル作成

## 参照

- [2026ad-oke-driver-implementation]
- [Kompox-ProviderDriver-OKE-DesignStudy]
- [Kompox-ProviderDriver]
- [Kompox-ProviderDriver-AKS]
- [design/tasks/README.md]

[2026ad-oke-driver-implementation]: ../../../../plans/2026/2026ad-oke-driver-implementation.ja.md
[Kompox-ProviderDriver-OKE-DesignStudy]: ../../../../v1/Kompox-ProviderDriver-OKE-DesignStudy.ja.md
[Kompox-ProviderDriver]: ../../../../v1/Kompox-ProviderDriver.ja.md
[Kompox-ProviderDriver-AKS]: ../../../../v1/Kompox-ProviderDriver-AKS.ja.md
[design/tasks/README.md]: ../../../README.md
