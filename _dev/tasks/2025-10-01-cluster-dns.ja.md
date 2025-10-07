---
id: 2025-10-01-cluster-dns
title: Cluster DNS 実処理・Usecase/CLI 実装
status: done
updated: 2025-10-06
language: ja
---

# Task: Cluster DNS 実処理・Usecase/CLI 実装

## 目的

- 既存の DNS 適用 API (Domain/Driver/Port) を実際の運用フローに組み込み、CLI から操作可能にする。
- `usecase/dns` を新設し、設定から FQDN 群を導出 → レコードセットを構築 → `ClusterPort.DNSApply` を呼び出す流れを実装する。
- 初期ターゲットとして AKS (Azure DNS) 向けの最小実装を進め、ベストエフォート/Strict/DryRun の動作を確認できる状態にする。

## スコープ / 非スコープ

- In:
  - Usecase 層の追加 (`usecase/dns`) と組み立てロジックの実装。
  - CLI コマンドの追加 (`kompoxops dns deploy` / `kompoxops dns destroy`).
  - `app deploy/destroy` への `--update-dns` 連携導入。
  - AKS ドライバでのゾーン選択/適用の最小実装 (Azure DNS 連携の骨組み).
  - DryRun/Strict の各オプションの一貫した挙動。
- Out:
  - すべてのクラウド/プロバイダのフル実装と E2E 完全化 (段階的対応).
  - 背景ガベージコレクション (孤児レコードの自動削除).
  - 複数レコードセット一括適用 API (必要になれば別タスクで).

## 設計/仕様サマリ

- Usecase (`usecase/dns`)
  - 入力: AppID (必須), CLI フラグ (`--strict`, `--dry-run`).
  - FQDN の導出: `kube.Client.IngressHostIPs()` を使用して、デプロイ済み App の実際の Ingress リソースから FQDN と IP のペアを取得。
    - `cluster.ingress.domain` は使用しない（実際の Ingress に反映されたホスト名のみを管理対象とする）。
    - `kube.Converter` を用いて namespace と label selector を取得。
  - 対象アドレス: 各 Ingress の Status.LoadBalancer.Ingress[].IP を使用（IngressHostIP.IP）。
  - レコードセットの構築: A レコードを IP アドレスで作成（`TTL==0` は既定 TTL、`RData` 空で削除）。
  - 適用: `ClusterPort.DNSApply(ctx, cluster, rset, opts...)` を FQDN 単位で呼ぶ。
  - ロギング/サマリ: 変更予定/結果をまとめて CLI に返す。

- CLI
  - `kompoxops dns deploy`: 導出したレコードセットの作成/更新 (ベストエフォート).
    - `--app` フラグで対象アプリを指定（必須）。
    - クラスタは `--cluster-name` または設定ファイルから取得し、アプリ名から AppID を解決。
  - `kompoxops dns destroy`: 決定的に紐づくレコードセットのみ削除 (A, AAAA, CNAME を試行).
    - `--app` フラグで対象アプリを指定（必須）。
  - 共通フラグ: `--strict`, `--dry-run`.
  - `app deploy` / `app destroy` から `--update-dns` 指定で AppID を直接渡して usecase を呼び出す（デフォルトの動作: best-effort, non-strict, non-dry-run）。

- Provider (AKS/Azure DNS の最小実装)
  - ゾーン選択: FQDN に対する最長一致で推定。
  - 適用セマンティクス: 既定はベストエフォート (書き込み失敗は警告、`--strict` でエラー化)。
  - DryRun: 変更を行わず、計画のみを出力。
  - 実装方針: 初期はノーオペから始め、ゾーン解決/入力検証/DryRun 表示 → 最小限の Upsert/Delete 呼び出しの順に段階実装。

- 運用タイミング/クリーンアップ (再掲)
  - `dns deploy/destroy` か、`app deploy/destroy --update-dns` の明示的な操作でのみ変更する。
  - 自動 GC は行わない。決定的に関連付け可能なレコードのみ対象とする。

## 計画 (チェックリスト)

- [x] Usecase: `usecase/dns` 追加
  - [x] FQDN と IP の収集実装 (`kube.Client.IngressHostIPs()` を使用)。
  - [x] `kube.Converter` を用いた namespace と label selector の取得。
  - [x] レコードセット構築ロジック（A レコード、TTL/RData のポリシー）をインライン化。
  - [x] `ClusterPort.DNSApply` 呼び出しとオプション適用（ZoneHint/Strict/DryRun）。
  - [x] ログ/結果サマリ出力の整形。
  - [x] KubeFactory の削除、inline kube.Client 作成パターンへの移行。

