---
id: 20260214b-box-kom-loader
title: Box KOM 定義とロード時バリデーション
status: draft
updated: 2026-02-14T13:30:32Z
language: ja
owner: yaegashi
---
# Task: Box KOM 定義とロード時バリデーション

## 目的

- [2026aa-kompox-box-update.ja.md] の Phase 1〜3 のうち、既存機能に影響を出さない範囲で Box の「定義」「読み込み」「バリデーション」を実装する。
- App に Box が存在しない場合は、現行の挙動(=単一 component `app`)が変わらないことを保証する。

## スコープ/非スコープ

- In:
  - Box の CRD 型定義(`BoxSpec`)の placeholder を最小の v1 として拡張
  - KOM ローダーで Box を読み込み可能にする(既存の multi-doc YAML 読み込み経路に乗せる)
  - ロード時の静的バリデーション(Box 単体 + 親子関係 + 一部の Box 種別ルール)
  - 既存の KOM テスト資産へ Box を追加した場合の回帰防止
- Out:
  - Converter の出力を Box/component 単位に分割する変更
  - ingress 配賦 / networkPolicy のマージ規則 / Compose topology 解析の本実装
  - CLI セレクタ(`--component/--pod/--container`)の刷新

## 仕様サマリ

- Box は App 配下の deployable unit(component) として扱う。
- Box の種類は `spec.image` の有無で判定する。
  - `spec.image` あり: Standalone Box
  - `spec.image` なし: Compose Box
- componentName は `metadata.name` を canonical とする。
  - `spec.component` は互換のために保持しうるが、指定する場合は `metadata.name` と一致することを必須にする。
- `metadata.name` は DNS-1123 label、かつ予約語 `app` を禁止する。

注記: Compose service の存在検証や `network_mode` の closure 解析など、App.spec.compose の内容に依存する検証は Phase 4 以降へ送る(本タスクでは「Box 単体で完結する検証」を先に固める)。

## 計画 (チェックリスト)

- [ ] `config/crd/ops/v1alpha1/types.go` の `BoxSpec` を placeholder から v1 最小へ拡張
- [ ] Box のロード時バリデーションを追加
  - [ ] `metadata.name` の制約(DNS-1123, `app` 禁止)
  - [ ] `spec.component` がある場合の一致制約
  - [ ] `spec.image` の有無による Compose/Standalone の形状チェック
    - Compose Box: `image/command/args/ingress` を禁止
    - Standalone Box: `image` 必須、`ingress` は禁止(予約)
- [ ] `cmd/kompoxops/kom_loader.go` の KOM 初期化で、Box を含む入力をロード・検証できることを確認
- [ ] テストを追加/更新
  - [ ] `config/crd/ops/v1alpha1` の loader/validator テストに Box の正常系/異常系を追加
  - [ ] 既存 E2E(KOM ローダー系)に Box を混ぜても既存パスが壊れないこと

## テスト

- ユニット:
  - `config/crd/ops/v1alpha1` の loader/validator テストで Box の代表ケースを網羅
- スモーク:
  - `make test` が通る
  - Box を含まない既存の KOM フィクスチャ/統合テストが通る

## 受け入れ条件

- Box が存在しない App の挙動/既存テストが変化しない。
- Box が存在する場合、ロード時に明確で再現性のあるエラーメッセージで不正入力を検出できる。
- 仕様の一次参照は [2026aa-kompox-box-update.ja.md] とし、実装がそれと矛盾しない。

## メモ

- リスク: 既存 `kompoxops box` の前提と BoxSpec が衝突する可能性がある。
  - 本タスクでは Converter/CLI の挙動変更は避け、あくまで「KOM として Box を表現できる土台」を優先する。

## 進捗

- 2026-02-14: タスク作成

## 参考

- [K4x-ADR-017]
- [2026aa-kompox-box-update.ja.md]
- [Kompox-KOM.ja.md]
- [config/crd/ops/v1alpha1/types.go]
- [cmd/kompoxops/kom_loader.go]

[K4x-ADR-017]: ../../../../adr/K4x-ADR-017.md
[2026aa-kompox-box-update.ja.md]: ../../../../plans/2026/2026aa-kompox-box-update.ja.md
[Kompox-KOM.ja.md]: ../../../../v1/Kompox-KOM.ja.md
[config/crd/ops/v1alpha1/types.go]: ../../../../../config/crd/ops/v1alpha1/types.go
[cmd/kompoxops/kom_loader.go]: ../../../../../cmd/kompoxops/kom_loader.go
