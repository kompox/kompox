---
id: Kompox-CRD
title: Kompox CRD-style configuration
version: v1
status: synced
updated: 2025-10-15
language: ja
---

# Kompox CRD-style configuration

## 概要

本ドキュメントは、Kompox v1 CLI および将来の v2 Operator で利用する Kubernetes CRD 互換の設定ファイル形式を定義します。

Kompox では、リソース定義を記述したポータブルな YAML ドキュメントを **Kompox Ops Manifest (KOM)** と呼びます。KOM は以下の特徴を持ちます:

- Kubernetes CRD 形式 (apiVersion/kind/metadata/spec) に準拠
- すべてのリソース (Workspace/Provider/Cluster/App/Box) が Resource ID を持つ
- Resource ID は階層的な型付きパス形式 (例: `/ws/<ws>/prv/<prv>/cls/<cls>`)
- ファイル間の参照整合性を検証し、オール・オア・ナッシング方式でインポート

本ドキュメントでは、KOM の構文、Resource ID の形式、ローダーの動作、CLI との連携方法、および将来の Operator 対応について説明します。

## Kompox Ops Manifest Schema

Kompox CRD のインポート・エクスポートに使用できるポータブルな YAML ドキュメントを Kompox Ops Manifest (KOM) と呼びます。

KOM の場所と名前は Resource ID で一意に識別されます。 Resource ID は Fully Qualified Name (FQN) とも呼ばれますが、本ドキュメントでは Resource ID を使用します。

KOM のフィールド:

| Key | Value | Required |
|-|-|-|
| `apiVersion` | `ops.kompox.dev/v1alpha1` | true |
| `kind` | リソースの種類 | true |
| `metadata.name` | リソースの名前 | true |
| `metadata.annotations["ops.kompox.dev/id"]` | Resource ID (FQN) | true |
| `metadata.annotations["ops.kompox.dev/doc-path"]` | YAMLファイルパス | false |
| `metadata.annotations["ops.kompox.dev/doc-index"]` | YAMLドキュメントインデックス (1-based) | false |
| `spec` | Kind 固有の情報 | true |

Resource ID の仕様:

- 形式: スラッシュ区切りで短縮 kind と name を交互に並べた階層パス
- 文法: `/<sk>/<name>(/<sk>/<name>)*`
  - sk は短縮 kind (下表参照)
  - name は DNS-1123 label(小英数と `-`, 1..63 文字)

kind と Resource ID の例:

| kind | 短縮 kind | 親 kind | Resource ID 例 |
|------|----------|---------------|----------------|
| Workspace | `ws` |  | `/ws/myWorkspace` |
| Provider | `prv` | Workspace | `/ws/myWorkspace/prv/azure1` |
| Cluster | `cls` | Provider | `/ws/myWorkspace/prv/azure1/cls/prod-cluster` |
| App | `app` | Cluster | `/ws/myWorkspace/prv/azure1/cls/prod-cluster/app/app1` |
| Box | `box` | App | `/ws/myWorkspace/prv/azure1/cls/prod-cluster/app/app1/box/web` |
| Ingress (将来) | `ing` | Cluster | `/ws/myWorkspace/prv/azure1/cls/prod-cluster/ing/traefik` |

KOM のバリデーション:

- 必須のフィールドに有効な値が存在すること
- Resource ID が正しい形式であること(先頭 `/`、短縮 kind/name の交互、`/<sk>/<name>(/<sk>/<name>)*`)
- kind と Resource ID 末尾の短縮 kind が対応すること(例: `.../box/<name>` ⇔ `kind: Box`)
- metadata.name と Resource ID の末尾の name が一致すること
- Resource ID に含まれる各セグメントについて、
  - name が DNS-1123 label (63文字以内) であること
  - sk が認識された短縮 kind であること
  - kind の親子関係が正しいこと

KOM YAML マルチドキュメントの例:

```yaml
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: <wsName>
  annotations:
    ops.kompox.dev/id: /ws/<wsName>
spec: {}
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: <prvName>
  annotations:
    ops.kompox.dev/id: /ws/<wsName>/prv/<prvName>
spec: {}
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  name: <clsName>
  annotations:
    ops.kompox.dev/id: /ws/<wsName>/prv/<prvName>/cls/<clsName>
spec: {}
---
apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  name: <appName>
  annotations:
    ops.kompox.dev/id: /ws/<wsName>/prv/<prvName>/cls/<clsName>/app/<appName>
spec: {}
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Box
metadata:
  name: <boxName>
  annotations:
    ops.kompox.dev/id: /ws/<wsName>/prv/<prvName>/cls/<clsName>/app/<appName>/box/<boxName>
spec: {}
```