- [x] CLI: `kompoxops dns deploy/destroy`
  - [x] `cmd/kompoxops` にサブコマンド追加とフラグ受け取り。
  - [x] `usecase_builder.go` から usecase を組み立てて呼び出し。
  - [x] クラスタ名からクラスタID、アプリ名からAppIDを解決するヘルパー関数追加。
  - [x] DryRun 出力のユーザー向け整形 (計画内容の一覧化)。

- [x] CLI: `app deploy/destroy --update-dns`
  - [x] 既存 `app` コマンドに `--update-dns` を追加。
  - [x] 指定時に AppID を直接渡して `dns deploy/destroy` 相当の usecase を呼ぶ配線。

- [x] Adapters: `kube.Client.IngressHostIPs()`
  - [x] namespace と labelSelector を引数として受け取るように変更。
  - [x] IngressHost 構造体 (FQDN + IP) を返すように実装。

- Provider (AKS): ゾーン選択/適用の最小実装
  - [x] 最長一致によるゾーン解決ロジック。
  - [x] 入力検証（FQDN/Type/RData/TTL）。
  - [x] DryRun での計画出力。
  - [ ] 最小限の Upsert/Delete (内部 API 呼び出しの骨組み)。

- [x] コード整理
  - [x] `usecase/dns/collect.go` 削除 (IngressHostIPs() を使用するため不要)。
  - [x] `usecase/dns/resolver.go` 削除 (IngressHost.IP を直接使用するため不要)。
  - [x] `usecase/dns/record.go` 削除 (レコードセット構築をインライン化)。
  - [x] 日本語コメントを英語に変更。

## テスト

- ユニット
  - Usecase: 設定→FQDN 抽出→レコードセット構築のマッピング検証。
  - CLI: フラグパース、DryRun 表示の体裁。
  - Provider (AKS): ゾーン解決 (最長一致) の単体検証。

- スモーク/E2E (段階導入)
  - AKS クラスタ上で Ingress 外部 IP/Hostname を検出し、`dns deploy --dry-run` で計画が妥当であること。
  - 権限がない状態でも既定 (非 Strict) ではエラーにならないこと。
  - `--strict` 指定時は書き込み失敗がエラーになること。

## 受け入れ条件

- `kompoxops dns deploy/destroy` がビルド/実行可能で、`--dry-run` 時に計画が表示される。
- `app deploy/destroy --update-dns` で DNS 処理が呼ばれる (デフォルトの動作で実行)。
- Usecase がデプロイ済み App の Ingress リソースから FQDN と IP を `kube.Client.IngressHostIPs()` で取得し、`ClusterPort.DNSApply` を適切に呼び出す。
- 各 Ingress の Status.LoadBalancer.Ingress[].IP を使用して A レコードを作成。
- Provider (AKS) でゾーン解決と入力検証、DryRun 表示が機能する。
- 非 Strict では書き込み失敗が警告で継続、Strict ではエラー化される。
- AppID ベースの操作で、クラスタとアプリが適切に解決される。

## メモ

- ADR ([K4x-ADR-004]) は抽象レベルの決定のみを保持。実装詳細は本タスクおよび後続タスクで管理する。
- レコードセット削除は `RData` を空にする設計 (互換を維持)。
- ゾーン解決は FQDN に対する最長一致で自動推定。
- FQDN の取得は実際にデプロイされた Ingress リソースから `kube.Client.IngressHostIPs()` で行う。設定ファイルの値ではなく、実運用状態を反映する。
- IP アドレスは各 Ingress リソースの Status.LoadBalancer.Ingress[].IP から取得し、A レコードとして登録。
- 実装の簡素化のため、レコードセット構築はインライン化し、ヘルパー関数を削除。

## 進捗

- 2025-10-01: 本タスクを作成。Usecase/CLI/Provider(最小) の段階実装を開始予定。
- 2025-10-05: 基本実装完了。
  - `kube.Client.IngressHostIPs()` メソッドを追加し、実際にデプロイされた Ingress リソースから FQDN と IP を取得。
  - `usecase/dns` に deploy/destroy を実装（AppID ベース、inline kube.Client 作成）。
  - `kompoxops dns deploy/destroy` コマンドを追加（`--app`, `--strict`, `--dry-run` フラグ）。
  - `kompoxops app deploy/destroy` に `--update-dns` フラグを統合。
  - AKS driver の DNS ゾーン解決と DryRun 表示を実装。
  - 検証: `make build` と `make test` が成功。

## 参考

- ADR
  - [K4x-ADR-004]

[K4x-ADR-004]: ../../design/adr/K4x-ADR-004.md
