---
id: 20260215c-networkpolicy-consolidate-namespace-selector
title: "NetworkPolicy の namespaceSelector 集約"
status: done
updated: 2026-02-15T11:02:34Z
language: ja
owner: yaegashi
adrs: []
plans: []
supersedes: []
supersededBy:
---
# Task: NetworkPolicy の namespaceSelector 集約

## 目的

- デフォルト NetworkPolicy の `spec.ingress[].from` で分割されている namespaceSelector を、意図を維持したまま 1 peer に集約して表現を簡潔化する。
- `from: - {}` 回避の要件を維持し、出力 YAML の可読性を向上させる。

## スコープ/非スコープ

- In:
  - `adapters/kube/converter.go` のデフォルト ingress peer 生成ロジックを集約形に変更する。
  - 既存回帰テストを集約後の表現に合わせて必要最小限更新する。
- Out:
  - ユーザー定義 NetworkPolicy ルール変換ロジックの仕様変更。
  - 許可 namespace の増減やセキュリティポリシーの方針変更。

## 仕様サマリ

- `kubernetes.io/metadata.name In [...]` の `values` に `app namespace + kube-system + ingress namespace` をまとめ、`from` を 1 peer で表現する。
- 生成結果に empty peer (`{}`) が含まれないことを維持する。

## NetworkPolicy manifest 比較 (fixture: 20260215b)

### 改善前 (現行: `kompoxops -C ./tests/fixtures/20260215b-networkpolicy-empty-peer app validate --out-manifest -`)

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: app1
  namespace: k4x-32kff1-app1-k9x84f
spec:
  ingress:
    - from:
        - namespaceSelector:
            matchExpressions:
              - key: kubernetes.io/metadata.name
                operator: In
                values:
                  - k4x-32kff1-app1-k9x84f
        - namespaceSelector:
            matchExpressions:
              - key: kubernetes.io/metadata.name
                operator: In
                values:
                  - kube-system
                  - traefik
  policyTypes:
    - Ingress
```

### 改善後 (集約後)

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: app1
  namespace: k4x-32kff1-app1-k9x84f
spec:
  ingress:
    - from:
        - namespaceSelector:
            matchExpressions:
              - key: kubernetes.io/metadata.name
                operator: In
                values:
                  - k4x-32kff1-app1-k9x84f
                  - kube-system
                  - traefik
  policyTypes:
    - Ingress
```

## 計画 (チェックリスト)

- [x] デフォルト ingress peer の `namespaceSelector` を 1 本へ集約する。
- [x] 集約後も重複 namespace を除外する。
- [x] 回帰テストを更新し、`from` 集約表現と empty peer 非出力を確認する。
- [x] `go test ./adapters/kube -run NetworkPolicy` で回帰確認する。

## テスト

- ユニット:
  - `go test ./adapters/kube -run NetworkPolicy`
- スモーク:
  - `go test ./...` (必要時)

## 受け入れ条件

- デフォルト NetworkPolicy の `spec.ingress[0].from` が 1 peer で出力される。
- 当該 peer の `namespaceSelector` が `kubernetes.io/metadata.name In [...]` で app namespace / `kube-system` / ingress namespace を含む。
- 生成 manifest に `from: - {}` が出力されない。
- 既存の NetworkPolicy 関連テストが成功する。

## メモ

- リスク:
  - 表現を集約した際に、テストが peer 分割前提だと誤検知する可能性がある。
- フォローアップ:
  - 必要に応じて `20260215b` との関係を記録し、最終的に status を更新する。

## 進捗

- 2026-02-15T10:53:30Z タスクファイル作成
- 2026-02-15T10:55:56Z 20260215b fixture の現行 manifest を採取し、改善前/改善後の NetworkPolicy 比較を追記
- 2026-02-15T11:02:34Z `adapters/kube/converter.go` で default ingress peer を1本化し、`converter_test.go` の既存/回帰テストを更新、`go test ./adapters/kube -run NetworkPolicy` 成功

## 参照

- [20260215b-networkpolicy-empty-peer]

[20260215b-networkpolicy-empty-peer]: ./20260215b-networkpolicy-empty-peer.ja.md
