---
title: Kompox 仕様ドラフト
version: v1
status: archived
updated: 2025-09-26
language: ja
---

# Kompox 仕様ドラフト

## 概要

Kubernetesを使って、RedmineやGiteaなどのコンテナWebアプリを低コストでホスティングできる、社内向けのマルチテナントコンテナアプリホスティングPaaSを作りたい。

- 各テナントの持つデータの分離と保全を重視する。
- 社内向けPaaSなので絶対的な可用性はあまり重視しない。
- 社内向けではあるがPaaS APIもWebアプリもパブリックアクセスのみのサービスとする。
- PaaSユーザーはホスティングされるWebアプリのDNSホスト名とコンテナイメージを提供する。
- イングレスを共通化しサイトごとに割り当てられたDNSホスト名を用いてサービスを提供するPodにルーティングする。
- アプリへのアクセスは社内関係者ユーザーからのみで少数なので、各サイトはシングルレプリカのPodで稼働する。Pod内にはWebサーバーやDBサーバーのコンテナが含まれる。
- 各サイトには固有のデータボリュームと呼ばれるブロックストレージPVが用意される。データボリュームはアプリコンテナ内にマウントされ、DBやアップロードファイルなどのアプリ固有データはすべてをこのボリュームに保存する。
- 各サイトのデータボリュームはKubernetesクラスタとは異なるライフサイクルのクラウドネイティブなストレージリソースで管理し、継続的なスナップショット取得によるバックアップと復元能力を実現する。
- KubernetesクラスタにはPVCでデータボリュームリソースを参照してアタッチする。クラスタで障害が発生したときはデータボリュームをデタッチし、別のクラスタにアタッチしてサービスを継続できるようにする。
- AzureのAKSを基本として、Oracle Cloud、Google Cloud、AWSなどの主要マネージドクラスタに対応する。またVMにインストールされたセルフホストのK3sクラスタもサポートする。

## Kompox

このPaaSを実装するソフトウェアの名称をKompoxとする。

- Kompoxという名称はKomposeから派生したソフトであることを示す。
- 作者はLinux VM上でのDocker Composeによる開発体験をリスペクトしており、それをKubernetesで再現する目的がある。
- KompoxではPodをシングルレプリカに限定し、永続化ボリュームにRWOストレージを採用することで、仮想マシンに近い環境をKubernetesで実現している。

## Kompox コマンド

次のようなコマンドを実装する。

|コマンド名|説明|
|-|-|
|kompoxops|Kompox 仕様のクラウドリソースデプロイ・運用ツール<br>Kompox PaaS とは独立した設定ファイル `kompoxops.yml` を読み取って動作する CLI|
|kompoxsvc|Kompox PaaS REST API サーバと管理ツール<br>REST API サーバは `kompoxsvc server` で起動するコンテナWebアプリ|
|kompox|Kompox PaaS REST API クライアント CLI|

## kompoxops 仕様

コマンドライン仕様

```
kompoxops init ... kompoxops.yml の雛形作成
kompoxops cluster deploy ... traefik ingress controller をデプロイする
kompoxops app validate ... compose.yml のバリデーションとK8sマニフェスト出力
kompoxops app deploy ... compose.yml のデプロイ
kompoxops app destroy ... デプロイの削除 (diskは残る)
kompoxops disk list ... ディスクリソースの一覧
kompoxops disk attach ... ディスクリソースの差し替え
kompoxops disk import ... ディスクリソースのインポート
kompoxops disk export ... ディスクリソースのエクスポート
kompoxops disk delete ... ディスクリソースの削除
kompoxops snapshot list ... スナップショットリソースの一覧
kompoxops snapshot create ... スナップショットリソースの作成
kompoxops snapshot restore ... スナップショットリソースの復元
kompoxops snapshot export ... スナップショットリソースのエクスポート
kompoxops snapshot delete ... スナップショットリソースの削除
```

設定ファイル `kompoxops.yml` 仕様 (`cluster.settings` および `app.settings` で設定すべき内容は `provider.driver` により異なる)

```yaml
version: 1
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
  auth:
    type: kubectl
    kubeconfig: ~/.kube/config
    context: my-aks
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


上記の設定では次のようなルーティングのアノテーションを持つ Ingress が作成される。

|URL|宛先ポート|説明|
|-|:-:|-|
|`https://my-app.my-aks.ops.kompox.dev`|80|自動作成・ポート80は `{app.name}` を使用|
|`https://myy-app-8080.my-aks.ops.kompox.dev`|8080|自動作成・ポート80以外では `{app.name}-ポート番号` を使用|
|`https://www.my-app.kompox.dev`|80|カスタムDNS|
|`https://admin.my-app.kompox.dev`|8080|カスタムDNS|

基本的なステートは K8s の Namespace リソースのアノテーションで保持する。

```yaml
kompox.dev/app: ops/aks1/my-aks/my-app
kompox.dev/provider: aks
kompox.dev/disk-current-id: /subscriptions/....
kompox.dev/disk-previous-id: /subscriptions/....
```

disk や snapshot の列挙は、クラウドリソースのタグにより識別する。

```yaml
kompox-app: ops/aks1/my-aks/my-app
kompox-created-at: 2025-08-11T12:34:56Z
```

## Kompox PaaS (kompoxsvc/kompox) 実装仕様要件

Kompox PaaS REST API リソースモデル

