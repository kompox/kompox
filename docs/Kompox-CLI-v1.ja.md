# Kompox PaaS CLI

## Overview

次のようなコマンドを実装する。

|コマンド名|説明|
|-|-|
|kompoxops|Kompox 仕様のクラウドリソースデプロイ・運用ツール<br>Kompox PaaS とは独立した設定ファイル `kompoxops.yml` を読み取って動作する CLI|
|kompoxsvc|Kompox PaaS REST API サーバと管理ツール<br>REST API サーバは `kompoxsvc server` で起動するコンテナWebアプリ|
|kompox|Kompox PaaS REST API クライアント CLI|

## kompoxops

### kompoxops Overview

kompoxops は Kompox PaaS 準拠のデプロイツールである。

```
kompoxops init              設定ファイルの雛形作成
kompoxops cluster           クラスタ操作
kompoxops app               アプリ操作
kompoxops disk              ディスク操作
kompoxops snapshot          スナップショット操作
kompoxops tool             メンテナンス用ランナー操作（アプリNS内）
kompoxops admin             管理ツール
```

共通オプション

- `--db-url <URL>` 永続化DBの接続URL。環境変数 KOMPOX_DB_URL で指定可能。

### kompoxops.yml

kompoxops は永続化DBなしで動作することができる。

リソース定義ファイル kompoxops.yml をグローバルオプションで `--db-url file:kompoxops.yml` のように指定すると、
リソース定義をインメモリDBに読み込み、それに対して Kompox PaaS と同様の操作を行うことができる。

kompoxops.yml には service, provider, cluster, app の各定義が 1 つずつ含まれる。
それらは kompoxops の memory データベースストアに読み込まれて
自動的に app → cluster → provider → service の依存関係が設定される。

kompoxops.yml の例:

```yaml
version: v1
service:
  name: ops
provider:
  name: aks1
  driver: aks
  settings:
    AZURE_AUTH_METHOD: azure_cli
    AZURE_SUBSCRIPTION_ID: 34809bd3-31b4-4331-9376-49a32a9616f2
    AZURE_LOCATION: japaneast
cluster:
  name: cluster1
  existing: false
  ingress:
    controller: traefik
    namespace: traefik
    domain: ops.kompox.dev
  settings:
    AZURE_RESOURCE_GROUP_NAME: rg-cluster1
app:
  name: app1
  compose:
    services:
      app:
        image: ghcr.io/kompox/app
        environment:
          TZ: Asia/Tokyo
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
  ingress:
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
```

### kompoxops cluster

K8s クラスタ操作を行う。

```
kompoxops cluster provision --cluster-name <clusterName>     Cluster リソース準拠の K8s クラスタを作成開始 (existingがfalseの場合)
kompoxops cluster deprovision --cluster-name <clusterName>   Cluster リソース準拠の K8s クラスタを削除開始 (existingがfalseの場合)
kompoxops cluster install --cluster-name <clusterName>       K8s クラスタ内のリソースをインストール開始
kompoxops cluster uninstall --cluster-name <clusterName>     K8s クラスタ内のリソースをアンインストール開始
kompoxops cluster status --cluster-name <clusterName>        K8s クラスタのステータスを表示
kompoxops cluster kubeconfig --cluster-name <clusterName>    kubectl 用 kubeconfig を取得/統合
```

共通オプション

- `--cluster-name | -C` クラスタ名を指定 (デフォルト: kompoxops.yml の cluster.name)

provision/deprovision コマンドは service/provider/cluster リソースの設定に従って K8s クラスタを作成・削除する。
既存のクラスタを参照する場合は cluster.existing を true に設定する。
cluster.existing が true の場合 provision/deprovision は常に成功を返す。

install/uninstall コマンドは K8s クラスタ内のリソースを作成・削除する。
Namespace や Traefik Proxy などの Ingress Controller を含む。

クラスタの作成方法や接続方法は cluster.settings で設定する。
設定内容は provider.driver により異なる。

status コマンドにより K8s クラスタと内部リソースの状態について次の項目が取得できる。

