---
id: Kompox-CRD
title: Kompox CRD-style configuration
version: v1
status: draft
updated: 2025-10-13
language: ja
---

# Kompox CRD-style configuration

## 概要

Kompox v1 CLI および v2 Operator で共通利用可能な Kubernetes CRD 互換の設定ファイルを定義します。

本書の目的:
- v1 CLI の「ファイルスキャン/インポート」モードで扱う YAML の正規仕様を定義
- 将来の v2 Operator が同一 G/V/K を Watch できるように、K8s CRD スタイルへ整形
- ID(FQN)、参照、検証、K8s 内部表現(ラベル/Namespace)の不変ルールを明文化

基本方針:
- Group: ops.kompox.dev / Version: v1alpha1
- Kind: Workspace / Provider / Cluster / App / Box
- インポートファイルは annotations["ops.kompox.dev/path"] により上位スコープを唯一の略記で指定
- 参照整合に失敗した場合は DB を更新しない(オール・オア・ナッシング)

## Portable YAML format

インポート対象 YAML は、各 Kind の親スコープを annotations の唯一キーで指定します。

- キー: `metadata.annotations["ops.kompox.dev/path"]`
- 値の形式(スラッシュ区切り):
  - Provider: `<ws>`
  - Cluster: `<ws>/<prv>`
  - App: `<ws>/<prv>/<cls>`
  - Box: `<ws>/<prv>/<cls>/<app>`
- Workspace は親参照を持たないため指定不要

注記:
- ディレクトリ配置や CLI フラグからの推測は行いません(本仕様では略記は path のみ)
- すべての YAML ドキュメントは apiVersion/kind/metadata.name を必須とします


```yaml
apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: <wsName>
spec: {}
```

```yaml
apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: <prvName>
  annotations:
    ops.kompox.dev/path: <wsName>
spec: {}
```

```yaml
apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  name: <clsName>
  annotations:
    ops.kompox.dev/path: <wsName>/<prvName>
spec: {}
```

```yaml
apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  name: <appName>
  annotations:
    ops.kompox.dev/path: <wsName>/<prvName>/<clsName>
spec: {}
```

```yaml
apiVersion: ops.kompox.dev/v1alpha1
kind: Box
metadata:
  name: <boxName>
  annotations:
    ops.kompox.dev/path: <wsName>/<prvName>/<clsName>/<appName>
spec: {}
```

## K8s CRD/NS internal representation

```yaml
apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  namespace: k4x-system
  name: <wsName>
  labels:
    app.kubernetes.io/managed-by: kompox
    ops.kompox.dev/workspace: <wsName>
    ops.kompox.dev/workspace-hash: <wsHash>
status:
  opsNamespace: k4x-ws-<wsHash>-<wsName>
spec: {}
---
apiVersion: v1
kind: Namespace
metadata:
  name: k4x-ws-<wsHash>-<wsName>
  labels:
    app.kubernetes.io/managed-by: kompox
    ops.kompox.dev/workspace: <wsName>
    ops.kompox.dev/workspace-hash: <wsHash>
```

```yaml
apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  namespace: k4x-ws-<wsHash>-<wsName>
  name: <prvName>
  labels:
    app.kubernetes.io/managed-by: kompox
    ops.kompox.dev/workspace: <wsName>
    ops.kompox.dev/provider: <prvName>
    ops.kompox.dev/workspace-hash: <wsHash>
    ops.kompox.dev/provider-hash: <prvHash>
status:
  opsNamespace: k4x-prv-<prvHash>-<prvName>
spec: {}
---
apiVersion: v1
kind: Namespace
metadata:
  name: k4x-prv-<prvHash>-<prvName>
  labels:
    app.kubernetes.io/managed-by: kompox
    ops.kompox.dev/workspace: <wsName>
    ops.kompox.dev/provider: <prvName>
    ops.kompox.dev/workspace-hash: <wsHash>
    ops.kompox.dev/provider-hash: <prvHash>
```

```yaml
apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  namespace: k4x-prv-<prvHash>-<prvName>
  name: <clsName>
  labels:
    app.kubernetes.io/managed-by: kompox
    ops.kompox.dev/workspace: <wsName>
    ops.kompox.dev/provider: <prvName>
    ops.kompox.dev/cluster: <clsName>
    ops.kompox.dev/workspace-hash: <wsHash>
    ops.kompox.dev/provider-hash: <prvHash>
    ops.kompox.dev/cluster-hash: <clsHash>
status:
  opsNamespace: k4x-cls-<clsHash>-<clsName>
spec: {}
---
apiVersion: v1
kind: Namespace
metadata:
  name: k4x-cls-<clsHash>-<clsName>
  labels:
    app.kubernetes.io/managed-by: kompox
    ops.kompox.dev/workspace: <wsName>
    ops.kompox.dev/provider: <prvName>
    ops.kompox.dev/cluster: <clsName>
    ops.kompox.dev/workspace-hash: <wsHash>
    ops.kompox.dev/provider-hash: <prvHash>
    ops.kompox.dev/cluster-hash: <clsHash>
```

