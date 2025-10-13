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

- 定義(正規 ID): すべてのリソースの正規 ID は FQN(path) とします。
  - WorkspaceID: `ws`
  - ProviderID: `ws/prv`
  - ClusterID: `ws/prv/cls`
  - AppID: `ws/prv/cls/app`
  - BoxID: `ws/prv/cls/app/box`
- リポジトリ契約(保存/取得): adapter/store(inmem, rdb) は次を満たします。
  - 主キー `ID` は FQN をそのまま保持する
  - 親参照の外部キーは親の FQN を保持する
  - Create は事前に設定された `ID` をそのまま保存する(空のときのみ採番)
  - Get は `ID`(=FQN) をキーに単一取得する
  - 重複 `ID` はエラーとする
- 将来方針: リネーム後も同一 ID を維持する要件が生じた場合は「UUID(v4) 主キー + FQN UNIQUE」へ移行を検討します。

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

### CLI 取り込み経路と優先度(kompoxops)

Kompox v1 CLI では、CRD スタイルの YAML を次のフラグ/環境変数で取り込み可能です。

- `--crd-path PATH`（複数指定可）
  - 指定されたファイル/ディレクトリから `.yml`/`.yaml` を再帰的に読み込みます（マルチドキュメント対応）。
  - 環境変数 `KOMPOX_CRD_PATH=path1,path2` でも指定可能です（カンマ区切り）。
  - フラグと環境変数の両方が与えられた場合、フラグが優先されます。
  - 指定したパスが存在しない場合はエラー（CLI は直ちに終了）。

- `--crd-app PATH`(デフォルト: `./kompoxapp.yml`)
  - PATH が存在する場合のみ読み込み対象に含めます。存在しない場合は無視(エラーにしない)。
  - 環境変数 `KOMPOX_CRD_APP` でも指定可能です。フラグが環境変数より優先されます。
  - 既定 AppID/ClusterID の推定: `--crd-app` で読み込んだファイル/ディレクトリ内に App(Kind: App) が「ちょうど 1 つ」存在する場合に限り、`--app-id` 未指定時のデフォルトとしてその App の FQN を採用します。さらに、その App が参照する Cluster が一意に決まる場合は `--cluster-id` の既定として当該 Cluster の FQN を採用します。

有効化条件と優先度:

- `--crd-path` と `--crd-app`（存在時）の合算セットに対して取り込み・検証が成功した場合、「CRD モード」が有効になります。
- CRD モードが有効な場合、`--db-url` と `KOMPOX_DB_URL` は無視されます（CRD が唯一のソース・オブ・トゥルース）。
- いずれの CRD 入力も存在せず取り込みが行われない場合は、従来通り `--db-url`（既定値: `file:kompoxops.yml`）の経路が利用されます。

検証と保存（再掲）:

- 全ドキュメントを収集後に整合検証を行い、エラーがあれば DB へは反映しない（オール・オア・ナッシング）。
- エラーには読み込み元のファイルパスとマルチドキュメント内のドキュメント番号（1-based）を含めます。

### CLI のリソース指定(kompoxops)

- FQN を第一級の識別子として扱います。CLI は各リソースを ID(FQN) で受け付けます。
- 追加フラグ:
  - `--app-id, -A` App の FQN を指定 `ws/prv/cls/app`
  - `--cluster-id, -C` Cluster の FQN を指定 `ws/prv/cls`
- 互換フラグ(非推奨表示):
  - `--app-name`, `--cluster-name` は単名/部分指定を受け付けますが、複数一致時は即エラーとします(曖昧解決しない)。
- 既定/自動設定:
  - 既定は ID フラグ(`--app-id`/`--cluster-id`) に対してのみ行います。name フラグの既定は行いません。
  - `--crd-app` 範囲で App がちょうど 1 件なら `--app-id` を FQN で既定化。参照 Cluster が一意なら `--cluster-id` も既定化します。

## 将来: オペレーター/クラスタ保管

- 本仕様の G/V/K と K8s 内部表現(ラベル/NS命名)により、Operator は `ops.kompox.dev/*` を直接 Watch 可能。
- クラスタ保管モードでは、同一 YAML を CRD に apply して GitOps ツールで管理することも選択肢です。

## References

- [K4x-ADR-007]
- [Kompox-CRD.ja.md]
- [2025-10-13-crd-p1.ja.md]
- [2025-10-13-crd-p2.ja.md]
- [config/crd/ops/v1alpha1/README.md]

[K4x-ADR-007]: ../adr/K4x-ADR-007.md
[Kompox-CLI.ja.md]: ./Kompox-CLI.ja.md
[2025-10-13-crd-p1.ja.md]: ../../_dev/tasks/2025-10-13-crd-p1.ja.md
[2025-10-13-crd-p2.ja.md]: ../../_dev/tasks/2025-10-13-crd-p2.ja.md
[config/crd/ops/v1alpha1/README.md]: ../../config/crd/ops/v1alpha1/README.md