```
existing: bool          cluster.existing の設定値
provisioned: bool       K8s クラスタが存在するとき true (existingがtrueの場合も実際に存在するか調べる)
installed: bool         K8s クラスタ内のリソースが存在するとき true
ingressGlobalIP: string クラスタ Ingress/LoadBalancer のグローバル IP アドレス（存在する場合）
ingressFQDN: string     クラスタ Ingress/LoadBalancer の FQDN（存在する場合）
```

provision/deprovision/install/uninstall は status により実行可否が変わる。
これらのコマンドは時間のかかる操作であるため実行すると即座に終了する。
冪等性を持つため複数回実行してもエラーにはならない。

|コマンド|エラー発生条件|実行内容|
|-|-|-|
|provision||existing=trueなら何もしない<br>provisined=falseならK8sクラスタ作成開始|
|deprovision|installed=true|existing=trueなら何もしない<br>provisioned=trueならK8sクラスタ削除開始|
|install|provisioned=false|installed=falseならK8sクラスタ内リソース作成開始|
|uninstall||installed=trueならK8sクラスタ内リソース削除開始|

#### kompoxops cluster provision

Cluster リソース準拠の AKS/K8s クラスタを作成開始します（idempotent）。`existing=true` の場合は何もしません。

#### kompoxops cluster deprovision

対象クラスタのリソースグループを削除します（idempotent）。`installed=true` の場合はエラーです。

#### kompoxops cluster install

Ingress Controller などのクラスタ内リソースをインストールします。`provisioned=false` の場合はエラーです。

#### kompoxops cluster uninstall

クラスタ内リソースをアンインストールします（best-effort）。

#### kompoxops cluster status

クラスタの `existing`/`provisioned`/`installed` に加えて Ingress のグローバル IP/FQDN を JSON で表示します（利用可能な場合）。

#### kompoxops cluster kubeconfig

Provider Driver からクラスタの kubeconfig（管理者資格）を取得し、標準出力/ファイル保存/既存 kubeconfig への統合を行う。

主なオプション:

- `-o, --out FILE` 出力先（`-` は標準出力。stdout は必ず明示的に `-o -` を指定）
- `--merge` 既存 kubeconfig に統合（既定: `~/.kube/config`）
- `--kubeconfig PATH` 統合先ファイル（既定: `~/.kube/config`）
- `--context NAME` 生成する context 名（既定: `kompoxops-<clusterName>`）
- `--namespace NS` context のデフォルト namespace
- `--set-current` 統合後に current-context を新しい context に設定
- `--force` 同名エントリがある場合に上書き（未指定時は `-1`, `-2` のように自動ユニーク化）
- `--temp` セキュアな一時ファイルに保存してパスを出力
- `--print-export` `export KUBECONFIG=...` を出力（`--out` か `--temp` と併用）
- `--format yaml|json` 標準出力時のフォーマット（既定: `yaml`）
- `--dry-run` 統合時の差分を要約表示のみ（書き込み無し）
- `--cred admin|user` 要求する資格（現状 `admin` のみサポート）
- `--timeout DURATION` 取得のタイムアウト（既定: `2m`）

注意:

- 何も出力先を指定しない（`--merge` なし、`--temp` なし、`--out` 未指定）の場合はエラーになります。
- stdout 出力は `-o -` を明示したときのみ行われます。

使用例:

```
# 標準出力へ（明示）
kompoxops cluster kubeconfig -C mycluster -o -

# 一時ファイル化と KUBECONFIG エクスポート
eval "$(kompoxops cluster kubeconfig -C mycluster --temp --print-export)"

# 既存へ統合して current-context を切替え
kompoxops cluster kubeconfig -C mycluster --merge --set-current

# context と namespace を指定して保存
kompoxops cluster kubeconfig -C mycluster --merge --context kompox/prod --namespace staging
```

### kompoxops app

アプリの操作を行う。

```
kompoxops app validate --app-name <appName>
kompoxops app deploy   --app-name <appName>
kompoxops app destroy  --app-name <appName>
kompoxops app status   --app-name <appName>
kompoxops app exec     --app-name <appName> -- <command> [args...]
```

共通オプション

- `--app-name | -A` アプリ名を指定 (デフォルト: kompoxops.yml の app.name)

validate コマンドは app.compose の内容を検証し Kompose により K8s マニフェストに変換する。
YAML 構文エラーや制約違反が検出された場合はエラーを返す。

