---
id: Kompox-KubeConverter
title: Kompox Kube Converter ガイド
version: v1
status: synced
updated: 2025-10-12
language: ja
---

# Kompox Kube Converter ガイド v1

## 概要

本書は `adapters/kube` が提供するコンバータ `kube.Converter` の設計と公開契約を解説します。Docker Compose を入力として、Kubernetes マニフェストへ変換する方針とルールを示します。

- 本書で扱う主な事項:
  - Workspace/Provider/Cluster/App 定義からのマニフェスト生成規則

## 方針

### リソース

次の Kubernetes リソースが作られる。

- アプリごとに以下を作成
  - Namespace 1個
  - NetworkPolicy
  - ServiceAccount
  - Role
  - RoleBinding
  - PV 複数個 (Provider のライフサイクルで管理される静的なクラウドディスクリソースを参照する RWO ボリューム)
  - PVC 複数個 (PVを参照する)
- コンポーネント (`app` `box` など) ごとに以下を生成
  - Deployment 1個 (シングルレプリカ、strategy.type=Recreate)
  - Service 複数個
    - ingress: 1個だけ生成。compose の host ポートを列挙して Ingress より参照される
    - headless: compose の service の数だけ作成、ローカル DNS 解決用
  - Secret 複数個 (任意)
    - Pod ごとのレジストリアクセス (pull)
    - container(service) ごとの環境変数設定 (base, override)
  - Ingress 0〜2個 (DNSホスト名から Service(ingress) へのルーティング)
    - デフォルトドメイン用 Ingress
    - カスタムドメイン用 Ingress
    - 生成条件は後述

上記リソースのすべてを Converter が出力するわけではない。一部はデプロイランタイム (CLI など) が生成・patch する。

### デプロイランタイムと Field Manager

Kompox では Kubernetes Server-Side Apply の Field Manager を用いてフィールド所有権を分離し、ユーザー操作との競合とドリフトを抑制する。

|Field Manager|用途|
|-|-|
|`kompox-converter`|Converter や CLI が生成する静的なマニフェストに含まれるリソース・フィールド|
|`kompox-runtime`|Secret 内容変化や CLI 操作結果を反映して patch されるリソース・フィールド|
|`user`|ユーザーの限定的カスタマイズ|

### 名前・ラベル・アノテーション

変換時に次のようなコンポーネント名 `<componentName>` を指定する。

- `app` (アプリ: app.compose により記述される)
- `box` (Kompox Box: kompoxops box deploy でデプロイする)

リソース命名規則

- Namespace: `k4x-<spHASH>-<appName>-<idHASH>`
  - バックエンドのクラウドリソースの名前としても使用する (Azureリソースグループ名など)
  - `k4x-<spHASH>`は名前順ソート時に関連リソースを一箇所に集めることで誤操作を防止するために配置する
  - `k4x`はKompoxの略
- PV/PVC: `k4x-<spHASH>-<volName>-<idHASH>-<volHASH>`
  - バックエンドのクラウドリソースの名前としても使用する (ディスク・スナップショットリソース名など)
  - Namespaceと同じ理由で`k4x-<spHASH>`を含む
  - PVCはPVと同名とする
- NetworkPolicy/ServiceAccount/Role/RoleBinding: `<appName>`
  - Namespace内のリソースで一意性が担保されているためハッシュを含まない
- Deployment/Service(ingress): `<appName>-<componentName>`
  - Namespace内のリソースで一意性が担保されているためハッシュを含まない
- Service(headless): `<containerName>`
  - app.compose.services により作られるコンテナの名前を使用する
  - Service(ingress) 名前衝突回避: `<appName>-app` または `<appName>-box` で始まる名前はエラーとする
  - Namespace内では単一のappしかデプロイできないのでapp.compose.servicesによる名前衝突はない
- ConfigMap/Secret: 命名は「ConfigMap/Secret リソース」節の命名表を参照
- Ingress:
  - デフォルトドメイン用: `<appName>-<componentName>-default`
  - カスタムドメイン用: `<appName>-<componentName>-custom`
  - Namespace内のリソースで一意性が担保されているためハッシュを含まない

各リソースには次のラベルを設定する。

|ラベル `NAME: VALUE`|対象 Kind|説明|
|-|-|-|
|`app: <appName>-<componentName>`|Deployment/Service/Ingress/Pod|セレクタラベル|
|`app.kubernetes.io/name: <appName>`|ALL|表示名|
|`app.kubernetes.io/instance: <appName>-<inHASH>`|ALL|インスタンス名|
|`app.kubernetes.io/component: <componentName>`|Deployment/Service/Ingress/Pod|コンポーネント名|
|`app.kubernetes.io/managed-by: kompox`|ALL||
|`kompox.dev/app-instance-hash: <inHASH>`|ALL|クラスタ依存インスタンスハッシュ|
|`kompox.dev/app-id-hash: <idHASH>`|ALL|クラスタ非依存アプリ識別ハッシュ|
|`kompox.dev/compose-service-headless: true`|Service(headless)||

Deployment および Service の Pod セレクタでは次のラベルを照合する。

```yaml
app: <appName>-<componentName>
```

Namespace には次のアノテーションを設定する。

```yaml
metadata:
  annotations:
    kompox.dev/app: <workspaceName>/<providerName>/<clusterName>/<appName>
    kompox.dev/provider-driver: <providerDriverName>
```

- `<providerDriverName>` は `aks` や `k3s` などのプロバイダドライバ名。

PV には次のアノテーションを設定する。

```yaml
metadata:
  annotations:
    kompox.dev/volume-handle-previous: <diskResourceId>
```

- `kompox.dev/volume-handle-previous` は初回のデプロイ時には設定しない。
- `<diskResourceId>` は `aks` の場合は Azure Disk リソース ID となる (サブスクリプション GUID 露出に注意: 閲覧権限を最小化)。

ConfigMap/Secret には次のアノテーションを設定する。

```yaml
metadata:
  annotations:
    kompox.dev/compose-content-hash: <hash>
```

