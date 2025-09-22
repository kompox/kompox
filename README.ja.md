# Kompox (K4x)

## 概要

**Docker Compose で書かれたステートフルアプリを、クラウドのマネージド Kubernetes 上でシームレスに動かすことができるオーケストレーションツール**

Kompox は [Kompose](https://kompose.io) のアイディアを拡張し、ステートフルワークロードの本番運用における課題を解決します。

K4x は Kompox の短縮形です。

## ステータスとロードマップ

**2025年9月時点で本プロジェクトはアルファバージョンの段階です。CLIや内部APIは将来的に破壊的変更が行われる可能性があります。**

Kompox はマルチクラウドに対応した設計となっていますが、当面は Microsoft Azure をターゲットに個別機能の POC を進めています。

完了

- [x] `kompoxops` CLI 基本実装
  - [x] `kompoxops.yml` 設定ファイルの定義
- [x] Kubernetes 基本実装
  - [x] Docker Compose → Kubernetes Manifest 変換
    - [x] `services` → 単一Pod・複数コンテナマッピング
    - [x] `volumes` → RWO PV/PVC マッピング
    - [x] `ports` → Service/Ingress マッピング
    - [x] `env_file` → Secret マッピング
  - [x] Ingress Controller
    - [x] Helm SDK による Traefik インストール
    - [x] Let's Encrypt TLS 証明書発行 (ALPN-TLS-01)
  - [x] App コンポーネント (アプリコンテナ)
    - [x] 独立ネームスペース作成とネットワーク隔離
    - [x] コンテナログ表示とシェル接続
  - [x] Kompox Box コンポーネント (開発・テスト・メンテナンスコンテナ)
    - [x] RWO PVマウント
    - [x] コンテナ SSH 接続
    - [x] コンテナ SCP ファイル転送
    - [x] コンテナ Rsync ファイル転送
- [x] AKS (Azure) Driver 基本実装
  - [x] AKS クラスタライフサイクル管理
  - [x] 管理ディスク・スナップショットライフサイクル管理
  - [x] 可用性ゾーン対応
  - [x] スナップショットによる RWO ディスクの復元とフェイルオーバー
  - [x] Traefik Static TLS 証明書 (Azure Key Vault CSI driver)

計画中

- [ ] K3s (selfhosted) Driver
- [ ] OKE (OCI) Driver
- [ ] EKS (AWS) Driver
- [ ] GKE (Google) Driver
- [ ] GitOps 対応
- [ ] PaaS サーバ実装
- [ ] クラウドサービス・K8sワークロードのビリング

## 発表・登壇

- **2025/09/25(木)** [**Kubernetes Novice Tokyo #38**](https://k8s-novice-jp.connpass.com/event/365526/)
  - Kompox: クラウドネイティブコンテナ Web アプリ DevOps ツールの紹介
- **2025/11/18(火)** [**CloudNative Days Winter 2025**](https://event.cloudnativedays.jp/cndw2025) (予定)
  - [Kompox: ステートフルワークロード運用がスケールする未来をつくる](https://event.cloudnativedays.jp/cndw2025/proposals/1000)
  - 採択前のプロポーザルです。興味を持たれた方はぜひ投票をお願いします

## Kompox の背景にある課題

Kubernetesはステートレスアプリの運用基盤として普及しましたが、ステートフルなワークロードの運用は依然として複雑です。

- **ストレージの制約:** 性能要件からReadWriteOnce (RWO) の永続ボリューム (PV) が必要になるケースが多くありますが、RWOボリュームは単一ノードにしか接続できず、障害時の復旧や移行が困難になる一因となっています。
- **クラウド間の差異:** 各クラウド (Azure, AWS, Google, OCI など) が提供するマネージドKubernetesやストレージサービスには微妙な仕様の違いがあり、アプリケーションのポータビリティを妨げます。
- **開発と本番のギャップ:** Docker Compose の `compose.yml` を使ったローカルでの手軽な開発体験を、そのまま本番のKubernetes環境に持ち込むことは困難です。

Kompoxはこれらの課題を解決し、ステートフルワークロードの運用をスケールさせることを目指します。

- **シンプルな定義ファイル:** `compose.yml` の資産を活かしつつ、`kompoxops.yml` というシンプルなファイルでアプリケーションとインフラを定義できます。
- **クラウド差異の吸収:** Provider Driverアーキテクチャにより、AKS (Azure), EKS (AWS), GKE (Google), OKE (OCI) といった各クラウドにおける RWO ボリュームやスナップショット機能の差異を吸収します。
- **容易なデータ管理:** クラウドネイティブなスナップショット機能を活用し、バックアップ・リストアや、障害発生時のアベイラビリティゾーン (AZ)・リージョン・クラウド間マイグレーションを容易にします。
- **一貫した操作性:** `kompoxops` CLIツールを通じて、ローカル開発環境から本番クラウド環境まで、一貫したコマンドでアプリケーションのデプロイや管理を行えます。

## Kompox の使用例

例えば次のような [Gitea](https://about.gitea.com/) を動かすための [compose.yml](./tests/aks-e2e-gitea/compose.yml) ファイルがあるとします。

```yaml
services:
  gitea:
    image: docker.gitea.com/gitea:1.24.6
    environment:
      - USER_UID=1000
      - USER_GID=1000
    env_file:
      - compose-gitea.env
    volumes:
      - ./data/gitea:/data
    ports:
      - "3000:3000"
  postgres:
    image: postgres:17
    env_file:
      - compose-postgres.env
    volumes:
      - ./data/postgres:/var/lib/postgresql/data
```

これは Docker Compose を使ってローカルの開発環境で普通にテストすることができます。

```bash
$ docker compose up -d
[+] Running 3/3
 ✔ Network aks-e2e-gitea_default       Created        0.1s
 ✔ Container aks-e2e-gitea-postgres-1  Started        0.3s
 ✔ Container aks-e2e-gitea-gitea-1     Started        0.3d
```

Azure Kubernetes Service (AKS) 向けの設定ファイル [kompoxops.yml](./tests/aks-e2e-gitea/kompoxops.yml.in) を用意し `kompoxops` CLI ツールを使うことで、次のことができます。

- AKS クラスタをプロビジョン (認証は Azure CLI によるものを使用)
- クラスタに Ingress Controller (traefik) や共通 Kubernetes リソースをインストール
- RWO PV の実体となる Azure 管理ディスク (Premium SSD v2, 10GiB) を作成
- compose.yml から変換した Kubernetes Manifest をデプロイしてアプリを公開

```yaml
version: v1
service:
  name: aks-e2e-gitea-4-20250921-103056
provider:
  name: aks1
  driver: aks
  settings:
    AZURE_AUTH_METHOD: azure_cli
    AZURE_SUBSCRIPTION_ID: 34809bd3-31b4-4331-9376-49a32a9616f2
    AZURE_LOCATION: eastus
cluster:
  name: cluster1
  existing: false
  ingress:
    certEmail: yaegashi@live.jp
    certResolver: staging
    domain: app-default.kompox.dev
  settings:
    AZURE_AKS_SYSTEM_VM_SIZE: Standard_D2ds_v4
    AZURE_AKS_SYSTEM_VM_DISK_TYPE: Ephemeral
    AZURE_AKS_SYSTEM_VM_DISK_SIZE_GB: 64
    AZURE_AKS_SYSTEM_VM_PRIORITY: Regular
    AZURE_AKS_SYSTEM_VM_ZONES:
    AZURE_AKS_USER_VM_SIZE: Standard_D2ds_v4
    AZURE_AKS_USER_VM_DISK_TYPE: Ephemeral
    AZURE_AKS_USER_VM_DISK_SIZE_GB: 64
    AZURE_AKS_USER_VM_PRIORITY: Regular
    AZURE_AKS_USER_VM_ZONES: 1
app:
  name: app1
  compose: file:compose.yml
  ingress:
    certResolver: staging
    rules:
      - name: main
        port: 3000
        hosts: [gitea.yb2.banadev.org]
  deployment:
    zone: "1"
  volumes:
    - name: default
      size: 10Gi
      options:
        sku: PremiumV2_LRS
```

実行例

```bash
# AKS クラスタをプロビジョン
$ kompoxops cluster provision
2025/09/21 23:21:12 INFO provision start cluster=cluster1
2025/09/21 23:21:12 INFO aks cluster provision begin subscription=34809bd3-31b4-4331-9376-49a32a9616f2 resource_group=k4x-485t8r_cls_cluster1_3mc4wy tags="map[kompox-cluster-hash:3mc4wy kompox-cluster-name:cluster1 kompox-provider-name:aks1 kompox-service-name:aks-e2e-gitea-4-20250921-103056 managed-by:kompox]"
2025/09/21 23:27:47 INFO aks cluster provision succeeded subscription=34809bd3-31b4-4331-9376-49a32a9616f2 resource_group=k4x-485t8r_cls_cluster1_3mc4wy
2025/09/21 23:27:47 INFO provision success cluster=cluster1

# クラスタに Ingress Controller (traefik) 他をインストール
$ kompoxops cluster install
2025/09/21 23:27:52 INFO install start cluster=cluster1
2025/09/21 23:27:52 INFO aks cluster install begin cluster=cluster1 provider=aks1
2025/09/21 23:27:59 INFO applying kind=ConfigMap name=traefik namespace=traefik force=false
2025/09/21 23:29:48 INFO aks cluster install succeeded cluster=cluster1 provider=aks1
2025/09/21 23:29:48 INFO install success cluster=cluster1

# クラスタの状態を表示
$ kompoxops cluster status
{
  "existing": false,
  "provisioned": true,
  "installed": true,
  "ingressGlobalIP": "172.212.3.21",
  "cluster_id": "94d5a2990ca1680a",
  "cluster_name": "cluster1"
}

# RWO PV 実体の Azure 管理ディスクを作成 (Premium SSD v2, 10GiB)
$ kompoxops disk create -V default
2025/09/21 23:47:13 INFO create volume instance start app=app1 volume=default
2025/09/21 23:47:15 INFO ensuring resource group subscription=34809bd3-31b4-4331-9376-49a32a9616f2 location=eastus resource_group=k4x-485t8r_app_app1_3dktww tags="map[kompox-app-id-hash:3dktww kompox-app-name:app1 kompox-provider-name:aks1 kompox-service-name:aks-e2e-gitea-4-20250921-103056 managed-by:kompox]"
2025/09/21 23:47:20 INFO ensuring role assignment scope=/subscriptions/34809bd3-31b4-4331-9376-49a32a9616f2/resourceGroups/k4x-485t8r_app_app1_3dktww principal_id=ec88a478-3be1-4cf1-9337-795c20aa2f25 role_definition_id=b24988ac-6180-42a0-ab88-20f7382dd24c
{
  "name": "0t2yq311rtm9",
  "volumeName": "default",
  "assigned": true,
  "size": 10737418240,
  "zone": "1",
  "options": {
    "iops": 3000,
    "mbps": 125,
    "sku": "PremiumV2_LRS"
  },
  "handle": "/subscriptions/34809bd3-31b4-4331-9376-49a32a9616f2/resourceGroups/k4x-485t8r_app_app1_3dktww/providers/Microsoft.Compute/disks/k4x-485t8r_disk_default_0t2yq311rtm9_3dktww",
  "createdAt": "2025-09-21T23:47:28.1200759Z",
  "updatedAt": "2025-09-21T23:47:28.1200759Z"
}

# クラスタに compose.yml から変換した Kubernetes Manifest をデプロイ
$ kompoxops app deploy
2025/09/21 23:48:31 INFO applying kind=Namespace name=k4x-485t8r-app1-3dktww namespace="" force=true
2025/09/21 23:48:32 INFO applying kind=ServiceAccount name=app1 namespace=k4x-485t8r-app1-3dktww force=true
2025/09/21 23:48:32 INFO applying kind=NetworkPolicy name=app1 namespace=k4x-485t8r-app1-3dktww force=true
2025/09/21 23:48:32 INFO applying kind=PersistentVolume name=k4x-485t8r-default-3dktww-5e0brq namespace="" force=true
2025/09/21 23:48:32 INFO applying kind=PersistentVolumeClaim name=k4x-485t8r-default-3dktww-5e0brq namespace=k4x-485t8r-app1-3dktww force=true
2025/09/21 23:48:33 INFO applying kind=Secret name=app1-app-gitea namespace=k4x-485t8r-app1-3dktww force=true
2025/09/21 23:48:33 INFO applying kind=Secret name=app1-app-postgres namespace=k4x-485t8r-app1-3dktww force=true
2025/09/21 23:48:33 INFO applying kind=Deployment name=app1-app namespace=k4x-485t8r-app1-3dktww force=true
2025/09/21 23:48:33 INFO applying kind=Service name=app1-app namespace=k4x-485t8r-app1-3dktww force=true
2025/09/21 23:48:34 INFO applying kind=Service name=gitea namespace=k4x-485t8r-app1-3dktww force=true
2025/09/21 23:48:34 INFO applying kind=Service name=postgres namespace=k4x-485t8r-app1-3dktww force=true
2025/09/21 23:48:34 INFO applying kind=Ingress name=app1-app-default namespace=k4x-485t8r-app1-3dktww force=true
2025/09/21 23:48:35 INFO applying kind=Ingress name=app1-app-custom namespace=k4x-485t8r-app1-3dktww force=true
2025/09/21 23:48:35 INFO deploy success app=app1

# クラスタにデプロイされたアプリの状態を表示
$ kompoxops app status
{
  "app_id": "646a9de0f6b93fee",
  "app_name": "app1",
  "cluster_id": "41bb23e8131f3f0e",
  "cluster_name": "cluster1",
  "ready": true,
  "image": "docker.gitea.com/gitea:1.24.6",
  "namespace": "k4x-485t8r-app1-3dktww",
  "node": "aks-npuser1-29605489-vmss000000",
  "deployment": "app1-app",
  "pod": "app1-app-7f5df44875-5smj7",
  "container": "gitea",
  "command": null,
  "args": null,
  "ingress_hosts": [
    "app1-3dktww-3000.app-default.kompox.dev",
    "gitea.yb2.banadev.org"
  ]
}

# アプリコンテナのログ表示
$ kompoxops app logs -c gitea
Generating /data/ssh/ssh_host_ed25519_key...
Generating /data/ssh/ssh_host_rsa_key...
Generating /data/ssh/ssh_host_ecdsa_key...
Server listening on :: port 22.
Server listening on 0.0.0.0 port 22.
2025/09/21 23:48:58 cmd/web.go:261:runWeb() [I] Starting Gitea on PID: 16
2025/09/21 23:48:58 cmd/web.go:114:showWebStartupMessage() [I] Gitea version: 1.24.6 built with GNU Make 4.4.1, go1.24.7 : bindata, timetzdata, sqlite, sqlite_unlock_notify
2025/09/21 23:48:58 cmd/web.go:115:showWebStartupMessage() [I] * RunMode: prod
2025/09/21 23:48:58 cmd/web.go:116:showWebStartupMessage() [I] * AppPath: /usr/local/bin/gitea
2025/09/21 23:48:58 cmd/web.go:117:showWebStartupMessage() [I] * WorkPath: /data/gitea
2025/09/21 23:48:58 cmd/web.go:118:showWebStartupMessage() [I] * CustomPath: /data/gitea
2025/09/21 23:48:58 cmd/web.go:119:showWebStartupMessage() [I] * ConfigFile: /data/gitea/conf/app.ini
2025/09/21 23:48:58 cmd/web.go:120:showWebStartupMessage() [I] Prepare to run install page
2025/09/21 23:48:58 cmd/web.go:323:listen() [I] Listen: http://0.0.0.0:3000
2025/09/21 23:48:58 cmd/web.go:327:listen() [I] AppURL(ROOT_URL): http://localhost:3000/
2025/09/21 23:48:58 modules/graceful/server.go:50:NewServer() [I] Starting new Web server: tcp:0.0.0.0:3000 on PID: 16
```

これで Gitea が AKS で稼働開始したので、カスタム DNS ドメイン `gitea.yb2.banadev.org` を Ingress の IP アドレス 172.212.3.21 に設定してブラウザで `https://gitea.yb2.banadev.org` を開けば Gitea の初期画面が現れます。 TLS 証明書は Let's Encrypt により自動的に発行されます。

この他にも `kompoxops` CLI を使って次のような運用をすることができます。

- アプリコンテナへのシェル接続: `kompoxops app exec -it -c gitea -- /bin/bash`
- ディスクのスナップショットを作成: `kompoxops snapshot create -V default`
- スナップショットからディスクを復元: `kompoxops snapshot restore -V default -S <SNAPSHOT-ID>`
- アプリにアサインするディスクを変更: `kompoxops disk attach -V default -D <DISK-ID>`
- アプリの再デプロイ・ディスクの切替: `kompoxops disk deploy`

Gitea リポジトリや Postgres データーベースは RWO PV である単一の Azure 管理ディスクの中に保存されています。
Azure 管理ディスクのライフサイクルは Kompox が管理しており AKS クラスタとは独立しているので、
スナップショットの作成、復元、別 AKS クラスタへの接続といったメンテナンスやマイグレーションが簡単にできます。

## 仕様・アーキテクチャ

[docs フォルダ](docs) に日本語によるデザインドキュメントが多くあります。
これらは GitHub Copilot Agent に対する実装の詳細を指示するために使われています。
常に最新版に更新されているわけではありませんので注意してください。

|ドキュメント|説明|
|-|-|
|[docs/Kompox-Spec-Draft.ja.md](docs/Kompox-Spec-Draft.ja.md)|仕様ドラフト・メモ|
|[docs/Kompox-Arch-v1.ja.md](docs/Kompox-Arch-v1.ja.md)|アーキテクチャ仕様|
|[docs/Kompox-KubeConverter-v1.ja.md](docs/Kompox-KubeConverter-v1.ja.md)|Compose → Kubernetes 変換仕様|
|[docs/Kompox-CLI-v1.ja.md](docs/Kompox-CLI-v1.ja.md)|CLI 仕様|

## ライセンス

[MIT License](LICENSE)