- `--out-compose FILE` を指定すると正規化した Docker Compose の YAML ドキュメントを出力する (`-` は stdout)
- `--out-manifest FILE` を指定すると K8s マニフェストの YAML ドキュメントを出力する (`-` は stdout)

#### kompoxops app validate

Compose の検証と K8s マニフェスト生成を行います。`--out-compose`/`--out-manifest` でファイル出力可能です。

#### kompoxops app deploy

検証・変換済みのリソースを対象クラスタに適用します（冪等)。

#### kompoxops app destroy

デプロイ済みリソースをクラスタから削除します (冪等)。

- 次のラベルがついたリソースのみ削除する
  - `app.kubernetes.io/instance: app1-inHASH`
  - `app.kubernetes.io/managed-by: kompox`
- 既定で Namespace 以外のリソース（PV/PVC を含む）を削除
- `--delete-namespace` を指定すると Namespace リソースも削除

備考:
- PV/PVC を削除しても、StorageClass/PV の ReclaimPolicy が Retain の場合はクラウドディスク本体は保持されます。

#### kompoxops app status

アプリの状態を JSON で表示します。Namespace、デプロイ状況、Ingress のホスト名一覧などを返します。

出力例

```json
{
  "app_id": "...",
  "app_name": "app1",
  "cluster_id": "...",
  "cluster_name": "cluster1",
  "namespace": "kompox-app1-<idHASH>",
  "deployed": true,
  "ingress_hosts": [
    "app1-<idHASH>-8080.example.com",
    "www.custom.example.com"
  ]
}
```

備考
- `namespace` はアプリの実リソースが存在する Kubernetes Namespace を示します。
- `ingress_hosts` には `app.ingress.rules.hosts` で指定したカスタムドメインに加え、`cluster.ingress.domain` が設定されている場合は `<appName>-<idHASH>-<port>.<domain>` の自動生成ドメインが含まれます。

#### kompoxops app exec

アプリの Namespace 内で稼働中の Pod に対してコマンドを実行します。対話モードにも対応します。

使用法:

```
kompoxops app exec -A <appName> [-i] [-t] [-e ESC] [-c CONTAINER] -- <command> [args...]
```

オプション:

- `-i, --stdin` 標準入力を接続
- `-t, --tty` TTY を割り当て（bash 等の対話時に推奨）
- `-e, --escape` デタッチ用エスケープシーケンス（既定: `~.`、`none` で無効化）
- `-c, --container` 実行対象のコンテナ名（未指定時は最初のコンテナ）

挙動:

- 対象 Pod の選択はアプリの Namespace 内で実施します。
- `kompox.dev/tool-runner=true` が付与されたメンテナンス用ランナー Pod は除外します。
- 少なくとも 1 つの Ready コンテナを持つ Pod を優先し、無ければ非終了中の Pod を選択します。
- `--tty` 指定時は stderr は stdout にマージされます（TTY の仕様）。
- `--escape` で指定したシーケンスを送るとセッションを切断して終了できます（例: `~.`）。

例:

```
# デプロイ中のアプリ Pod でシェルを開く（対話）
kompoxops app exec -A app1 -it -- bash

# コンテナ名を指定してログを確認
kompoxops app exec -A app1 -t -c app -- sh -c 'tail -n 100 /var/log/app.log'
```

### kompoxops disk

app.volumes で定義された論理ボリュームに属するディスク（ボリュームインスタンス）を操作する。

```
kompoxops disk list   --app-name <appName> --vol-name <volName>                     ディスク一覧表示
kompoxops disk create --app-name <appName> --vol-name <volName>                     新しいディスク作成 (サイズは app.volumes 定義を使用)
kompoxops disk assign --app-name <appName> --vol-name <volName> --disk-name <name>  指定ディスクを <volName> の Assigned に設定 (他は自動的に Unassign)
kompoxops disk delete --app-name <appName> --vol-name <volName> --disk-name <name>  指定ディスク削除 (Assigned 中はエラー)
```

共通オプション

- `--app-name | -A` アプリ名を指定 (デフォルト: kompoxops.yml の app.naame)
- `--vol-name | -V` ボリューム名を指定
- `--disk-name | -D` ディスク名を指定