Deployment の pod template には次のアノテーションを設定するが、これは Converter では出力しない。
デプロイランタイムがデプロイ完了後にすべての ConfigMap/Secret リソースをスキャンして Deployment リソースに patch する。
このときの Field Manager は `kompox-runtime` を用いる。

```yaml
metadata:
  annotations:
    kompox.dev/compose-content-hash: <podContentHASH>
```

### ハッシュの種類と生成規則

それぞれの `BASE` に対して次の `HASH` を適用する。

```
HASH = BASEのSHA256バイト列を256bitのLSB first bigintとして扱い36進数表記した冒頭6文字
```

`<spHASH>` (ワークスペース・プロバイダハッシュ)

```
BASE = workspace.name + ":" + provider.name
```

`<inHASH>` (クラスタ依存アプリハッシュ)

```
BASE = workspace.name + ":" + provider.name + ":" + cluster.name + ":" + app.name
```

`<idHASH>` (クラスタ非依存アプリハッシュ)

```
BASE = workspace.name + ":" + provider.name + ":" + app.name
```

`<volHASH>` (クラウドディスクリソースハッシュ)

```
BASE = クラウドディスクリソースのID (/subscriptions/.... など)
```

`<contentHASH>` (ConfigMap/Secret の内容を示すハッシュ)

```
BASE = リソース種別ごとに以下のとおり。
  - Secret: すべての `KEY=VALUE` について `KEY` を辞書順にソートして `KEY=VALUE<NUL>` を連結したバイト列
  - ConfigMap: すべての `KEY=VALUE` について `KEY` を辞書順にソートして `KEY=VALUE<NUL>` を連結したバイト列
```

`<podContentHASH>` (Pod が参照する ConfigMap/Secret リソースのハッシュ)

```
BASE = Pod template が参照するすべての ConfigMap/Secret リソースの `kompox.dev/compose-content-hash` アノテーションの文字列(存在しない場合は空文字列)を、imagePullSecrets列挙順、コンテナの名前の辞書順・コンテナ内の列挙順に連結したバイト列
```

各ハッシュの衝突が理論上発生した場合は実装側でハッシュ長 (6→8→10 文字…) を自動延長する。

### ボリューム

ボリューム関連名称の制約 [K4x-ADR-003]

- Volume 名: DNS-1123 ラベル、長さ 1..16
  - 正規表現: `^[a-z0-9]([-a-z0-9]{0,14}[a-z0-9])?$`
- Disk 名: DNS-1123 ラベル、長さ 1..24
- Snapshot 名: DNS-1123 ラベル、長さ 1..24
- 注: プロバイダドライバは基盤プラットフォームの制約に合わせて、上記より厳しい制限を追加で行うことがある。

app.volumes スキーマ

```yaml
app.volumes:
  - name: <name>
    size: <size>
    type: <type>  # optional: "disk" (default) or "files"
    options:
      <key>: <value>
```

- name: DNS-1123 ラベル、長さ 1..16、正規表現: `^[a-z0-9]([-a-z0-9]{0,14}[a-z0-9])?$`
- size: `32Gi` など
- type: ボリュームタイプ
  - `"disk"` (既定値、空でも同義): ブロックデバイス型ストレージ (通常 RWO アクセス)
  - `"files"`: ネットワークファイル共有 (RWX アクセス; プロバイダ管理の共有ストレージ)
  - 不明な値はバリデーションエラー
- options: Provider Driver が解釈するボリュームオプション。key/value ともに文字列。

ボリュームタイプと Kubernetes 変換

- `Type = "disk"` (既定):
  - PV/PVC の `accessModes` は既定で `[ReadWriteOnce]` (プロバイダドライバが `VolumeClass.AccessModes` で上書き可能)
  - 例: Azure Managed Disk (`disk.csi.azure.com`), AWS EBS (`ebs.csi.aws.com`), GCP Persistent Disk (`pd.csi.storage.gke.io`)
- `Type = "files"`:
  - PV/PVC の `accessModes` は既定で `[ReadWriteMany]` (プロバイダドライバが `VolumeClass.AccessModes` で設定)
  - CSI volumeAttributes には `Options` から `protocol` (`smb` | `nfs`), `skuName`, `availability` などを反映
  - 例: Azure Files (`file.csi.azure.com`, SMB/NFS), AWS EFS (`efs.csi.aws.com`), GCP Filestore (`filestore.csi.storage.gke.io`)
  - スナップショット機能は多くのプロバイダで非対応 (ドライバは `ErrNotSupported` を返す)

詳細は [K4x-ADR-014] と各プロバイダドライバ仕様 (例: [Kompox-ProviderDriver-AKS.ja.md]) を参照。

Compose の `services.<service>.volumes` は compose-go によりパースされる。

|種類|形式|Kompoxでの取り扱い|
|-|-|-|
|Abs path bind|`/sub/path:/mount/path`|エラー|
|Rel path bind|`./sub/path:/mount/path`|app.volumes[0] を参照し `/sub/path` を `/mount/path` にマウント|
|Root path volume|`name:/mount/path`|app.volumes[name] を参照し `/` を `/mount/path` にマウント|
|Sub path volume|`name/sub/path:/mount/path`|app.volumes[name] を参照し `/sub/path` を `/mount/path` にマウント|

参照する volume が見つからない場合はエラーとする。
app.volumes が空でも自動的に作成するようなことはしない。

`sub/path` の正規化や `/mount/path` の重複チェックは compose-go により行われる。

initContainers により各 volume の sub path ディレクトリを自動作成する。
作成するディレクトリのパーミッションは 1777 とする。

解決とエラー判定順

- compose-go により Compose service.volumes 行をパース
- 各 ServiceVolumeConfig について
  - `Target` または `Source` が空の場合はエラー
  - `Type` で場合分けして `name` と `subPath` を決定
    - `bind`
      - `Source` が `/` で始まる場合はエラー (Abs path bind)
      - `name={app.volumes[0].name}` `subPath={Source}` (Rel path bind)
    - `volume`
      - `Source` に `/` が含まれる場合: `name={Source:最初の"/"より前}` `subPath={Source:最初の"/"より後}` (Sub path volume)
      - `Source` に `/` が含まれない場合: `name={Source}` `subPath={空}` (Root path volume)
    - それ以外
      - エラー
  - `app.volumes[name]` が存在しない場合はエラー

