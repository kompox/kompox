---
id: Kompox-KOM
title: Kompox KOM configuration
version: v1
status: synced
updated: 2025-10-15
language: ja
---

# Kompox KOM configuration

## 概要

本ドキュメントは、Kompox v1 CLI および将来の v2 Operator で利用するポータブルな設定ファイル形式「Kompox Ops Manifest (KOM)」を定義します。KOM は Kubernetes CRD と互換な構造(apiVersion/kind/metadata/spec)を持ちますが、K8s の CRD そのものではありません。

Kompox では、リソース定義を記述したポータブルな YAML ドキュメントを **Kompox Ops Manifest (KOM)** と呼びます。KOM は以下の特徴を持ちます:

- CRD 互換の構造 (apiVersion/kind/metadata/spec)
- すべてのリソース (Workspace/Provider/Cluster/App/Box) が Resource ID を持つ
- Resource ID は階層的な型付きパス形式 (例: `/ws/<ws>/prv/<prv>/cls/<cls>`)
- ファイル間の参照整合性を検証し、オール・オア・ナッシング方式でインポート

本ドキュメントでは、KOM の構文、Resource ID の形式、ローダーの動作、CLI との連携方法、および将来の Operator 対応について説明します。

## Kompox Ops Manifest Schema

Kompox KOM のインポート・エクスポートに使用できるポータブルな YAML ドキュメントを Kompox Ops Manifest (KOM) と呼びます。

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

- 入力: ファイル/ディレクトリを受け取り、拡張子 .yml/.yaml を再帰的に走査する。マルチドキュメントのYAMLファイルに対応する。入力はローカルのみを対象とし、ワイルドカードは非対応。
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

追加検証(Defaults 関連):

- `--kom-app` で指定したファイルに直接含まれない App がローカルファイル参照(`file:` や相対 hostPath など)を含む場合はエラーとします。

## 命名制約

- 各セグメント(ws/prv/cls/app/box)は DNS-1123 label に準拠し 1..63 文字
  - 正規表現: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
  - 小文字英数とハイフンのみ
- Resource ID 文字列の長さ制限は設けません(内部専用)。
- K8s ラベル値は 63 文字制限のため、`ops.kompox.dev/*-hash` を併用します。
  - `*-hash` の算出入力は Resource ID とします(例: `ops.kompox.dev/workspace-hash = ShortHash("/ws/<ws>")`)。
- K8s リソース名(Namespace/Service 等)は 63 以内となるよう「切り詰め + 安定ハッシュ」を用いて生成します。

## CLI 対応(kompoxops)

### 概要

Kompox v1 CLI (`kompoxops`) は、本仕様の KOM (Kompox Ops Manifest) を直接読み込んで動作します。

主なコマンド:

- `kompoxops app`: `kompoxapp.yml` を基点に、既定 componentName=app の Box をデプロイ/破棄
- `kompoxops box`: `-a/--app`, `-c/--component` 等のフラグにより任意の Box をデプロイ/破棄

PaaS ユーザー向けの簡易ファイル:

- `kompoxapp.yml`: App 相当(`ops.kompox.dev/v1alpha1`, `kind: App`)
- `kompoxbox.yml`: Box 相当(任意。存在すれば app に上書き適用)

いずれも KOM スキーマ(`ops.kompox.dev/id`)に準拠したマニフェストとして取り込めます。

### KOM モード

CLI は、KOM を読み込む「KOM モード」と、従来の `--db-url` による永続化モードの二通りの動作モードを持ちます。

KOM モード有効化条件:

- `--kom-path` または `--kom-app`(存在時)で指定されたファイルが1件以上読み込まれ、検証に成功した場合
- KOM モードが有効な場合、`--db-url` および `KOMPOX_DB_URL` は無視されます(KOM が唯一のソース・オブ・トゥルース)

いずれの KOM 入力も存在しない場合は、従来通り `--db-url`(既定値: `file:kompoxops.yml`)を利用します。

### Defaults 疑似リソース

`kompoxapp.yml` に、KOM ロード対象とデフォルト選択を宣言する疑似リソース `Defaults` を記述できます。`Defaults` はローダー/CLI 専用であり、保存の対象にはなりません。

例:

```yaml
apiVersion: ops.kompox.dev/v1alpha1
kind: Defaults
spec:
  komPath:
    - file.yml                    # 同一ディレクトリの相対ファイル
    - ./path/to/dir               # ローカル相対ディレクトリ(再帰、YAMLのみ)
    - /path/to/absolute.yml       # ローカル絶対ファイル
  appId: /ws/<ws>/prv/<prv>/cls/<cls>/app/<app>
```

