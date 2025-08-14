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
kompoxops admin             管理ツール
```

グローバルオプション

```
--db-url <URL>              永続化DBの接続URL。環境変数 KOMPOX_DB_URL で指定可能。
```

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
  name: my-aks
  existing: false
  ingress:
    controller: traefik  
    namespace: traefik
  domain: ops.kompox.dev
  settings:
    AZURE_RESOURCE_GROUP_NAME: rg-CLU
app:
  name: my-app
  compose: compose.yml
  ingress:
    http_80: www.my-app.kompox.dev
    http_8080: admin.my-app.kompox.dev     
  resources:
    cpu: 500m
    memory: 1Gi
  settings:
    AZURE_RESOURCE_GROUP_NAME: rg-APP
    AZURE_DISK_SIZE: 50
    AZURE_DISK_TYPE: Standard_LRS
```

### kompoxops cluster

K8s クラスタ操作を行う。

```
kompoxops cluster provision     Cluster リソース準拠の K8s クラスタを作成開始 (existingがfalseの場合)
kompoxops cluster deprovision   Cluster リソース準拠の K8s クラスタを削除開始 (existingがfalseの場合)
kompoxops cluster install       K8s クラスタ内のリソースをインストール開始
kompoxops cluster uninstall     K8s クラスタ内のリソースをアンインストール開始
kompoxops cluster status        K8s クラスタのステータスを表示
```

引数として Cluster リソースの名前を指定する。名前のデフォルトは cluster.name とする。

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