設定例

```yaml
app:
  name: app1
  compose:
    services:
      app:
        image: app
        volumes:
        - /abs/path:/mnt/abs     # error
        - ./sub/path:/mnt/rel    # mount default:/sub/path on /mnt/rel
        - data:/mnt/root         # mount data:/ on /mnt/root
        - data/sub/path:/mnt/sub # mount data:/sub/path on /mnt/sub
  volumes:
  - name: default  # PV/PVC k4x-<spHASH>-default-<idHASH>-<volHASH>
    size: 32Gi
  - name: data     # PV/PVC k4x-<spHASH>-data-<idHASH>-<volHASH>
    size: 32Gi
```

### entrypoint/command

Compose の `entrypoint` と `command` を Kubernetes の `command` と `args` にマッピングする。

| Compose フィールド | Kubernetes フィールド | 説明 |
|-------------------|---------------------|------|
| `entrypoint` | `command` | コンテナのエントリポイント (実行ファイルパス) |
| `command` | `args` | コマンドライン引数 |

変換規則:
- `entrypoint` が指定されている場合、その値を `command` に設定する
- `command` が指定されている場合、その値を `args` に設定する
- どちらも未指定の場合、Kubernetes フィールドを設定しない (イメージのデフォルトを使用)
- 文字列形式 (shell form) と配列形式 (exec form) の両方をサポートする
  - 文字列形式: `/bin/sh -c` でラップして配列化
  - 配列形式: そのまま使用

例:

```yaml
# Compose
services:
  app:
    image: app
    entrypoint: ["/app/entrypoint.sh"]
    command: ["--config", "/etc/app.conf"]
```

```yaml
# Kubernetes
containers:
  - name: app
    image: app
    command: ["/app/entrypoint.sh"]
    args: ["--config", "/etc/app.conf"]
```

### x-kompox (リソース変換)

| キー | 意味 | K8s 出力 |
|------|------|---------|
| x-kompox.resources.cpu | CPU リクエスト (例: 100m) | resources.requests.cpu |
| x-kompox.resources.memory | メモリリクエスト (例: 256Mi) | resources.requests.memory |
| x-kompox.limits.cpu | CPU 上限 | resources.limits.cpu |
| x-kompox.limits.memory | メモリ上限 | resources.limits.memory |

未指定フィールドは出力しない。limits のみ指定時に requests を補完しない。

### Config/Secret

#### ConfigMap/Secret リソース

この節では、Kompox が生成・参照する ConfigMap/Secret の命名規則を統一表で示す。Compose の `configs`/`secrets` 由来のリソースと、CLI/compose で予約される Secret(pull/base/override)を含む。詳細仕様(マウントや制約、競合解決など)は次節「configs/secrets」を参照。

次の命名表に従って、必要な場合のみリソースが作成される。

|名前|タイプ|生成条件|説明|
|-|-|-|-|
|`<appName>-<componentName>--cfg-<configName>`|ConfigMap|Compose: `configs`(トップレベル定義を `services.<svc>.configs` から参照)|単一ファイル構成(テキスト)。UTF-8(BOM 無し)、NUL 無し、≤1 MiB。subPath 単一ファイル readOnly マウント|
|`<appName>-<componentName>--sec-<secretName>`|Secret(`Opaque`)|Compose: `secrets`(トップレベル定義を `services.<svc>.secrets` から参照)|単一ファイル秘密(テキスト/バイナリ)。UTF-8 かつ NUL 無しは `data`、それ以外は `binaryData`。subPath 単一ファイル readOnly マウント|
|`<appName>-<componentName>--pull`|Secret(`kubernetes.io/dockerconfigjson`)|CLI: `kompoxops secret pull`|コンテナレジストリ認証|
|`<appName>-<componentName>-<containerName>-base`|Secret(`Opaque`)|Compose: `env_file`|コンテナ環境変数|
|`<appName>-<componentName>-<containerName>-override`|Secret(`Opaque`)|CLI: `kompoxops secret env`|コンテナ環境変数|

注記(アノテーション・命名制約)
- すべての ConfigMap/Secret にアノテーション `kompox.dev/compose-content-hash` を付与する(内容から決定的に算出)。
- `<configName>`/`<secretName>` は DNS-1123 ラベル準拠(1..63 文字、英小文字・数字・ハイフン。先頭末尾は英数字)。
- リソース名の総文字数は Kubernetes の上限(≤253)以内とする。

Converter は pod template においてコンテナレジストリ認証 Secret を参照する imagePullSecrets を出力しない。
CLI による設定時に imagePullSecrets を patch する。

Converter は pod template の全コンテナにおいてコンテナ環境変数 Secret `-base` `-override` を参照する envFrom を常に出力する。
その際に optional: true として Secret リソースが存在しない場合でもエラーにならないようにする。

```yaml
envFrom:
- secretRef:
    name: <appName>-<componentName>-<containerName>-base
    optional: true
- secretRef:
    name: <appName>-<componentName>-<containerName>-override
    optional: true
```

Converter は pod template においてアノテーション `kompox.dev/compose-content-hash: <podContentHASH>` を出力しない。
デプロイランタイムがデプロイ完了後にすべての Secret リソースをスキャンして Deployment リソースに patch する。
このときの Field Manager は `kompox-runtime` を用いる。

`<podContentHASH>` は次のように計算する。
- Pod が参照する ConfigMap/Secret を次の順で列挙
  - imagePullSecrets(Secret のみ): 列挙順
  - envFrom(Secret のみ): コンテナ名の辞書順 → コンテナ内の列挙順
  - volumeMounts(ConfigMap/Secret): コンテナ名の辞書順 → コンテナ内の列挙順 → volumeMount 名の辞書順
- 列挙した各リソースの `kompox.dev/compose-content-hash` の文字列(存在しなければ空文字列)を取得
- 列挙順に連結した文字列を BASE として HASH を適用する