規則:

- `komPath`: ローカルファイルシステムのみ。ワイルドカード不可(`* ? [` は使用不可)。ディレクトリは再帰的に走査(YAML のみ)。相対パスは `kompoxapp.yml` の配置ディレクトリ基準。
- `appId`: デフォルト App の Resource ID を指定(後述の優先順位に従う)。
- 文書の出現順序は重要ではありません。全ドキュメントを集約後にトポロジカルソートと検証を行います。

ローカルファイルシステム参照の制約:

- `kompoxapp.yml` に直接含まれる App のみ `file:compose.yml` や `./data:/data` 等のローカル参照を許可。
- `komPath` で読み込まれた App はローカル参照を禁止(検証エラー)。

### フラグと優先順位

#### KOM 読み込み経路

KOM 読み込みパスの優先順位:

- `--kom-path PATH`(複数指定可)
  - 指定されたファイル/ディレクトリから `.yml`/`.yaml` を再帰的に読み込み(マルチドキュメント対応)。
  - 環境変数 `KOMPOX_KOM_PATH=path1,path2` でも指定可能(カンマ区切り)。
  - 指定したパスが存在しない場合はエラー(CLI は直ちに終了)。
- `--kom-app PATH`(デフォルト: `./kompoxapp.yml`)
  - PATH が存在する場合のみ読み込み対象に含める。存在しない場合は無視(エラーにしない)。
  - 環境変数 `KOMPOX_KOM_APP` でも指定可能。

読み込みパス優先順位: `--kom-path` > `KOMPOX_KOM_PATH` > `Defaults.spec.komPath` > なし

フラグと環境変数の両方が与えられた場合、フラグが優先されます。

#### App/Cluster の自動選定

既定 App の優先順位:

1. `--app-id` フラグ
2. `Defaults.spec.appId`
3. `--kom-app` で指定したファイルに直接含まれる App がちょうど1件ならその Resource ID
4. 上記いずれも該当しない場合は指定必須

App が決まった場合、その App が参照する Cluster が一意に決まるなら `--cluster-id` の既定として採用します。

### リソース指定フラグ

Resource ID による指定(推奨):

- `--app-id, -A`: App の Resource ID を指定 `/ws/<ws>/prv/<prv>/cls/<cls>/app/<app>`
- `--cluster-id, -C`: Cluster の Resource ID を指定 `/ws/<ws>/prv/<prv>/cls/<cls>`

互換フラグ(非推奨表示):

- `--app-name`, `--cluster-name`: 単名/部分指定を受け付けますが、複数一致時は即エラー(曖昧解決しない)。

既定/自動設定:

- 既定は ID フラグ(`--app-id`/`--cluster-id`)に対してのみ行います。name フラグの既定は行いません。

### 検証とエラー報告

- 全ドキュメントを収集後に整合検証を行い、エラーがあれば DB へは反映しない(オール・オア・ナッシング)。
- エラーには読み込み元のファイルパスとマルチドキュメント内のドキュメント番号(1-based)を含めます。
  - 例: `provider "/ws/ws1/prv/prv1" validation error: parent "/ws/ws1" does not exist from /path/to/config.yaml (document 2)`

## 将来: オペレーター/クラスタ保管

- 本仕様の G/V/K と K8s 内部表現(ラベル/NS命名)により、Operator は `ops.kompox.dev/*` を直接 Watch 可能。
- クラスタ保管モードでは、同一 YAML を CRD に apply して GitOps ツールで管理することも選択肢です。

## References

- [K4x-ADR-007]
- [K4x-ADR-009]
- [K4x-ADR-010]
- [Kompox-CLI.ja.md]
- [2025-10-13-crd-p1.ja.md]
- [2025-10-13-crd-p2.ja.md]
- [config/crd/ops/v1alpha1/README.md]

[K4x-ADR-007]: ../adr/K4x-ADR-007.md
[K4x-ADR-009]: ../adr/K4x-ADR-009.md
[K4x-ADR-010]: ../adr/K4x-ADR-010.md
[Kompox-CLI.ja.md]: ./Kompox-CLI.ja.md
[2025-10-13-crd-p1.ja.md]: ../../_dev/tasks/2025-10-13-crd-p1.ja.md
[2025-10-13-crd-p2.ja.md]: ../../_dev/tasks/2025-10-13-crd-p2.ja.md
[config/crd/ops/v1alpha1/README.md]: ../../config/crd/ops/v1alpha1/README.md
