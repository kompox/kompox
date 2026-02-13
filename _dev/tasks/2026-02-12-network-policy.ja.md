---
id: 2026-02-12-network-policy
title: NetworkPolicy リソース出力拡張
status: done
updated: 2026-02-13
language: ja
owner:
---
# Task: NetworkPolicy リソース出力拡張

## 目的

- App 変換時に出力される NetworkPolicy について、既定の許可元 (kube-system と ingress controller namespace) に加えて、ユーザーが追加の許可元を指定できるようにする。
- App.spec.networkPolicy で許可する宛先 (プロトコル、ポート) を列挙できるようにする。
- [Kompox-KubeConverter.ja.md] の仕様を更新し、仕様と実装を一致させる。

## スコープ / 非スコープ

- In:
  - KOM の App.spec に NetworkPolicy の Ingress 許可ルールを追加する設定を追加
    - 許可元: namespaceSelector を指定可能
    - 許可先: protocol と port のリストを指定可能
  - CRD ローダー/変換 (KOM -> domain model) で設定を取り込み、kube converter の NetworkPolicy 出力に反映
  - ドキュメント更新: [Kompox-KubeConverter.ja.md]
  - テスト追加/更新: KOM 変換の取り込み、および NetworkPolicy 出力の検証
- Out:
  - kompoxops.yml 互換入力経路への同等設定追加
  - label selector 以外の高度な指定 (IPBlock、FQDN、L7 制御など)
  - Egress 制御の追加

## 仕様サマリ

- 既存の NetworkPolicy の Ingress 許可は次を維持する:
  - 同一 namespace の Pod 間通信は許可
  - kube-system からの接続は許可
  - ingress controller の namespace からの接続は許可
- 新規に App.spec.networkPolicy.ingressRules を追加する。
  - 意味: 既定の許可ルールに対して additive に追加で許可する。
  - 各ルールは "許可元" と "許可ポート" をセットで表現する。
  - 空/未指定: 既存通り。

App.spec.networkPolicy のスキーマ案

```yaml
spec:
  networkPolicy:
    ingressRules:
      - from:
          - namespaceSelector:
              matchLabels:
                kubernetes.io/metadata.name: monitoring
        ports:
          - protocol: TCP
            port: 9090
```

設計メモ

- 許可元指定は namespaceSelector を用いる。
- ports が省略された場合は、NetworkPolicy の標準仕様に従い "すべてのポート" を許可する。
  - 既定の許可元については従来通り ports なしで "すべて許可" を維持する。
- protocol は TCP/UDP/SCTP のいずれかを許可し、未指定時は TCP とする。
- `ingressRules[].from` と `ingressRules[].ports` はいずれも配列とする。
  - K8s NetworkPolicy の `spec.ingress[].from` と `spec.ingress[].ports` に 1:1 で写像できるようにする。

## 計画 (チェックリスト)

- [x] ドキュメント
  - [x] [Kompox-KubeConverter.ja.md] の NetworkPolicy セクションを更新
  - [x] KOM の App.spec スキーマ例を追加
  - [x] 例の manifest を更新 (追加許可ルールを含める)
- [x] スキーマ/モデル
  - [x] `config/crd/ops/v1alpha1/types.go`: AppSpec に networkPolicy.ingressRules 設定を追加
  - [x] selector 型の表現 (matchLabels, matchExpressions) を確定
  - [x] `domain/model/app.go`: app の NetworkPolicy 設定を保持できるように拡張
- [x] 変換
  - [x] `config/crd/ops/v1alpha1/sink_tomodels.go`: App.spec の設定を domain model に取り込む
  - [x] `adapters/kube/converter.go`: NetworkPolicy の ingress rules に selector と ports を反映
- [x] テスト
  - [x] KOM -> domain model 変換テストを追加/更新
  - [x] kube converter の NetworkPolicy 出力テストを追加/更新

## テスト

- ユニット:
  - KOM 変換: App.spec.networkPolicy.ingressRules が domain model に反映される
  - kube converter: NetworkPolicy の ingress rules に selector と ports が反映される
- スモーク:
  - `make test` が成功する

## 受け入れ条件

- App.spec.networkPolicy.ingressRules で追加した許可元が NetworkPolicy の Ingress 許可対象に含まれる。
- App.spec.networkPolicy.ingressRules.ports で追加した protocol と port が NetworkPolicy の Ingress 許可対象に反映される。
- 既定の許可 (kube-system と ingress controller namespace) は維持される。
- 設定未指定の場合は既存の出力と互換である。
- 不正な protocol/port/selector が指定された場合に、検知できる形で失敗する (エラーまたはバリデーション)。

## メモ

- namespaceSelector は `kubernetes.io/metadata.name` を使った name ベースの指定に限定しない。
- 追加ルールの導入により、広い許可ルール (ports 省略など) があると制限が効かなくなる。仕様と例で注意喚起する。

## 進捗

- 2026-02-12: タスク作成
- 2026-02-12: 許可ポート (protocol, port) と namespaceSelector 対応をスコープに追加
- 2026-02-13: 最新コミット d37f46469eb5b5d4d16f456c39980f884eff3f51 を確認。ドキュメント・スキーマ/モデル・変換・テストの実装が完了し、ステータスを done に更新

## 参考

- [Kompox-KubeConverter.ja.md]
- [Kubernetes NetworkPolicy]

[Kompox-KubeConverter.ja.md]: ../../design/v1/Kompox-KubeConverter.ja.md
[Kubernetes NetworkPolicy]: https://kubernetes.io/docs/concepts/services-networking/network-policies/