CLI による Secret リソース設定方法

```
# <appName>-<componentName>-<containerName>-override を設定・削除 (containerName=serviceName)
kompoxops secret env set -S service -f override.env
kompoxops secret env delete -S service
# <appName>-<componentName>--pull を設定・削除
kompoxops secret pull set -f ~/.docker/config.json
kompoxops secret pull delete
```

(上記の命名・制約・アノテーションを前提として、以下に configs/secrets の構文と変換仕様を示す。)

#### configs/secrets

Compose 標準の `configs`/`secrets` を単一ファイルの注入に用いる。`volumes` は「ディレクトリ専用」とし、単一ファイルは必ず `configs`/`secrets` で表現する。
変換結果は Compose 宣言を主要な決定要因とする。ただし、bind volumes の検証では、ファイルシステム状態を参照して単一ファイル bind を検出・拒否する（configs/secrets への移行を促す）。
詳細な設計と根拠は [K4x-ADR-005] を参照。

- Compose 側の構文
  - トップレベル定義(Compose ルート)
    - `configs:` / `secrets:` に名前付きエントリを定義する。
    - 形: `{ file | name | external }` を許可する。
  - `file`: 参照するローカルファイル(相対パスのみを推奨)。
  - `name`: 明示名を指定(省略時はキー名を使用)。
  - `external`: 外部定義として扱う(Kompox では name としてパススルー)。
  - サービス参照(`services.<svc>.configs` / `services.<svc>.secrets`)
  - 短縮形: `<name>`(`source: <name>` と同義)。
    - 拡張形: `{ source, target, mode? }` をサポート。`uid/gid` は無視する。
  - `target` はコンテナ内のファイルパス(単一ファイル)。省略時のデフォルト:
    - `configs`: `/<configName>` (Docker Swarm 仕様準拠)
    - `secrets`: `/run/secrets/<secretName>` (Docker Swarm 仕様準拠)
  - 同一 `target` の多重割り当てはエラー。
    - `volumes` と同一 `target` が競合する場合は `configs/secrets` を優先し、`volumes` エントリは無視して警告する。
  - `mode`: Kubernetes の `volumes[].{configMap|secret}.items[].mode` に反映する(8進数、例: `0444`)。未指定時は Kubernetes 既定(`0644`)に従う。

- Kubernetes への変換
  - configs → ConfigMap
    - 各エントリを 1 つの ConfigMap の `data` として格納。テキスト制約: UTF-8(BOM 無し)・NUL 文字なし。サイズ上限は 1 MiB/ConfigMap。
    - キー名は既定で `basename(file)`。サービス参照の `target` ファイル名との対応は `subPath=<key>` で行う。
    - マウント: `volumes` + `volumeMounts`(`subPath=<key>`, `mountPath=<target>`, `readOnly: true`)。
    - アノテーション: `kompox.dev/compose-content-hash` を付与。`data` のキーを辞書順に並べ、値(raw bytes)連結の SHA256 を基に決定的に計算する。
  - secrets → Secret(type: Opaque)
    - 内容はテキスト/バイナリを許容。UTF-8 かつ NUL 無しは `data`、それ以外は `binaryData` に格納。
    - マウント: `volumes` + `volumeMounts`(`subPath=<key>`, `mountPath=<target>`, `readOnly: true`)。
    - アノテーション: `kompox.dev/compose-content-hash` を付与。`data`/`binaryData` のキーを辞書順に並べ、値(raw bytes)連結の SHA256 を基に決定的に計算する。
  - リソース命名は「ConfigMap/Secret リソース」節を参照。
  - Pod 内の `volumes`/`volumeMounts` の命名規則:
    - ConfigMap をマウントする場合の `volumes[].name` と `volumeMounts[].name` は `cfg-<configName>`。
    - Secret をマウントする場合の `volumes[].name` と `volumeMounts[].name` は `sec-<secretName>`。
    - ここで `<configName>`/`<secretName>` はトップレベル `configs`/`secrets` のエントリ名(DNS-1123 準拠)。

- volumes ポリシー(ディレクトリ専用)
  - 相対 bind: `./sub/dir:/mount` は `app.volumes[0]` を参照し `subPath=sub/dir` として PVC サブパスでマウント。
  - 絶対 bind: `/host:/mount` はエラー。
  - 単一ファイル bind の検出:
    - bind source が実際に存在し、かつファイルの場合 → エラー（解決策を提示: "単一ファイルは configs/secrets を使用"）
    - bind source が存在しない場合 → 許可（ディレクトリとして自動作成される想定）
    - bind source が存在し、かつディレクトリの場合 → 許可（PVC subPath として処理）
  - `target` が `configs/secrets` と衝突する場合:
    - 同一 `target` を `configs/secrets` が占有していれば `volumes` を無視して警告。

例(抜粋)

Compose（明示的な target 指定）:

```yaml
services:
  app:
    image: app
    configs:
      - source: nginx-conf
        target: /etc/nginx/nginx.conf
        mode: 0444
    secrets:
      - source: api-key
        target: /run/secrets/api-key
        mode: 0444
configs:
  nginx-conf:
    file: ./nginx/nginx.conf
secrets:
  api-key:
    file: ./secrets/api-key.txt
```

Compose（短縮形: デフォルト target 使用）:

```yaml
services:
  app:
    image: app
    configs:
      - nginx-conf  # デフォルト target: /nginx-conf
    secrets:
      - api-key     # デフォルト target: /run/secrets/api-key
configs:
  nginx-conf:
    file: ./nginx/nginx.conf
secrets:
  api-key:
    file: ./secrets/api-key.txt
```

Kubernetes(概念図):

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <app>-<comp>--cfg-nginx-conf
  annotations:
    kompox.dev/compose-content-hash: <hash>
data:
  nginx.conf: |
    ...
---
apiVersion: v1
kind: Secret
metadata:
  name: <app>-<comp>--sec-api-key
  annotations:
    kompox.dev/compose-content-hash: <hash>
type: Opaque
data:
  api-key: <base64>