```go
// サービス (シングルトンリソース)
type Service struct {
  Name string // DNSホスト名の制約
}

// プロバイダ
type Provider struct {
  Name string
  Service string             // Service.Name の参照
  Driver string              // aks, k3s, oke, etc.
  Settings map[string]string // Provider固有の設定値
}

// Kubernetesクラスタ (Serviceに所属する)
type Cluster struct {
  Name string                // DNSホスト名の制約
  Provider string            // Provider.Name の参照
  Auth ClusterAuth
  Ingress ClusterIngress
  Domain string              // デフォルトDNSドメイン
  Settings map[string]string // Cluster固有の設定値
}

type ClusterAuth struct {
  Type string       // kubectl, etc.
  Kubeconfig string // "~/.kube/config"
  Context string    // "my-aks"
}

type ClusterIngress struct {
  Controller string // "traefik"
  Namespace string  // "traefik"
}

// アプリケーション (Clusterに所属する)
type App struct {
  Name string                 // DNSホスト名の制約
  Cluster string              // Cluster.Nameの参照
  Compose string              // Docker Compose 設定テキスト
  Ingress map[string]string   // カスタムDNSホスト名 Ingress 設定
  Resources map[string]string // Podリソース設定 (cpu, memory, etc.)
  Settings map[string]string  // App固有の設定値
}          
```

Kompox PaaS REST API 実装仕様

- kompoxsvcはコンテナWebアプリとして実装するが、具体的なホストサービスはターゲットインフラの種類ごとに異なる。
  - コントロールプレーンAPIを提供するサーバ(kompoxsvc)と、それにアクセスするクライアントCLI(kompox)をGo言語で実装する。  
  - Azureの場合はAzure Cotainer AppsとAzure Database for MySQL flexible serverでホストする。
  - シングルノードK3sの場合はそのK3sホストで稼働するprivilegedなコンテナWebアプリとしてsqlite3でホストする。
- Service
  - 管理者が設定するシングルトンのリソース
  - NameはRFC1123準拠のDNSラベル名
- Provider
  - 管理者が設定するクラスタプロバイダのリソース
  - NameはRFC1123準拠のDNSラベル名
  - Serviceに所属する
  - Driverでクラスタの種類を指定する aks,k3s,oke など
  - SettingsでProvider固有の設定を指定する
- Cluster
  - 管理者が設定するKubernetesクラスタのリソース
  - NameはRFC1123準拠のDNSラベル名
  - Providerに所属する
  - Authでクラスタ接続方法を指定する
  - IngressでTraefik Proxyのインストール方法を指定する
  - SettingsでCluster固有の設定を指定する
    - DEFULAT_DOMAIN は "kompox-apps.com" のようなデフォルトドメイン階層のFQDN
- App
  - ユーザーが所有するアプリのリソース
  - NameはRFC1123準拠のDNSラベル名
  - Clusterに所属する
  - ComposeにDocker Compose設定を格納する
  - IngressでカスタムDNSホスト名のIngress設定を指定する    
  - ResourcesでPodの割り当てを指定する  
  - SettingsでCluster.Provider固有の設定を指定する  
- 各 Cluster では Traefik Proxy をイングレスコントローラとしてデプロイする。
- 各 Cluster ごとにデフォルト DNS として \*.{cluster.ingress.domain} のワイルドカードSSL証明書を Traefik Proxy に設定する。 証明書の取得・保持方法は Provider により異なる。
- ユーザーは App を作成・所有・デプロイできる。
- ユーザーが App をデプロイすることで次のようなことが起きる。
  - App.Compose が Kompose によって Kubernetes マニフェストに変換され App.Cluster で指定したクラスタに適用される。
  - App.Composeではボリュームを1つだけ参照でき、その実態としてディスクのクラウドリソースを割り当てるPVCが自動的に作成される。
  - Compose の ports でエクスポーズされた各ポート指定に対応する ServiceリソースとIngressリソースが自動的に作成される。IngresリソースにはTraefik Proxyが読み取るアノテーションが設定され、DNSバーチャルホストによる各サービスへのアクセスが可能になる。
  - エクスポーズされた各ポートには固有のDNSホスト名が自動的に割り当てられる。
  - {App.Name}.{cluster.ingress.domain} → ポート80
  - {App.Name}-8080.{cluster.ingress.domain} → ポート8080
  - エクスポーズされた各ポートには次のような App.Ingress によりカスタムDNSホスト名が割り当てられる。
      http_80: www.custom-apps.com
      http_8080: admin.custom-apps.com
- ユーザーはClusterのパブリックFQDNとIPアドレスを取得できる。カスタムDNSホストのCNAMEやAレコードにこれを設定することはユーザーの責任とする。Traefik ProxyはLet's EncryptのTLS-ALPN-01チャレンジによりSSL証明書を自動取得する。
- ユーザーはAppに属するディスクのリスト、インポート、エクスポート、削除、切り替えができる。
- ユーザーはAppに属するスナップショットのリスト、作成、復元、エクスポート、削除ができる。
- Appに属するディスクやスナップショットなどのクラウドリソースの列挙の方法は Provider/Cluster/App に依存する。通常は "{Service.Name}/{Provider.Name}/{Cluster.Name}/{App.Name}" という値を持つタグをリソースに設定することで識別する。    
- ユーザーは所有する App の K8s ネームスペースに限定した権限を持つ K8s API トークンが得られる。これによりユーザーはコンテナの稼働状況やログを取得でき、コマンド実行やシェル接続もできる。リソースの更新や削除は許可しない。
- AKS においては OIDC 連携に対応する。指定した Entra ID ユーザー・サービスプリンシパルは上記の権限を持つK8s API トークンが得られる。