仕様

- `<volName>` は app.volumes に存在しない場合エラー。
- create: ディスク名は自動生成 (例: 時刻ベース) または `--name` 指定 (存在重複はエラー)。
- assign: 1 論理ボリュームにつき同時に Assigned=true は 0 または 1。既に同一ディスクが Assigned なら成功 (冪等)。別ディスクが Assigned の場合は自動でそのディスクを Unassign 後に指定を Assign。
- delete: 対象が存在しなければ成功 (冪等)。Assigned=true のディスクは `--force` 無しで拒否。
- list 出力列例: NAME  ASSIGNED  SIZE  HANDLE(SHORT)  CREATED              UPDATED
- SIZE 表示は Gi 単位 (内部は bytes)。
- manifest 生成 (app deploy) 時: 各 volName で Assigned インスタンスがちょうど 1 件でない場合エラー。

例

```
$ kompoxops disk list default
NAME        ASSIGNED  SIZE   HANDLE        CREATED              UPDATED
vol-202401  true      32Gi   1f3ab29 (az)  2024-01-10T12:00Z    2024-01-10T12:05Z
vol-202312  false     32Gi   9ab1c02 (az)  2023-12-31T09:00Z    2024-01-10T12:05Z
```

#### kompoxops disk list

ボリュームインスタンスの一覧を表示します。

#### kompoxops disk create

新しいボリュームインスタンスを作成します（サイズは app.volumes 定義）。

#### kompoxops disk assign

指定インスタンスを Assigned=true に設定し、他を自動的に Unassign します。

#### kompoxops disk delete

指定インスタンスを削除します（Assigned 中は `--force` なしで拒否）。

### kompoxops snapshot

app.volumes で定義された論理ボリュームに属するスナップショットを操作する。

```
kompoxops snapshot list    --app-name <appName> --vol-name <volName>                       スナップショット一覧表示
kompoxops snapshot create  --app-name <appName> --vol-name <volName> --disk-name <disk>    指定ディスクからスナップショット作成
kompoxops snapshot delete  --app-name <appName> --vol-name <volName> --snapshot-name <snap>指定スナップショットを削除 (NotFoundは成功)
kompoxops snapshot restore --app-name <appName> --vol-name <volName> --snapshot-name <snap>スナップショットから新規ディスクを作成
```

共通オプション

- `--app-name | -A` アプリ名を指定 (デフォルト: kompoxops.yml の app.name)
- `--vol-name | -V` ボリューム名を指定

サブコマンドごとの必須オプション

- `create`: `--disk-name | -D` 作成元ディスク名を指定
- `delete`/`restore`: `--snapshot-name | -S` 対象スナップショット名を指定

仕様

- list: `CreatedAt` の降順で返す。出力は JSON 配列。
- create: 指定ディスクからクラウドネイティブのスナップショットを作成し、JSON で 1 件返す。
- delete: 存在しない場合も成功（冪等）。
- restore: 指定スナップショットから新しいディスクを作成し、JSON で 1 件返す。復元ディスクは `Assigned=false`。切替は `kompoxops disk assign` で行う。

使用例

```
# 一覧
kompoxops snapshot list -A app1 -V db

# ディスク db の現在のアクティブインスタンスから作成（例: 名前が ULID）
kompoxops snapshot create -A app1 -V db -D 01J8WXYZABCDEF1234567890GH

# 復元して新規ディスクを作る
kompoxops snapshot restore -A app1 -V db -S 01J8WXYZABCDEF1234567890JK

# スナップショット削除
kompoxops snapshot delete -A app1 -V db -S 01J8WXYZABCDEF1234567890JK
```

#### kompoxops snapshot list

スナップショットの一覧を表示します。

#### kompoxops snapshot create

指定ディスクからスナップショットを作成します。

#### kompoxops snapshot delete

指定スナップショットを削除します（NotFound の場合も成功）。

#### kompoxops snapshot restore

指定スナップショットから新しいボリュームインスタンスを作成します（復元ディスクは Assigned=false）。

### kompoxops admin

### kompoxops tool