---
# Deployment の volumeMounts(抜粋)
volumeMounts:
  - name: cfg-nginx-conf
    mountPath: /etc/nginx/nginx.conf
    subPath: nginx.conf
    readOnly: true
  - name: sec-api-key
    mountPath: /run/secrets/api-key
    subPath: api-key
    readOnly: true
---
# Deployment の volumes(抜粋)
volumes:
  - name: cfg-nginx-conf
    configMap:
      name: <app>-<comp>--cfg-nginx-conf
      items:
        - key: nginx.conf
          path: nginx.conf
          mode: 0444
  - name: sec-api-key
    secret:
      secretName: <app>-<comp>--sec-api-key
      items:
        - key: api-key
          path: api-key
          mode: 0444
```

#### env_file

Compose の `env_file` は次のように取り扱う。

- 列挙順にすべてのファイルを読み込みマージし 1 つの Secret リソースを生成する。
- Secret リソースの名前は `<appName>-<componentName>-<containerName>-base` とする。
- `required` フィールド対応:
  - `required: false` を指定したファイルが存在しない場合、エラーにせず空のマップとして扱う（Docker Compose 仕様準拠）
  - `required: true`（デフォルト）の場合は従来通りファイル不在時にエラー
- ファイルパス制約:
  - 相対パスのみ (正規化後に `..` を含むものはエラー)
  - symlink / ディレクトリ / デバイス / FIFO / ソケットはエラー (外部脱出や非決定性を防ぐ)
  - UTF-8 (BOM なし) で読めない場合はエラー
- サポート形式: `.env` / `.yml` `.yaml` / `.json`
- 重複キーは「後勝ち」(後から読み込んだ値で上書き)。
- 1 行 / 1 キーの上書き発生ごとに (実装で verbose 指定時) 警告を出せる。
- `${VAR}` などの変数参照は展開しない。`"${VAR}"` を含む値はそのまま保持し、(必要に応じ) 警告可能。
- 値に NUL バイト (0x00) や制御文字 (0x01–0x08, 0x0B, 0x0C, 0x0E–0x1F, 0x7F) が含まれる場合はエラー
- マージ後の Secret リソースサイズの合計 (全キー名 + 値バイト長) > 1,000,000 bytes でエラー

`.env` パーサ仕様:

- 行単位読み込み。空行 / 先頭 `#` 行は無視。
- `export ` プレフィックスは除去。
- 行をそのまま扱い、`=` の最初の出現で左右を分割。
  - 左辺 KEY は前後空白トリム後に検証。右辺 VALUE は「未クオートなら」先頭の 1 個分の空白のみ除去し末尾は保持 (末尾空白を意図喪失させない)。
- KEY 正規表現: `[A-Za-z_][A-Za-z0-9_]*`
- VALUE 形式:
  - 未クオート: 行末までそのまま (内部 `#` はコメント扱いしない)
  - ダブルクオート: 外側除去し以下エスケープ解釈: `\\` `\"` `\n` `\r` `\t`
  - シングルクオート: 外側除去 (エスケープ解釈なし)
- `=` を含まない行 / KEY 不正 / 重複するエスケープ列はエラー
- 同一ファイル内の重複 KEY は後勝ち (他ファイルと同様)

YAML / JSON パーサ仕様:

- トップレベルがオブジェクトであること
- キーは文字列
- 値型は string / number / boolean を許容 (number, boolean は文字列へ変換)
- null / 配列 / オブジェクト値はエラー

#### environment

Compose の `environment` は次のように取り扱う。

- 個別に `env` 出力し `env_file` の値を上書きできる
- `environment` で指定したキーは Secret へは追加しない (差分が Pod 定義の変更として残りやすい)

### Ports/Service/Ingress

Compose の ports 指定の仕様
- (HTTP 前提) Ingress 経由で利用するためアプリ層は HTTP 想定。Service は TCP ポートで生成。
- `hostPort:containerPort` の形式のみサポートする。
- 複数のサービスが同じ `containerPort` を使用する設定は明示的なエラーとする (コンテナは同一Podで稼働するため)。

app.ingress スキーマ

```yaml
app:
  ingress:
    certResolver: staging | production (デフォルト: {cluster.ingress.certResolver})
    rules:
      - name: <portName>
        port: <hostPort:int>
        hosts: [<fqdn>, ...]   # 1件以上
```

- name: `^[a-z]([-a-z0-9]{0,14})$` (Kubernetes Service port 名制約)
- port: Compose `hostPort` のいずれか。未定義ならエラー。
- 同一 port を複数エントリが参照することは禁止 (エラー)。
- hosts: 各要素 DNS-1123 subdomain。エントリ内重複は 1 回目のみ採用し警告。異なるエントリ間で同一 FQDN 再出現はエラー。
- app.ingress.rules が空 (または未指定) の場合 Ingress を生成しない。

Service 生成の仕様
- `ports` は app.ingress.rules の定義順。
- `port` = `hostPort`, `targetPort` = 対応する `containerPort`。
- 複数サービス (Compose) が同一 containerPort を公開 (ports に含める) する構成はエラー。

デフォルトドメイン Ingress 生成の仕様
- `app.ingress.rules` が空配列ではなく、かつ `cluster.ingress.domain` が空文字列でないときのみ生成
- 名前は `<appName>-<componentName>-default`
- ingressClassName は `traefik`
- `rules`
  - `app.ingress.rules` の各エントリに対して1つを出力
  - `host` は `<appName>-idHASH-<port>.{cluster.ingress.domain}`
  - ここで `<port>` は `app.ingress.rules.port`(Compose の `hostPort`)
    - 例: `main(8080→80)` は `app1-idHASH-8080.ops.kompox.dev`、`admin(8081→8080)` は `app1-idHASH-8081.ops.kompox.dev`
  - `path: /` および `pathType: Prefix`
- annotations 設定(certresolver を設定せず静的 TLS 証明書を使用する)
```yaml
traefik.ingress.kubernetes.io/router.entrypoints: websecure
traefik.ingress.kubernetes.io/router.tls: "true"
```

