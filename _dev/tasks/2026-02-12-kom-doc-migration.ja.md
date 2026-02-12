---
id: 2026-02-12-kom-doc-migration
title: kompoxops.yml ドキュメント廃止と KOM ベース化
status: active
updated: 2026-02-12
language: ja
owner: yaegashi
---
# Task: kompoxops.yml ドキュメント廃止と KOM ベース化

## 目的

- ドキュメント上の主要な設定形式を `kompoxops.yml` から KOM (Workspace/Provider/Cluster/App) に移行する。
- [Kompox-KubeConverter.ja.md] の入力説明を KOM ベースに書き換え、変換結果の公開契約(生成マニフェストの性質)は維持する。
- `kompoxops.yml` は互換のため残しつつ、ドキュメント上は deprecated と明記し、新規利用を推奨しない状態にする。

## スコープ / 非スコープ

- In:
  - ドキュメント修正のみ(仕様・実装の変更なし)
  - [Kompox-KubeConverter.ja.md] の Example Input を KOM に置き換え
  - [README.md] / [README.ja.md] の主要説明を KOM/kompoxapp.yml ベースへ更新
  - [Kompox-CLI.ja.md] の `kompoxops` 概要を KOM モード中心へ整合
  - [config/crd/ops/v1alpha1/README.md] の注釈キー表記などを実装/設計に合わせて整合
  - `kompoxops.yml` を前提にした古い設計文書には Notice を追加(ただし archived 前提)
- Out:
  - CLI の挙動変更(警告追加、既定値変更、単一ファイルモード削除など)
  - E2E テストテンプレートの移行(kompoxops.yml.in -> KOM 形式など)
  - 仕様の破壊的変更(KOM スキーマや変換ロジックの変更)

## 仕様サマリ

- KOM を primary として扱う。
  - KOM の定義は [Kompox-KOM.ja.md] を一次参照とする。
  - CLI の読み込みフローは [Kompox-CLI.ja.md] の KOM モード定義に従う。
- `kompoxops.yml` は単一ファイルモードとして deprecated。
  - ドキュメントの残存箇所は「互換用(廃止予定)」として扱い、新規の主経路として説明しない。
- [Kompox-KubeConverter.ja.md] の入力例は KOM (multi-document YAML) を採用し、生成される Kubernetes Manifest 例は現状の公開契約を維持する。

## 計画(チェックリスト)

- [ ] [Kompox-KubeConverter.ja.md] の `### kompoxops.yml` を KOM 例に置換
  - [ ] Workspace/Provider/Cluster/App を multi-document YAML で提示
  - [ ] `metadata.annotations["ops.kompox.dev/id"]` (Resource ID) を用いた例に統一
  - [ ] 以降の説明で `kompoxops.yml` 前提の文言を KOM 前提へ修正
- [ ] [README.md] と [README.ja.md] を KOM/kompoxapp.yml 前提へ更新
  - [ ] `kompoxops.yml` を primary として紹介しない
  - [ ] 互換としての単一ファイルモードは短い注記に留める
- [ ] [Kompox-CLI.ja.md] の冒頭説明を KOM 中心へ整合
  - [ ] `kompoxops` の説明を「KOM を読み込んで動作する CLI」へ変更
  - [ ] 単一ファイルモードは「廃止予定」として位置付けを維持
- [ ] [config/crd/ops/v1alpha1/README.md] の記載を `ops.kompox.dev/id` ベースに整合
  - [ ] `ops.kompox.dev/path` など誤解を招く表現があれば整理
  - [ ] `doc-path` / `doc-index` などの説明を必要最小限で追記
- [ ] 古い設計文書(archived)の `kompoxops.yml` 前提箇所に Notice を追加
  - [ ] [Kompox-Spec-Draft.ja.md] など、誤読を防ぐための冒頭注記

## テスト

- ドキュメント整合:
  - `kompoxops.yml` が primary として紹介されていないこと(例: README と KubeConverter guide)
  - KOM の注釈キーが設計/実装と一致していること(`ops.kompox.dev/id`)
- インデックス更新:
  - `make gen-index` が成功すること

## 受け入れ条件

- [Kompox-KubeConverter.ja.md] が KOM ベースの入力説明になっている。
- [README.md] / [README.ja.md] が KOM ベースの説明になっている。
- [Kompox-CLI.ja.md] の `kompoxops` の概要が KOM ベースになっている(単一ファイルモードは deprecated 扱い)。
- [config/crd/ops/v1alpha1/README.md] の説明が設計([Kompox-KOM.ja.md])および実装と矛盾しない。
- `kompoxops.yml` への言及は deprecated/legacy 文脈に限定される(新規利用を促さない)。

## メモ

- 互換性(単一ファイルモード)は本タスクでは削除しない。削除や既定値変更は別タスクで扱う。
- 既存のテストやテンプレートが `kompoxops.yml` を生成/参照する場合があるため、docs-only でも「文言の一貫性」には注意する。

## 進捗

- 2026-02-12: タスク作成

## 参考

- [Kompox-KubeConverter.ja.md]
- [Kompox-KOM.ja.md]
- [Kompox-CLI.ja.md]
- [README.md]
- [README.ja.md]
- [config/crd/ops/v1alpha1/README.md]

[Kompox-KubeConverter.ja.md]: ../../design/v1/Kompox-KubeConverter.ja.md
[Kompox-KOM.ja.md]: ../../design/v1/Kompox-KOM.ja.md
[Kompox-CLI.ja.md]: ../../design/v1/Kompox-CLI.ja.md
[Kompox-Spec-Draft.ja.md]: ../../design/v1/Kompox-Spec-Draft.ja.md
[README.md]: ../../README.md
[README.ja.md]: ../../README.ja.md
[config/crd/ops/v1alpha1/README.md]: ../../config/crd/ops/v1alpha1/README.md