## K8s CRD/NS internal representation

このセクションの仕様は暫定的なものであり、将来の v2 Operator の要件に応じて変更される可能性があります。

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
# すべてのハッシュは Resource ID(ops.kompox.dev/id)の文字列から導出する
wsHash  = ShortHash("/ws/<wsName>")
prvHash = ShortHash("/ws/<wsName>/prv/<prvName>")
clsHash = ShortHash("/ws/<wsName>/prv/<prvName>/cls/<clsName>")
appHash = ShortHash("/ws/<wsName>/prv/<prvName>/cls/<clsName>/app/<appName>")
boxHash = ShortHash("/ws/<wsName>/prv/<prvName>/cls/<clsName>/app/<appName>/box/<boxName>")
```

## Non-K8s リソース ID・永続化ストア

- 定義(正規 ID): すべてのリソースの正規 ID は Resource ID とします。
  - WorkspaceID: `/ws/<wsName>`
  - ProviderID: `/ws/<wsName>/prv/<prvName>`
  - ClusterID: `/ws/<wsName>/prv/<prvName>/cls/<clsName>`
  - AppID: `/ws/<wsName>/prv/<prvName>/cls/<clsName>/app/<appName>`
  - BoxID: `/ws/<wsName>/prv/<prvName>/cls/<clsName>/app/<appName>/box/<boxName>`
- リポジトリ契約(保存/取得): adapter/store(inmem, rdb)は次を満たします。
  - 主キー `ID` は Resource ID をそのまま保持する
  - 親参照の外部キーは親リソースの Resource ID を保持する
  - Create は `metadata.annotations["ops.kompox.dev/id"]` に与えられた ID をそのまま保存する(空や不一致はバリデーションエラー)
  - Get は `ID`(= Resource ID)をキーに単一取得する
  - 重複 `ID` は一意制約違反としてエラー
  - 参照整合: 子の ID は親チェーンに整合しなければならない(例: Box の ID は `.../app/<appName>/box/<boxName>` を終端に持つ)
- 将来方針: リネーム後も同一 ID を維持する要件が生じた場合は「UUID(v4) 主キー + Resource ID UNIQUE」を検討します。
- 補足: K8s へ出力するラベルの `*-hash` は Resource ID を入力に短縮ハッシュを算出します(ラベル長制約対策)。

## ローダー(インポート)仕様

- 入力: ファイル/ディレクトリを受け取り、拡張子 .yml/.yaml を再帰的に走査する。マルチドキュメントのYAMLファイルに対応する。
- 受理条件: `apiVersion=ops.kompox.dev/v1alpha1` かつ `kind ∈ {Workspace,Provider,Cluster,App,Box}`
- 解析順: まず全ドキュメントを読み込み、次に検証/索引化を行う
  - 検証順序: Workspace → Provider → Cluster → App → Box
  - `ops.kompox.dev/id` の構文(kind/name 整合・先頭 `/`・短縮 kind)、親存在確認、同名衝突をチェック
  - いずれかの検証に失敗した場合はエラーを返し、DB は更新しない
- 保存: 検証成功時のみ Resource ID をキーとして inmem/RDB に一括反映
- **ドキュメント追跡**:
  - `Document.Path`: ドキュメントの読み込み元ファイルパス
  - `Document.Index`: マルチドキュメント YAML 内での位置(1-based)
  - エラー報告: 検証エラーにファイルパスとドキュメント位置を含める
    - 例: `provider "/ws/ws1/prv/prv1" validation error: parent "/ws/ws1" does not exist from /path/to/config.yaml (document 2)`

## 命名制約

- 各セグメント(ws/prv/cls/app/box)は DNS-1123 label に準拠し 1..63 文字
  - 正規表現: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
  - 小文字英数とハイフンのみ
- Resource ID 文字列の長さ制限は設けません(内部専用)。
- K8s ラベル値は 63 文字制限のため、`ops.kompox.dev/*-hash` を併用します。
  - `*-hash` の算出入力は Resource ID とします(例: `ops.kompox.dev/workspace-hash = ShortHash("/ws/<ws>")`)。
- K8s リソース名(Namespace/Service 等)は 63 以内となるよう「切り詰め + 安定ハッシュ」を用いて生成します。

## CLI/UX 対応(要点)

- kompoxops app: kompoxapp.yml を基点に、既定 componentName=app の Box をデプロイ/破棄
- kompoxops box: -a/--app, -c/--component 等のフラグにより任意 Box をデプロイ/破棄
- PaaS ユーザー向け:
  - kompoxapp.yml: App 相当(ops.kompox.dev/v1alpha1, kind: App)
  - kompoxbox.yml: Box 相当(ops.kompox.dev/v1alpha1, kind: Box) — 任意。存在すれば app に上書き適用
- いずれも本仕様の KOM スキーマ(`ops.kompox.dev/id` を付与)に準拠したマニフェストを取り込めます。

### CLI 取り込み経路と優先度(kompoxops)

Kompox v1 CLI では、CRD スタイルの YAML を次のフラグ/環境変数で取り込み可能です。

- `--crd-path PATH`(複数指定可)
  - 指定されたファイル/ディレクトリから `.yml`/`.yaml` を再帰的に読み込みます(マルチドキュメント対応)。
  - 環境変数 `KOMPOX_CRD_PATH=path1,path2` でも指定可能です(カンマ区切り)。
  - フラグと環境変数の両方が与えられた場合、フラグが優先されます。
  - 指定したパスが存在しない場合はエラー(CLI は直ちに終了)。

- `--crd-app PATH`(デフォルト: `./kompoxapp.yml`)
  - PATH が存在する場合のみ読み込み対象に含めます。存在しない場合は無視(エラーにしない)。
  - 環境変数 `KOMPOX_CRD_APP` でも指定可能です。フラグが環境変数より優先されます。
  - 既定 AppID/ClusterID の推定: `--crd-app` で読み込んだファイル/ディレクトリ内に App(Kind: App) が「ちょうど 1 つ」存在する場合に限り、`--app-id` 未指定時のデフォルトとしてその App の Resource ID を採用します。さらに、その App が参照する Cluster が一意に決まる場合は `--cluster-id` の既定として当該 Cluster の Resource ID を採用します。

有効化条件と優先度:

- `--crd-path` と `--crd-app`(存在時)の合算セットに対して取り込み・検証が成功した場合、「CRD モード」が有効になります。
- CRD モードが有効な場合、`--db-url` と `KOMPOX_DB_URL` は無視されます(CRD が唯一のソース・オブ・トゥルース)。
- いずれの CRD 入力も存在せず取り込みが行われない場合は、従来通り `--db-url`(既定値: `file:kompoxops.yml`)の経路が利用されます。

検証と保存(再掲):

- 全ドキュメントを収集後に整合検証を行い、エラーがあれば DB へは反映しない(オール・オア・ナッシング)。
- エラーには読み込み元のファイルパスとマルチドキュメント内のドキュメント番号(1-based)を含めます。

### CLI のリソース指定(kompoxops)

- Resource ID を第一級の識別子として扱います。CLI は各リソースを Resource ID で受け付けます。
- 追加フラグ:
  - `--app-id, -A` App の Resource ID を指定 `/ws/<ws>/prv/<prv>/cls/<cls>/app/<app>`
  - `--cluster-id, -C` Cluster の Resource ID を指定 `/ws/<ws>/prv/<prv>/cls/<cls>`
- 互換フラグ(非推奨表示):
  - `--app-name`, `--cluster-name` は単名/部分指定を受け付けますが、複数一致時は即エラーとします(曖昧解決しない)。
- 既定/自動設定:
  - 既定は ID フラグ(`--app-id`/`--cluster-id`) に対してのみ行います。name フラグの既定は行いません。
  - `--crd-app` 範囲で App がちょうど 1 件なら `--app-id` を Resource ID で既定化。参照 Cluster が一意なら `--cluster-id` も既定化します。

## 将来: オペレーター/クラスタ保管

- 本仕様の G/V/K と K8s 内部表現(ラベル/NS命名)により、Operator は `ops.kompox.dev/*` を直接 Watch 可能。
- クラスタ保管モードでは、同一 YAML を CRD に apply して GitOps ツールで管理することも選択肢です。

## References

- [K4x-ADR-007]
- [K4x-ADR-009]
- [Kompox-CLI.ja.md]
- [2025-10-13-crd-p1.ja.md]
- [2025-10-13-crd-p2.ja.md]
- [config/crd/ops/v1alpha1/README.md]

[K4x-ADR-007]: ../adr/K4x-ADR-007.md
[K4x-ADR-009]: ../adr/K4x-ADR-009.md
[Kompox-CLI.ja.md]: ./Kompox-CLI.ja.md
[2025-10-13-crd-p1.ja.md]: ../../_dev/tasks/2025-10-13-crd-p1.ja.md
[2025-10-13-crd-p2.ja.md]: ../../_dev/tasks/2025-10-13-crd-p2.ja.md
[config/crd/ops/v1alpha1/README.md]: ../../config/crd/ops/v1alpha1/README.md
