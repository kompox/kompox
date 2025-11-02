---
id: Kompox-KOM
title: Kompox KOM configuration
version: v1
status: synced
updated: 2025-11-03
language: ja
---

# Kompox KOM configuration

## 概要

本ドキュメントは、Kompox v1 CLI および将来の v2 Operator で利用するポータブルな設定ファイル形式「Kompox Ops Manifest (KOM)」を定義する。

KOM は Kubernetes CRD と互換な構造(apiVersion/kind/metadata/spec)を持つが、K8s の CRD そのものではない。すべてのリソース (Workspace/Provider/Cluster/App/Box) が階層的な型付きパス形式の Resource ID (例: `/ws/<ws>/prv/<prv>/cls/<cls>`) を持ち、ファイル間の参照整合性を検証してオール・オア・ナッシング方式でインポートする。

本ドキュメントは KOM の構文とスキーマ、Resource ID の形式、ローダーが要求する制約、将来の Operator 対応を記述する。

関連ADR: [K4x-ADR-009] [K4x-ADR-010] [K4x-ADR-011] [K4x-ADR-012]

## Kompox Ops Manifest Schema

KOM のフィールド:

| Key | Value | Required |
|-|-|-|
| `apiVersion` | `ops.kompox.dev/v1alpha1` | true |
| `kind` | リソースの種類 | true |
| `metadata.name` | リソースの名前 | true |
| `metadata.annotations["ops.kompox.dev/id"]` | Resource ID (別名: FQN) | true |
| `metadata.annotations["ops.kompox.dev/doc-path"]` | ファイルパス (ローダーが設定) | false |
| `metadata.annotations["ops.kompox.dev/doc-index"]` | 1-basedインデックス (ローダーが設定) | false |
| `spec` | Kind 固有の情報 | true |

Resource ID (別名: FQN) の仕様:

- 形式: スラッシュ区切りで短縮 kind と name を交互に並べた階層パス
- 文法: `/<sk>/<name>(/<sk>/<name>)*`
  - sk は短縮 kind (下表参照)
  - name は DNS-1123 label 準拠 (最短1文字、最長63文字、正規表現: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

kind と Resource ID の例:

| kind | 短縮 kind | 親 kind | Resource ID 例 |
|------|----------|---------------|----------------|
| Workspace | `ws` |  | `/ws/myWorkspace` |
| Provider | `prv` | Workspace | `/ws/myWorkspace/prv/azure1` |
| Cluster | `cls` | Provider | `/ws/myWorkspace/prv/azure1/cls/prod-cluster` |
| App | `app` | Cluster | `/ws/myWorkspace/prv/azure1/cls/prod-cluster/app/app1` |
| Box | `box` | App | `/ws/myWorkspace/prv/azure1/cls/prod-cluster/app/app1/box/web` |
| Ingress (将来) | `ing` | Cluster | `/ws/myWorkspace/prv/azure1/cls/prod-cluster/ing/traefik` |

バリデーション:

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

## Defaults (非永続リソース)

- `Defaults` は Kompox アプリファイルからの読み込みに限って解釈される、KOM ローダー専用の特殊な Kind である。
- Kompox アプリファイルは CLI で既定のエントリポイントとなる設定ファイル `kompoxapp.yml` を指す。
- [K4x-ADR-011] 参照。

### 仕様

スキーマ定義(型と構造):

```yaml
apiVersion: ops.kompox.dev/v1alpha1   # 定数
kind: Defaults                        # 定数
metadata:
  name: string?                       # 任意(省略可)
spec:
  komPath: string[]?                  # 追加で読み込む KOM のパス配列(任意)
  appId: string?                      # 既定 App の Resource ID/FQN (任意)
```

- 記述位置: Kompox アプリファイル(kompoxapp.yml)内のみ。出現は最大1件。
- 識別: Resource ID を持たない(FQN なし)。
- メタデータ: metadata.name は任意。省略可。
- 永続性: Kubernetes に適用しない。永続化しない。
- 設定項目:
  - spec.komPath: 追加で読み込む KOM のファイルまたはディレクトリの配列。
  - spec.appId: 既定の App の Resource ID。明示指定がない場合の既定値として利用する。

### 例

```yaml
apiVersion: ops.kompox.dev/v1alpha1
kind: Defaults
spec:
  komPath:
    - ./kom                      # A: 同一プロジェクト配下のディレクトリ
    - ../shared                  # B: 兄弟ディレクトリ配下の共有フォルダ
    - ../shared/workspace.yml    # C: 単一ファイル(Workspace)
    - $KOMPOX_DIR/shared/prv.yml # D: プロジェクトディレクトリ配下の絶対パス
    - ./kom-linked               # E: シンボリックリンク(解決後の実体で評価)
  appId: /ws/myWorkspace/prv/azure1/cls/prod-cluster/app/app1
```

### spec.komPath

- 型: string[]
- 目的: Kompox アプリファイルを起点に、追加の KOM 入力元を宣言する。
- 入力要件:
  - ローカルパスのみ許可(URL不可)。
  - 相対/絶対の両方を許可。相対パスは Kompox アプリファイルのディレクトリを基準に解決。
  - 親ディレクトリ参照 ../ を許可。
  - 解決手順: path.Clean と EvalSymlinks を行い実パスを得る。
  - 文字列展開: `$KOMPOX_DIR` と `$KOMPOX_CFG_DIR` をサポートする (CLI により解決される)。
  - セキュリティ境界: 解決後の実パスは `$KOMPOX_DIR` または `$KOMPOX_CFG_DIR` の配下でなければならない。
  - ディレクトリは再帰的に走査し、.yml/.yaml のみを対象とする。
  - 無視パス: .git/ .github/ node_modules/ vendor/ .direnv/ .venv/ dist/ build/
  - グロブ/ワイルドカードは不可。
  - 去重と循環防止: 正規化済み実パスで重複を除外する。
- エラー条件:
  - パスが存在しない。
  - 解決済み実パスが `$KOMPOX_DIR` または `$KOMPOX_CFG_DIR` の配下ではない。
  - 対象拡張子が .yml/.yaml 以外。

### spec.appId

- 型: string (Resource ID)
- 目的: 既定の App を指す Resource ID を宣言する。
- 要件:
  - Resource ID の構文に合致すること。
  - 読み込み済みの KOM に一致する App が存在すること。
  - 明示的な指定がある場合は既定より優先される。優先順位の詳細は [Kompox-CLI.ja.md] を参照。

## K8s 永続化仕様

KOM を Kubernetes リソースとして永続化する場合の CRD および Namespace への変換の暫定仕様:

- `metadata.labels` フィールド
  - `app.kubernetes.io/managed-by: kompox` を設定する
  - `ops.kompox.dev/<kind>-hash` に Resource ID から算出したハッシュを設定する
- `metadata.annotations` フィールド
  - `ops.kompox.dev/id` に Resource ID を保持する
- `status.opsNamespace` フィールド (コントローラーが自動設定)
  - 配下のリソースを格納する Namespace 名を設定する
  - Namespace 名は `k4x-<sk>-<hash>-<name>` の形式として先頭 63 文字で切り捨てる

変換例 (読み込み用の K8s manifest とは異なる):

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
  annotations:
    ops.kompox.dev/id: /ws/<wsName>
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
  annotations:
    ops.kompox.dev/id: /ws/<wsName>
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
  annotations:
    ops.kompox.dev/id: /ws/<wsName>/prv/<prvName>
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
  annotations:
    ops.kompox.dev/id: /ws/<wsName>/prv/<prvName>
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
  annotations:
    ops.kompox.dev/id: /ws/<wsName>/prv/<prvName>/cls/<clsName>
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
  annotations:
    ops.kompox.dev/id: /ws/<wsName>/prv/<prvName>/cls/<clsName>
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
  annotations:
    ops.kompox.dev/id: /ws/<wsName>/prv/<prvName>/cls/<clsName>/app/<appName>
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
  annotations:
    ops.kompox.dev/id: /ws/<wsName>/prv/<prvName>/cls/<clsName>/app/<appName>
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
  annotations:
    ops.kompox.dev/id: /ws/<wsName>/prv/<prvName>/cls/<clsName>/app/<appName>/box/<boxName>
spec: {}
```

すべてのハッシュは Resource ID の文字列から導出する。 `ShortHash` は internal/naming パッケージの短縮ハッシュ関数を指す。

```
wsHash  = ShortHash("/ws/<wsName>")
prvHash = ShortHash("/ws/<wsName>/prv/<prvName>")
clsHash = ShortHash("/ws/<wsName>/prv/<prvName>/cls/<clsName>")
appHash = ShortHash("/ws/<wsName>/prv/<prvName>/cls/<clsName>/app/<appName>")
boxHash = ShortHash("/ws/<wsName>/prv/<prvName>/cls/<clsName>/app/<appName>/box/<boxName>")
```

## 非 K8s 永続化仕様

KOM を Kubernetes ではないストア (RDB など) で永続化する場合の仕様:

- 定義(正規 ID): すべてのリソースの正規 ID は Resource ID とする。
  - WorkspaceID: `/ws/<wsName>`
  - ProviderID: `/ws/<wsName>/prv/<prvName>`
  - ClusterID: `/ws/<wsName>/prv/<prvName>/cls/<clsName>`
  - AppID: `/ws/<wsName>/prv/<prvName>/cls/<clsName>/app/<appName>`
  - BoxID: `/ws/<wsName>/prv/<prvName>/cls/<clsName>/app/<appName>/box/<boxName>`
- リポジトリ契約(保存/取得): adapter/store(inmem, rdb)は次を満たす。
  - 主キー `ID` は Resource ID をそのまま保持する
  - 親参照の外部キーは親リソースの Resource ID を保持する
  - Create は `metadata.annotations["ops.kompox.dev/id"]` に与えられた ID をそのまま保存する(空や不一致はバリデーションエラー)
  - Get は `ID`(= Resource ID)をキーに単一取得する
  - 重複 `ID` は一意制約違反としてエラー
  - 参照整合: 子の ID は親チェーンに整合しなければならない(例: Box の ID は `.../app/<appName>/box/<boxName>` を終端に持つ)
- 将来方針: リネーム後も同一 ID を維持する要件が生じた場合は「UUID(v4) 主キー + Resource ID UNIQUE」を検討する。
- 補足: K8s へ出力するラベルの `*-hash` は Resource ID を入力に短縮ハッシュを算出する(ラベル長制約対策)。

## ローダー(インポート)仕様

- 前処理:
  - CLI フラグ・環境変数・Kompox アプリファイル (kompoxapp.yml) を読み込む
  - Defaults リソースを読み込み検証する
  - 入力のファイル・ディレクトリパスリストを決定する
- 入力処理:
  - ローカルファイルシステムのみを対象とする
  - ファイル・ディレクトリのパスを受け取り、拡張子 .yml/.yaml のファイルを再帰的に走査する
  - ワイルドカードのパスは対応しない
  - 各ファイルから 1 つ以上の YAML ドキュメントを読み込む (マルチドキュメント対応)
  - App リソースの処理
    - `RefBase`: ドキュメントの読み込み元ディレクトリを `file:///path/to/dir/` 形式に正規化して設定する
    - `Compose`: ローダーでは内容を解釈せず、文字列として保持する
    - ローカルファイルシステム参照ポリシーの検証は変換・実行フェーズで行う ([K4x-ADR-012])
  - ドキュメントごとに次の情報を追加する
    - `metadata.annotations["ops.kompox.dev/doc-path"]` ドキュメントの読み込み元ファイルパス
    - `metadata.annotations["ops.kompox.dev/doc-index"]` マルチドキュメント YAML 内での位置(1-based)
  - 検証エラー報告などでは `doc-path` と `doc-index` の情報を付与する
    - 例: `provider "/ws/ws1/prv/prv1" validation error: parent "/ws/ws1" does not exist from /path/to/config.yaml (document 2)`
- 検証:
  - 全ドキュメントを読み込む
  - 階層の浅い順にソートする (Workspace → Provider → Cluster → App → Box)
  - 全ドキュメントについて検証:
    - `apiVersion` が `ops.kompox.dev/v1alpha1` であること
    - `kind` が有効なリソース種別であること
    - `ops.kompox.dev/id` (Resource ID) の形式が正しく一意であること
    - `kind`・`ops.kompox.dev/id`・`metadata.name` に矛盾がないこと
  - 既存のリソースを含む検証:
    - 一意制約: 同一 Resource ID の重複がないこと
    - 参照整合性: すべての親リソースが存在すること
  - Defaults 検証:
    - spec.appId が指定する App リソースが存在すること
- 保存・変換:
  - K8s 永続化の場合は K8s Manifest に変換して適用する
  - 非 K8s 永続化の場合は Resource ID をキーとして一括保存する

## References

- [K4x-ADR-009]
- [K4x-ADR-010]
- [K4x-ADR-011]
- [K4x-ADR-012]
- [Kompox-CLI.ja.md]
- [2025-10-15-kom.ja.md]
- [2025-10-17-defaults.ja.md]
- [2025-10-18-refbase.ja.md]
- [config/crd/ops/v1alpha1/README.md]

[K4x-ADR-009]: ../adr/K4x-ADR-009.md
[K4x-ADR-010]: ../adr/K4x-ADR-010.md
[K4x-ADR-011]: ../adr/K4x-ADR-011.md
[K4x-ADR-012]: ../adr/K4x-ADR-012.md
[Kompox-CLI.ja.md]: ./Kompox-CLI.ja.md
[2025-10-15-kom.ja.md]: ../../_dev/tasks/2025-10-15-kom.ja.md
[2025-10-17-defaults.ja.md]: ../../_dev/tasks/2025-10-17-defaults.ja.md
[2025-10-18-refbase.ja.md]: ../../_dev/tasks/2025-10-18-refbase.ja.md
[config/crd/ops/v1alpha1/README.md]: ../../config/crd/ops/v1alpha1/README.md