アプリの Namespace に「メンテナンス用ランナー」(Deployment/Pod) をデプロイ・操作します。PV/PVC をアプリ定義のボリュームにバインドしてマウントでき、バックアップやメンテ作業、対話的なシェル実行に利用します。

```
kompoxops tool deploy   --app-name <appName> [--image IMG] [-V vol:disk:/path]... [-c CMD]... [-a ARG]...
kompoxops tool destroy  --app-name <appName>
kompoxops tool status   --app-name <appName>
kompoxops tool exec     --app-name <appName> -- <command> [args...]
```

共通オプション

- `--app-name | -A` 対象アプリ名 (デフォルト: `kompoxops.yml` の `app.name`)

注意

- ランナーはアプリの Namespace に `kompox.dev/tool-runner=true` ラベル付きでデプロイされます。名前は固定で `tool-runner` です。
- PV/PVC は必要に応じて生成されますが、`destroy` は Deployment/Pod のみ削除します（PV/PVC は保持）。

#### kompoxops tool deploy

メンテナンス用ランナーをデプロイします（冪等）。アプリ定義のボリュームと既存ディスクを指定してマウントできます。

主なオプション:

- `--image IMG` ランナーのコンテナイメージ（既定: `busybox`）
- `-V, --volume volName:diskName:/mount/path` マウント指定を繰り返し指定可能
  - `volName` は `app.volumes` のボリューム名
  - `diskName` はそのボリューム配下の既存ディスク名（例: `kompoxops disk list` で確認）
  - `/mount/path` はコンテナ内の絶対パス
- `-c, --command TOKEN` エントリポイント（複数指定でトークン分割、image の ENTRYPOINT を上書き）
- `-a, --args TOKEN` 引数（複数指定でトークン分割、image の CMD を上書き）

コマンド/引数の既定動作:

- `--command` も `--args` も未指定: `sh -c` として `sleep infinity` を実行し待機
- `--args` のみ指定: `sh -c` をエントリポイントにし、与えた引数をそのままシェルに渡す
- `--command` のみ指定: 与えたエントリポイントのみを使用（引数なし）

例:

```
# ボリューム db のディスク vol-202401 を /var/lib/postgresql にマウントし、シェルで待機
kompoxops tool deploy -A app1 -V db:vol-202401:/var/lib/postgresql --image debian:stable

# 1回のジョブ的にコマンドを流して終了（args のみ指定）
kompoxops tool deploy -A app1 -V default:vol-01ABCD:/data -a "tar czf /data.bak.tgz /data"
```

#### kompoxops tool destroy

ランナーの Deployment/Pod を削除します（冪等）。PV/PVC は保持されます。

#### kompoxops tool status

ランナーの状態を JSON で表示します。`ready`、`namespace`、`name`、`node_name`、`image`、`command`、`args` を返します。

出力例

```json
{
  "ready": true,
  "namespace": "kompox-app1-01HXYZ...",
  "name": "tool-runner",
  "node_name": "aks-nodepool1-12345678-vmss000001",
  "image": "debian:stable",
  "command": ["sh", "-c"],
  "args": ["sleep infinity"]
}
```

#### kompoxops tool exec

稼働中のランナーポッド内でコマンドを実行します。対話モードにも対応します。

使用法:

```
kompoxops tool exec -A <appName> [-i] [-t] [-e ESC] -- <command> [args...]
```

オプション:

- `-i, --stdin` 標準入力を接続
- `-t, --tty` TTY を割り当て（bash 等の対話時に推奨）
- `-e, --escape` デタッチ用エスケープシーケンス（既定: `~.`、`none` で無効化）

例:

```
# 付与したマウントを確認
kompoxops tool exec -A app1 -it -- sh -c 'df -hT; mount | grep pvc'

# 進行中の操作からデタッチ（~. を入力）
kompoxops tool exec -A app1 -it -- bash
```

管理用途 CLI を提供する。
一般ユーザー向け REST API とは異なり認証認可を無視した OOB の接続により CRUD 操作を直接呼び出すことができる。

```
kompoxops admin [--db-url <URL>] <KIND> <VERB> <options...>
kompoxops admin service list
kompoxops admin service get svc-a
kompoxops admin service create -f svc-a.yml
kompoxops admin service update svc-a -f svc-a.yml
kompoxops admin service delete svc-a
```
