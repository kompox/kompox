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
    AZURE_TOKEN_CREDENTIALS: dev
    AZURE_TENANT_ID: xxxxxx
    AZURE_SUBSCRIPTION_ID: xxxxxx
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
kompoxops cluster create        クラスタを作成する
kompoxops cluster destroy       クラスタを削除する
kompoxops cluster provision     クラスタ内のリソースを作成する
kompoxops cluster deprovision   クラスタ内のリソースを削除する
```

create コマンドは service/provider/cluster の settings を使用してクラスタを作成する。

destroy コマンドは作成したクラスタを削除する。

provision コマンドはクラスタ内のリソースを作成する。
Traefik Proxy などの Ingress Controller を含む。

deprovision コマンドは provision コマンドで作成されたクラスタ内のリソースを削除する。

既存のクラスタを参照する場合は cluster.existing を true に設定する。
cluster.existing が true の場合 create/destroy は何もしない。
既存のクラスタの接続方法は cluster.settings で設定する。
接続方法の設定内容は provider.driver により異なる。

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