カスタムドメイン Ingress 生成の仕様
- `app.ingress.rules` が空配列でないときのみ生成
- 名前は `<appName>-<componentName>-custom`
- ingressClassName は `traefik`
- `rules`
  - `app.ingress.rules` の `hosts` 配列の各要素ごとに1つを出力
  - `path: /` および `pathType: Prefix`
- annotations 設定(certresolver を設定して ACME TLS 証明書を使用する)
```yaml
traefik.ingress.kubernetes.io/router.entrypoints: websecure
traefik.ingress.kubernetes.io/router.tls: "true"
traefik.ingress.kubernetes.io/router.tls.certresolver: {app.ingress.certResolver}
```

カスタムドメインホスト名の制約
- `cluster.ingress.domain` で指定したドメイン以下のホスト名を指定するとエラー
- `app.ingress.rules` の同一エントリ内の重複は警告、異なるエントリ間の重複はエラー

参考: Traefik Helm values.yaml 設定
```yaml
persistence:
  enabled: true
  accessMode: ReadWriteOnce
  size: 1Gi
  path: /data

additionalArguments:
  # production
  - --certificatesresolvers.production.acme.tlschallenge=true
  - --certificatesresolvers.production.acme.caserver=https://acme-v02.api.letsencrypt.org/directory
  - --certificatesresolvers.production.acme.email={cluster.ingress.certEmail}
  - --certificatesresolvers.production.acme.storage=/data/acme-production.json
  # staging
  - --certificatesresolvers.staging.acme.tlschallenge=true
  - --certificatesresolvers.staging.acme.caserver=https://acme-staging-v02.api.letsencrypt.org/directory
  - --certificatesresolvers.staging.acme.email={cluster.ingress.certEmail}
  - --certificatesresolvers.staging.acme.storage=/data/acme-staging.json
```

### ディスクの切り替え

- 新しいクラウドディスクに切り替える場合は新しい `<volHASH>` を持つ PV / PVC (同名) を追加し、Deployment の claimName をその新 PVC 名へ変更する (同一 apply 可)。
- 切替時は旧 PV/PVC を即削除せず動作確認後に手動削除。
- アノテーション `kompox.dev/volume-handle-previous` の設定は PV ごとに設定する。
- ロールバック (旧世代へ戻す) は旧 PV/PVC を削除していない場合のみ可能。

### クラスタの切り替え

手順 (同一クラウドディスクを再利用):
1. 旧クラスタ Namespace 削除 (namespaced リソース一括削除)
2. PVC 削除 → PV 状態 Released
3. PV 削除 (クラウド側 detach 完了確認)
4. 新クラスタで Namespace / PV / PVC / Deployment / Service / Ingress を apply

### Deployment

app.deployment スキーマ

```yaml
app:
  deployment:
    pool: <pool>
    zone: <zone>
```

- pool: ノードプールの種類。未指定の場合は `user`。
Deployment.spec.template.spec.nodeSelector に `kompox.dev/node-pool: <pool>` を設定する。
- zone: プロバイダドライバがサポートするゾーン名 (例: `"1"`)。
指定があった場合のみ Deployment.spec.template.spec.nodeSelector に `kompox.dev/node-zone: <zone>` を設定する。

### Network Policy

各アプリの Namespace に対して、ネットワークアクセスをコントロールするための NetworkPolicy リソースをひとつ作成する。

アクセスの制御は Namespace ベースで行う。

- Ingress
  - クラスタのシステム Namespace (kube-system) や Ingress Controller (traefik) Namespace からの接続は受け付ける
  - その他のアプリ Namespace からの接続はブロックする。
- Egress
  - 設定なし。基本的にそのまま送信する。

### Service Account

各アプリの Namespace 内 Pod に対して、次の操作が可能な権限を持つ ServiceAccount/Role/RoleBinding リソースを作成する。

|api|resource|verb|
|-|-|-|
||pods|get list watch|
||pods/log|get, watch|
||pods/exec|create|
||pods/portforward|create|
||pods/attach|create|
||pods/ephemeralcontainers|update|
||events|get, list, watch|
||services|get, list, watch|
||endpoints|get, list, watch|
|apps|deployments|get list watch|
|apps|replicasets|get list watch|

この Service Account は Kompox ユーザー(人間)用であり、ワークロードの Pod へは自動で割り当てない。

## 例1

### kompoxops.yml

```yaml
version: v1
workspace:
  name: ops
provider:
  name: aks1
cluster:
  name: cluster1
  ingress:
    controller: traefik
    namespace: traefik
    certEmail: admin@example.com
    certResolver: staging
    domain: ops.kompox.dev
    certificates:
      - name: foo-cert1
        source: https://kv-foo.vault.azure.net/secrets/cert1
      - name: bar-cert2
        source: https://kv-bar.vault.azure.net/secrets/cert2
app:
  name: app1
  compose:
    services:
      app:
        image: ghcr.io/kompox/app
        env_file:
          - env.yml
        environment:
          TZ: Asia/Tokyo
        configs:
          - source: nginx-conf
            target: /etc/nginx/nginx.conf
            mode: 0444
        secrets:
          - source: api-key
            target: /run/secrets/api-key
            mode: 0444
        ports:
          - "8080:80"
          - "8081:8080"
        volumes:
          - ./data/app:/data
        x-kompox:
          resources:
            cpu: 100m
            memory: 256Mi
          limits:
            cpu: 200m
            memory: 512Mi
      postgres:
        image: postgres
        environment:
          POSTGRES_PASSWORD: secret
        volumes:
          - db/data:/var/lib/postgresql/data
        x-kompox:
          resources:
            cpu: 100m
            memory: 256Mi
          limits:
            cpu: 200m
            memory: 512Mi
    configs:
      nginx-conf:
        file: ./nginx/nginx.conf
    secrets:
      api-key:
        file: ./secrets/api-key.txt
  ingress:
    certResolver: staging
    rules:
      - name: main
        port: 8080
        hosts: [www.custom.kompox.dev]
      - name: admin
        port: 8081
        hosts: [admin.custom.kompox.dev]
  volumes:
    - name: default
      size: 32Gi
    - name: db
      size: 64Gi
  deployment:
    zone: "1"
```

