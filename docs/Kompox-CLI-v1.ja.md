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
kompoxops volume            ボリューム操作
kompoxops snapshot          スナップショット操作
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
kompoxops app deploy --app-name <appName>
kompoxops app destroy --app-name <appName>
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

補足: PV/PVC を削除しても、StorageClass/PV の ReclaimPolicy が Retain の場合はクラウドディスク本体は保持されます。

### kompoxops volume

app.volumes で定義された論理ボリュームに属するボリュームインスタンスを操作する。

```
kompoxops volume list --app-name <appName> --vol-name <volName>                                      ボリュームインスタンス一覧表示
kompoxops volume create --app-name <appName> --vol-name <volName>                                    新しいボリュームインスタンス作成 (サイズは app.volumes 定義を使用)
kompoxops volume assign --app-name <appName> --vol-name <volName> --vol-inst-name <volInstanceName>  指定インスタンスを <volName> の Assigned に設定 (他は自動的に Unassign)
kompoxops volume delete --app-name <appName> --vol-name <volName> --vol-inst-name <volInstanceName>  指定インスタンス削除 (Assigned 中はエラー)
```

共通オプション

- `--app-name | -A` アプリ名を指定 (デフォルト: kompoxops.yml の app.naame)
- `--vol-name | -V` ボリューム名を指定
- `--vol-inst-name | -I` ボリュームインスタンス名を指定

仕様

- `<volName>` は app.volumes に存在しない場合エラー。
- create: インスタンス名は自動生成 (例: 時刻ベース) または `--name` 指定 (存在重複はエラー)。
- assign: 1 論理ボリュームにつき同時に Assigned=true は 0 または 1。既に同一インスタンスが Assigned なら成功 (冪等)。別インスタンスが Assigned の場合は自動でそのインスタンスを Unassign 後に指定を Assign。
- delete: 対象が存在しなければ成功 (冪等)。Assigned=true のインスタンスは `--force` 無しで拒否。
- list 出力列例: NAME  ASSIGNED  SIZE  HANDLE(SHORT)  CREATED              UPDATED
- SIZE 表示は Gi 単位 (内部は bytes)。
- manifest 生成 (app deploy) 時: 各 volName で Assigned インスタンスがちょうど 1 件でない場合エラー。

例

```
$ kompoxops volume list default
NAME        ASSIGNED  SIZE   HANDLE        CREATED              UPDATED
vol-202401  true      32Gi   1f3ab29 (az)  2024-01-10T12:00Z    2024-01-10T12:05Z
vol-202312  false     32Gi   9ab1c02 (az)  2023-12-31T09:00Z    2024-01-10T12:05Z
```

#### kompoxops volume list

ボリュームインスタンスの一覧を表示します。

#### kompoxops volume create

新しいボリュームインスタンスを作成します（サイズは app.volumes 定義）。

#### kompoxops volume assign

指定インスタンスを Assigned=true に設定し、他を自動的に Unassign します。

#### kompoxops volume delete

指定インスタンスを削除します（Assigned 中は `--force` なしで拒否）。

### kompoxops admin

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