```yaml
apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  namespace: k4x-cls-<clsHash>-<clsName>
  name: <appName>
  labels:
    app.kubernetes.io/managed-by: kompox
    ops.kompox.dev/workspace: <wsName>
    ops.kompox.dev/provider: <prvName>
    ops.kompox.dev/cluster: <clsName>
    ops.kompox.dev/app: <appName>
    ops.kompox.dev/workspace-hash: <wsHash>
    ops.kompox.dev/provider-hash: <prvHash>
    ops.kompox.dev/cluster-hash: <clsHash>
    ops.kompox.dev/app-hash: <appHash>
status:
  opsNamespace: k4x-app-<appHash>-<appName>
spec: {}
---
apiVersion: v1
kind: Namespace
metadata:
  name: k4x-app-<appHash>-<appName>
  labels:
    app.kubernetes.io/managed-by: kompox
    ops.kompox.dev/workspace: <wsName>
    ops.kompox.dev/provider: <prvName>
    ops.kompox.dev/cluster: <clsName>
    ops.kompox.dev/app: <appName>
    ops.kompox.dev/workspace-hash: <wsHash>
    ops.kompox.dev/provider-hash: <prvHash>
    ops.kompox.dev/cluster-hash: <clsHash>
    ops.kompox.dev/app-hash: <appHash>
```

```yaml
apiVersion: ops.kompox.dev/v1alpha1
kind: Box
metadata:
  namespace: k4x-app-<appHash>-<appName>
  name: <boxName>
  labels:
    app.kubernetes.io/managed-by: kompox
    ops.kompox.dev/workspace: <wsName>
    ops.kompox.dev/provider: <prvName>
    ops.kompox.dev/cluster: <clsName>
    ops.kompox.dev/app: <appName>
    ops.kompox.dev/box: <boxName>
    ops.kompox.dev/workspace-hash: <wsHash>
    ops.kompox.dev/provider-hash: <prvHash>
    ops.kompox.dev/cluster-hash: <clsHash>
    ops.kompox.dev/app-hash: <appHash>
    ops.kompox.dev/box-hash: <boxHash>
spec: {}
```

```
wsHash = ShortHash("<wsName>")
prvHash = ShortHash("<wsName>/<prvName>")
clsHash = ShortHash("<wsName>/<prvName>/<clsName>")
appHash = ShortHash("<wsName>/<prvName>/<clsName>/<appName>")
boxHash = ShortHash("<wsName>/<prvName>/<clsName>/<appName>/<boxName>")
```

## ID・FQN・ストア

- 正規 ID は FQN(path) とします。
  - WorkspaceID: `ws`
  - ProviderID: `ws/prv`
  - ClusterID: `ws/prv/cls`
  - AppID: `ws/prv/cls/app`
  - BoxID: `ws/prv/cls/app/box`
- adapter/store(inmem, rdb) は上記 FQN を主キーとして格納します。
- 将来、リネーム後も同一 ID を維持する必要が生じた場合は「UUID(v4) 主キー + FQN UNIQUE」への移行を検討します。

## ローダー(インポート)仕様

- 入力: ファイル/ディレクトリを受け取り、拡張子 .yml/.yaml を再帰的に走査する。マルチドキュメントのYAMLファイルに対応する。
- 受理条件: `apiVersion=ops.kompox.dev/v1alpha1` かつ `kind ∈ {Workspace,Provider,Cluster,App,Box}`
- 解析順: まず全ドキュメントを読み込み、次に検証/索引化を行う
  - 検証順序: Workspace → Provider → Cluster → App → Box
  - `ops.kompox.dev/path` のセグメント数・親存在確認・同名衝突をチェック
  - いずれかの検証に失敗した場合はエラーを返し、DB は更新しない
- 保存: 検証成功時のみ FQN をキーとして inmem/RDB に一括反映
- **ドキュメント追跡**:
  - `Document.Path`: ドキュメントの読み込み元ファイルパス
  - `Document.Index`: マルチドキュメント YAML 内での位置（1-based）
  - エラー報告: 検証エラーにファイルパスとドキュメント位置を含める
    - 例: `provider "ws1/prv1" validation error: parent "ws1" does not exist from /path/to/config.yaml (document 2)`

## 命名制約

- 各セグメント(ws/prv/cls/app/box)は DNS-1123 label に準拠し 1..63 文字
  - 正規表現: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
  - 小文字英数とハイフンのみ
- FQN 文字列の長さ制限は設けません(内部専用)。
- K8s ラベル値は 63 文字制限のため、`ops.kompox.dev/*-hash` を併用します。
- K8s リソース名(Namespace/Service 等)は 63 以内となるよう「切り詰め + 安定ハッシュ」を用いて生成します。

## CLI/UX 対応(要点)

- kompoxops app: kompoxapp.yml を基点に、既定 componentName=app の Box をデプロイ/破棄
- kompoxops box: -a/--app, -c/--component 等のフラグにより任意 Box をデプロイ/破棄
- PaaS ユーザー向け:
  - kompoxapp.yml: App 相当(ops.kompox.dev/v1alpha1, kind: App)
  - kompoxbox.yml: Box 相当(ops.kompox.dev/v1alpha1, kind: Box) — 任意。存在すれば app に上書き適用
- いずれも本仕様の `ops.kompox.dev/path` を付与した Portable YAML を取り込めます。

## 将来: オペレーター/クラスタ保管

- 本仕様の G/V/K と K8s 内部表現(ラベル/NS命名)により、Operator は `ops.kompox.dev/*` を直接 Watch 可能。
- クラスタ保管モードでは、同一 YAML を CRD に apply して GitOps ツールで管理することも選択肢です。

## References

- [K4x-ADR-007]
- [2025-10-13-crd.ja.md]
- [config/crd/ops/v1alpha1/README.md]

[K4x-ADR-007]: ../adr/K4x-ADR-007.md
[2025-10-13-crd.ja.md]: ../../_dev/tasks/2025-10-13-crd.ja.md
[config/crd/ops/v1alpha1/README.md]: ../../config/crd/ops/v1alpha1/README.md