### Kubernetes Manifest

#### Namespace/NetworkPolicy/ServiceAccount/Role/RoleBinding

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: k4x-spHASH-app1-idHASH
  labels:
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
    app.kubernetes.io/managed-by: kompox
    kompox.dev/app-instance-hash: inHASH
    kompox.dev/app-id-hash: idHASH
  annotations:
    kompox.dev/app: ops/aks1/cluster1/app1
    kompox.dev/provider-driver: aks
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: app1
  namespace: k4x-spHASH-app1-idHASH
  labels:
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
    app.kubernetes.io/managed-by: kompox
    kompox.dev/app-instance-hash: inHASH
    kompox.dev/app-id-hash: idHASH
spec:
  podSelector: {}
  policyTypes:
    - Ingress
  ingress:
    - from:
        - podSelector: {}
        - namespaceSelector:
            matchExpressions:
              - key: kubernetes.io/metadata.name
                operator: In
                values: ["kube-system"]
        - namespaceSelector:
            matchExpressions:
              - key: kubernetes.io/metadata.name
                operator: In
                values: ["traefik"]
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: app1
  namespace: k4x-spHASH-app1-idHASH
  labels:
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
    app.kubernetes.io/managed-by: kompox
    kompox.dev/app-instance-hash: inHASH
    kompox.dev/app-id-hash: idHASH
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: app1
  namespace: k4x-spHASH-app1-idHASH
  labels:
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
    app.kubernetes.io/managed-by: kompox
    kompox.dev/app-instance-hash: inHASH
    kompox.dev/app-id-hash: idHASH
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["pods/log"]
    verbs: ["get", "watch"]
  - apiGroups: [""]
    resources: ["pods/exec", "pods/portforward", "pods/attach"]
    verbs: ["create"]
  - apiGroups: [""]
    resources: ["events", "services", "endpoints"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["deployments", "replicasets"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["pods/ephemeralcontainers"]
    verbs: ["update"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: app1
  namespace: k4x-spHASH-app1-idHASH
  labels:
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
    app.kubernetes.io/managed-by: kompox
    kompox.dev/app-instance-hash: inHASH
    kompox.dev/app-id-hash: idHASH
subjects:
  - kind: ServiceAccount
    name: app1
    namespace: k4x-spHASH-app1-idHASH
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: app1
```

#### PersistentVolume/PersistentVolumeClaim

```yaml
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: k4x-spHASH-default-idHASH-volHASH
  labels:
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
    app.kubernetes.io/managed-by: kompox
    kompox.dev/app-instance-hash: inHASH
    kompox.dev/app-id-hash: idHASH
  annotations:
    # 初回デプロイ: kompox.dev/volume-handle-previous は未設定
spec:
  capacity:
    storage: 32Gi
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  storageClassName: managed-csi
  csi:
    driver: disk.csi.azure.com
    volumeHandle: /subscriptions/...
    volumeAttributes:
      fsType: ext4
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: k4x-spHASH-db-idHASH-volHASH
  labels:
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
    app.kubernetes.io/managed-by: kompox
    kompox.dev/app-instance-hash: inHASH
    kompox.dev/app-id-hash: idHASH
  annotations:
    # 初回デプロイ: kompox.dev/volume-handle-previous は未設定
spec:
  capacity:
    storage: 64Gi
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  storageClassName: managed-csi
  csi:
    driver: disk.csi.azure.com
    volumeHandle: /subscriptions/...
    volumeAttributes:
      fsType: ext4
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: k4x-spHASH-default-idHASH-volHASH
  namespace: k4x-spHASH-app1-idHASH
  labels:
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
    app.kubernetes.io/managed-by: kompox
    kompox.dev/app-instance-hash: inHASH
    kompox.dev/app-id-hash: idHASH
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 32Gi
  storageClassName: managed-csi
  volumeName: k4x-spHASH-default-idHASH-volHASH
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: k4x-spHASH-db-idHASH-volHASH
  namespace: k4x-spHASH-app1-idHASH
  labels:
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
    app.kubernetes.io/managed-by: kompox
    kompox.dev/app-instance-hash: inHASH
    kompox.dev/app-id-hash: idHASH
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 64Gi
  storageClassName: managed-csi
  volumeName: k4x-spHASH-db-idHASH-volHASH
```

#### Deployment/Service/Secret/Ingress

```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app1-app
  namespace: k4x-spHASH-app1-idHASH
  labels:
    app: app1-app
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
    app.kubernetes.io/component: app
    app.kubernetes.io/managed-by: kompox
    kompox.dev/app-instance-hash: inHASH
    kompox.dev/app-id-hash: idHASH
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: app1-app
  template:
    metadata:
      labels:
        app: app1-app
        app.kubernetes.io/name: app1
        app.kubernetes.io/instance: app1-inHASH
        app.kubernetes.io/component: app
        app.kubernetes.io/managed-by: kompox
        kompox.dev/app-instance-hash: inHASH
        kompox.dev/app-id-hash: idHASH
    spec:
      nodeSelector:
        kompox.dev/node-pool: user
        kompox.dev/node-zone: "1"
      containers:
      - name: app
        image: ghcr.io/kompox/app
        envFrom:
        - secretRef:
            name: app1-app-app-base
            optional: true
        - secretRef:
            name: app1-app-app-override
            optional: true
        env:
        - name: TZ
          value: Asia/Tokyo
        ports:
        - containerPort: 80
        - containerPort: 8080
        volumeMounts:
        - name: cfg-nginx-conf
          mountPath: /etc/nginx/nginx.conf
          subPath: nginx.conf
          readOnly: true
        - name: sec-api-key
          mountPath: /run/secrets/api-key
          subPath: api-key
          readOnly: true
        - name: default
          mountPath: /data
          subPath: data/app
        resources:
          requests:
            cpu: 100m
            memory: 256Mi
          limits:
            cpu: 200m
            memory: 512Mi
      - name: postgres
        image: postgres
        envFrom:
        - secretRef:
            name: app1-app-postgres-base
            optional: true
        - secretRef:
            name: app1-app-postgres-override
            optional: true
        env:
        - name: POSTGRES_PASSWORD
          value: secret        
        volumeMounts:
        - name: db
          mountPath: /var/lib/postgresql/data
          subPath: data
        resources:
          requests:
            cpu: 100m
            memory: 256Mi
          limits:
            cpu: 200m
            memory: 512Mi
      initContainers:
      - name: init-volume-subpaths
        image: busybox:1.36
        command:
          - sh
          - -c
          - |
            mkdir -m 1777 -p /work/default/data/app
            mkdir -m 1777 -p /work/db/data
        volumeMounts:
          - name: default
            mountPath: /work/default
          - name: db
            mountPath: /work/db
      volumes:
        - name: cfg-nginx-conf
          configMap:
            name: app1-app--cfg-nginx-conf
            items:
              - key: nginx.conf
                path: nginx.conf
                mode: 0444
        - name: sec-api-key
          secret:
            secretName: app1-app--sec-api-key
            items:
              - key: api-key
                path: api-key
                mode: 0444
        - name: default
          persistentVolumeClaim:
            claimName: k4x-spHASH-default-idHASH-volHASH
        - name: db
          persistentVolumeClaim:
            claimName: k4x-spHASH-db-idHASH-volHASH
---
apiVersion: v1
kind: Service
metadata:
  name: app1-app
  namespace: k4x-spHASH-app1-idHASH
  labels:
    app: app1-app
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
    app.kubernetes.io/component: app
    app.kubernetes.io/managed-by: kompox
    kompox.dev/app-instance-hash: inHASH
    kompox.dev/app-id-hash: idHASH
spec:
  ports:
  - name: main
    port: 8080
    targetPort: 80
  - name: admin
    port: 8081
    targetPort: 8080
  selector:
    app: app1-app
---
apiVersion: v1
kind: Service
metadata:
  name: app
  namespace: k4x-spHASH-app1-idHASH
  labels:
    app: app1-app
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
    app.kubernetes.io/component: app
    app.kubernetes.io/managed-by: kompox
    kompox.dev/app-instance-hash: inHASH
    kompox.dev/app-id-hash: idHASH
    kompox.dev/compose-service-headless: true
spec:
  clusterIP: None
  selector:
    app: app1-app
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
  namespace: k4x-spHASH-app1-idHASH
  labels:
    app: app1-app
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
    app.kubernetes.io/component: app
    app.kubernetes.io/managed-by: kompox
    kompox.dev/app-instance-hash: inHASH
    kompox.dev/app-id-hash: idHASH
    kompox.dev/compose-service-headless: true
spec:
  clusterIP: None
  selector:
    app: app1-app
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: app1-app--cfg-nginx-conf
  namespace: k4x-spHASH-app1-idHASH
  labels:
    app: app1-app
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
    app.kubernetes.io/component: app
    app.kubernetes.io/managed-by: kompox
    kompox.dev/app-instance-hash: inHASH
    kompox.dev/app-id-hash: idHASH
  annotations:
    kompox.dev/compose-content-hash: <hash>
data:
  nginx.conf: |
    ...
---
apiVersion: v1
kind: Secret
metadata:
  name: app1-app--sec-api-key
  namespace: k4x-spHASH-app1-idHASH
  labels:
    app: app1-app
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
    app.kubernetes.io/component: app
    app.kubernetes.io/managed-by: kompox
    kompox.dev/app-instance-hash: inHASH
    kompox.dev/app-id-hash: idHASH
  annotations:
    kompox.dev/compose-content-hash: <hash>
type: Opaque
data:
  api-key: <base64>
---
apiVersion: v1
kind: Secret
metadata:
  name: app1-app-app-base
  namespace: k4x-spHASH-app1-idHASH
  labels:
    app: app1-app
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
    app.kubernetes.io/component: app
    app.kubernetes.io/managed-by: kompox
    kompox.dev/app-instance-hash: inHASH
    kompox.dev/app-id-hash: idHASH
  annotations:
    kompox.dev/compose-content-hash: containerContentHASH
type: Opaque
data:
  USERNAME: YWRtaW4=
  PASSWORD: MWYyZDFlMmU2N2Rm
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app1-app-default
  namespace: k4x-spHASH-app1-idHASH
  labels:
    app: app1-app
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
    app.kubernetes.io/component: app
    app.kubernetes.io/managed-by: kompox
    kompox.dev/app-instance-hash: inHASH
    kompox.dev/app-id-hash: idHASH
  annotations:
    traefik.ingress.kubernetes.io/router.entrypoints: websecure
    traefik.ingress.kubernetes.io/router.tls: "true"
spec:
  ingressClassName: traefik
  rules:
    - host: app1-idHASH-8080.ops.kompox.dev
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: app1-app
                port:
                  name: main
    - host: app1-idHASH-8081.ops.kompox.dev
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: app1-app
                port:
                  name: admin
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app1-app-custom
  namespace: k4x-spHASH-app1-idHASH
  labels:
    app: app1-app
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
    app.kubernetes.io/component: app
    app.kubernetes.io/managed-by: kompox
    kompox.dev/app-instance-hash: inHASH
    kompox.dev/app-id-hash: idHASH
  annotations:
    traefik.ingress.kubernetes.io/router.entrypoints: websecure
    traefik.ingress.kubernetes.io/router.tls: "true"
    traefik.ingress.kubernetes.io/router.tls.certresolver: staging
spec:
  ingressClassName: traefik
  rules:
    - host: www.custom.kompox.dev
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: app1-app
                port:
                  name: main
    - host: admin.custom.kompox.dev
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: app1-app
                port:
                  name: admin
```

<!-- ADR References -->

[K4x-ADR-003]: ../adr/K4x-ADR-003.md
[K4x-ADR-005]: ../adr/K4x-ADR-005.md
[K4x-ADR-014]: ../adr/K4x-ADR-014.md

<!-- Design References -->

[Kompox-ProviderDriver-AKS.ja.md]: ./Kompox-ProviderDriver-AKS.ja.md
