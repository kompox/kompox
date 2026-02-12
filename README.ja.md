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
  - [x] `kompoxops.yml` 設定ファイルの定義 (互換用の単一ファイルモード)
  - [x] クロスプラットフォーム CI/CD (GoReleaser)
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
- [ ] クロスプラットフォーム対応
  - [ ] darwin/arm64, linux/arm64, windows/amd64 動作テスト

## 発表・登壇

- **2025/09/25(木)** [**Kubernetes Novice Tokyo #38**](https://k8s-novice-jp.connpass.com/event/365526/)
  - Kompox: クラウドネイティブコンテナ Web アプリ DevOps ツールの紹介
  - [スライド資料 (PDF)](https://www.docswell.com/s/yaegashi/544R21-KNT38-Kompox)
- **2025/11/18(火)** [**CloudNative Days Winter 2025**](https://event.cloudnativedays.jp/cndw2025) (予定)
  - [Kompox: ステートフルワークロード運用がスケールする未来をつくる](https://event.cloudnativedays.jp/cndw2025/proposals/1000)
  - 採択前のプロポーザルです。興味を持たれた方はぜひ投票をお願いします

## Kompox の背景にある課題

Kubernetesはステートレスアプリの運用基盤として普及しましたが、ステートフルなワークロードの運用は依然として複雑です。

- **ストレージの制約:** 性能要件からReadWriteOnce (RWO) の永続ボリューム (PV) が必要になるケースが多くありますが、RWOボリュームは単一ノードにしか接続できず、障害時の復旧や移行が困難になる一因となっています。
- **クラウド間の差異:** 各クラウド (Azure, AWS, Google, OCI など) が提供するマネージドKubernetesやストレージサービスには微妙な仕様の違いがあり、アプリケーションのポータビリティを妨げます。
- **開発と本番のギャップ:** Docker Compose の `compose.yml` を使ったローカルでの手軽な開発体験を、そのまま本番のKubernetes環境に持ち込むことは困難です。

Kompoxはこれらの課題を解決し、ステートフルワークロードの運用をスケールさせることを目指します。

- **シンプルな定義ファイル:** `compose.yml` の資産を活かしつつ、`kompoxapp.yml` と KOM (Workspace/Provider/Cluster/App) でアプリケーションとインフラを定義できます。
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

AKS 向けの KOM 設定(例: `kompoxapp.yml` と Workspace/Provider/Cluster/App の YAML)を用意し `kompoxops` CLI ツールを使うことで、次のことができます。

注: `kompoxops.yml` 単一ファイルモードは互換用途として残っていますが、廃止予定のため新規利用は推奨しません。

- AKS クラスタをプロビジョン (認証は Azure CLI によるものを使用)
- クラスタに Ingress Controller (traefik) や共通 Kubernetes リソースをインストール
- RWO PV の実体となる Azure 管理ディスク (Premium SSD v2, 10GiB) を作成
- compose.yml から変換した Kubernetes Manifest をデプロイしてアプリを公開

```yaml
apiVersion: ops.kompox.dev/v1alpha1
kind: Defaults
spec:
  komPath:
    - ./kom
  appId: /ws/aks-e2e-gitea-20250925-060355/prv/aks1/cls/cluster1/app/app1
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: aks-e2e-gitea-20250925-060355
  annotations:
    ops.kompox.dev/id: /ws/aks-e2e-gitea-20250925-060355
spec: {}
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: aks1
  annotations:
    ops.kompox.dev/id: /ws/aks-e2e-gitea-20250925-060355/prv/aks1
spec:
  driver: aks
  settings:
    AZURE_AUTH_METHOD: azure_cli
    AZURE_SUBSCRIPTION_ID: 9473abf6-f25e-420e-b3f2-128c1c7b46f2
    AZURE_LOCATION: eastus
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  name: cluster1
  annotations:
    ops.kompox.dev/id: /ws/aks-e2e-gitea-20250925-060355/prv/aks1/cls/cluster1
spec:
  existing: false
  ingress:
    certEmail: yaegashi@live.jp
    certResolver: staging
    domain: cluster1.aks1.exp.kompox.dev
    certificates:
      - name: l0wdevtls
        source: https://l0wdevtls-jpe-prd1.vault.azure.net/secrets/cluster1-aks1-exp-kompox-dev
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
---
apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  name: app1
  annotations:
    ops.kompox.dev/id: /ws/aks-e2e-gitea-20250925-060355/prv/aks1/cls/cluster1/app/app1
spec:
  compose: file:compose.yml
  ingress:
    certResolver: staging
    rules:
      - name: main
        port: 3000
        hosts: [gitea.custom.exp.kompox.dev]
  deployment:
    zone: "1"
  volumes:
    - name: default
      size: 10Gi
      options:
        sku: PremiumV2_LRS
```

実行例

```console
# AKS クラスタをプロビジョン
$ kompoxops cluster provision
2025/09/25 06:04:14 INFO provision start cluster=cluster1
2025/09/25 06:04:14 INFO aks cluster provision begin subscription=9473abf6-f25e-420e-b3f2-128c1c7b46f2 resource_group=k4x-50vf7y_cls_cluster1_62mpgv tags="map[kompox-cluster-hash:62mpgv kompox-cluster-name:cluster1 kompox-provider-name:aks1 kompox-service-name:aks-e2e-gitea-20250925-060355 managed-by:kompox]"
2025/09/25 06:10:39 INFO aks cluster provision succeeded subscription=9473abf6-f25e-420e-b3f2-128c1c7b46f2 resource_group=k4x-50vf7y_cls_cluster1_62mpgv
2025/09/25 06:10:39 INFO provision success cluster=cluster1

# クラスタに Ingress Controller (traefik) 他をインストール
$ kompoxops cluster install
2025/09/25 06:10:45 INFO install start cluster=cluster1
2025/09/25 06:10:45 INFO aks cluster install begin cluster=cluster1 provider=aks1
2025/09/25 06:11:01 INFO successfully assigned Key Vault Secrets User role key_vault=l0wdevtls-jpe-prd1 secret_name=cluster1-aks1-exp-kompox-dev cert_name=l0wdevtls principal_id=09331589-56b6-49d0-a440-6515949f2cbf
2025/09/25 06:11:01 INFO Key Vault role assignment summary success_count=1 error_count=0 total_count=1
2025/09/25 06:11:01 INFO applying kind=SecretProviderClass name=traefik-kv-l0wdevtls-jpe-prd1 namespace=traefik force=false
2025/09/25 06:11:03 INFO applying kind=ConfigMap name=traefik namespace=traefik force=false
2025/09/25 06:12:15 INFO aks cluster install succeeded cluster=cluster1 provider=aks1
2025/09/25 06:12:15 INFO install success cluster=cluster1

# クラスタの状態を表示
$ ./kompoxops cluster status
{
  "existing": false,
  "provisioned": true,
  "installed": true,
  "ingressGlobalIP": "135.222.244.115",
  "cluster_id": "ccdf75d3320cf5ea",
  "cluster_name": "cluster1"
}

# クラスタに compose.yml から変換した Kubernetes Manifest をデプロイ
# App.spec.volumes で定義した Azure 管理ディスクが自動的に作成され RWO PV としてマウントされます
$ kompoxops app deploy --bootstrap-disks
2025/09/25 06:12:20 INFO bootstrap disks before deploy app=app1
2025/09/25 06:12:24 INFO ensuring resource group subscription=9473abf6-f25e-420e-b3f2-128c1c7b46f2 location=eastus resource_group=k4x-50vf7y_app_app1_13o40q tags="map[kompox-app-id-hash:13o40q kompox-app-name:app1 kompox-provider-name:aks1 kompox-service-name:aks-e2e-gitea-20250925-060355 managed-by:kompox]"
2025/09/25 06:12:26 INFO ensuring role assignment scope=/subscriptions/9473abf6-f25e-420e-b3f2-128c1c7b46f2/resourceGroups/k4x-50vf7y_app_app1_13o40q principal_id=bf4fc6cf-a899-4dad-85a7-48bf1c513373 role_definition_id=b24988ac-6180-42a0-ab88-20f7382dd24c
2025/09/25 06:12:41 INFO applying kind=Namespace name=k4x-50vf7y-app1-13o40q namespace="" force=true
2025/09/25 06:12:41 INFO applying kind=ServiceAccount name=app1 namespace=k4x-50vf7y-app1-13o40q force=true
2025/09/25 06:12:41 INFO applying kind=NetworkPolicy name=app1 namespace=k4x-50vf7y-app1-13o40q force=true
2025/09/25 06:12:41 INFO applying kind=PersistentVolume name=k4x-50vf7y-default-13o40q-5xmnms namespace="" force=true
2025/09/25 06:12:42 INFO applying kind=PersistentVolumeClaim name=k4x-50vf7y-default-13o40q-5xmnms namespace=k4x-50vf7y-app1-13o40q force=true
2025/09/25 06:12:42 INFO applying kind=Secret name=app1-app-postgres-base namespace=k4x-50vf7y-app1-13o40q force=true
2025/09/25 06:12:42 INFO applying kind=Secret name=app1-app-gitea-base namespace=k4x-50vf7y-app1-13o40q force=true
2025/09/25 06:12:42 INFO applying kind=Deployment name=app1-app namespace=k4x-50vf7y-app1-13o40q force=true
2025/09/25 06:12:43 INFO applying kind=Service name=app1-app namespace=k4x-50vf7y-app1-13o40q force=true
2025/09/25 06:12:43 INFO applying kind=Service name=gitea namespace=k4x-50vf7y-app1-13o40q force=true
2025/09/25 06:12:43 INFO applying kind=Service name=postgres namespace=k4x-50vf7y-app1-13o40q force=true
2025/09/25 06:12:43 INFO applying kind=Ingress name=app1-app-default namespace=k4x-50vf7y-app1-13o40q force=true
2025/09/25 06:12:44 INFO applying kind=Ingress name=app1-app-custom namespace=k4x-50vf7y-app1-13o40q force=true
2025/09/25 06:12:44 INFO deploy success app=app1
2025/09/25 06:12:45 INFO patched deployment secrets deployment=app1-app hashChanged=true imagePullSecretsChanged=false

# クラスタにデプロイされたアプリの状態を表示
$ kompoxops app status
{
  "app_id": "d7a5e3f3326dc6bf",
  "app_name": "app1",
  "cluster_id": "3fdb93b7b0e964d2",
  "cluster_name": "cluster1",
  "ready": false,
  "image": "docker.gitea.com/gitea:1.24.6",
  "namespace": "k4x-50vf7y-app1-13o40q",
  "node": "aks-npuser1-33452345-vmss000000",
  "deployment": "app1-app",
  "pod": "app1-app-5bb7f44495-ckbpt",
  "container": "gitea",
  "command": null,
  "args": null,
  "ingress_hosts": [
    "app1-13o40q-3000.cluster1.aks1.exp.kompox.dev",
    "gitea.custom.exp.kompox.dev"
  ]
}

# アプリコンテナのログ表示
$ ./kompoxops app logs -c gitea
Generating /data/ssh/ssh_host_ed25519_key...
Generating /data/ssh/ssh_host_rsa_key...
Generating /data/ssh/ssh_host_ecdsa_key...
Server listening on :: port 22.
Server listening on 0.0.0.0 port 22.
2025/09/25 06:13:54 cmd/web.go:261:runWeb() [I] Starting Gitea on PID: 15
2025/09/25 06:13:54 cmd/web.go:114:showWebStartupMessage() [I] Gitea version: 1.24.6 built with GNU Make 4.4.1, go1.24.7 : bindata, timetzdata, sqlite, sqlite_unlock_notify
2025/09/25 06:13:54 cmd/web.go:115:showWebStartupMessage() [I] * RunMode: prod
2025/09/25 06:13:54 cmd/web.go:116:showWebStartupMessage() [I] * AppPath: /usr/local/bin/gitea
2025/09/25 06:13:54 cmd/web.go:117:showWebStartupMessage() [I] * WorkPath: /data/gitea
2025/09/25 06:13:54 cmd/web.go:118:showWebStartupMessage() [I] * CustomPath: /data/gitea
2025/09/25 06:13:54 cmd/web.go:119:showWebStartupMessage() [I] * ConfigFile: /data/gitea/conf/app.ini
2025/09/25 06:13:54 cmd/web.go:120:showWebStartupMessage() [I] Prepare to run install page
2025/09/25 06:13:54 cmd/web.go:323:listen() [I] Listen: http://0.0.0.0:3000
2025/09/25 06:13:54 cmd/web.go:327:listen() [I] AppURL(ROOT_URL): http://localhost:3000/
2025/09/25 06:13:54 modules/graceful/server.go:50:NewServer() [I] Starting new Web server: tcp:0.0.0.0:3000 on PID: 15

# KUBECONFIG取得とkubectlの実行
$ ./kompoxops cluster kubeconfig --merge --set-current
$ kubectl get pod -o wide
NAME                        READY   STATUS    RESTARTS   AGE   IP             NODE                              NOMINATED NODE   READINESS GATES
app1-app-5bb7f44495-ckbpt   2/2     Running   0          52m   10.244.1.160   aks-npuser1-33452345-vmss000000   <none>           <none>
$ kubectl get ingress -o wide
NAME               CLASS     HOSTS                                           ADDRESS           PORTS   AGE
app1-app-custom    traefik   gitea.custom.exp.kompox.dev                     135.222.244.115   80      52m
app1-app-default   traefik   app1-13o40q-3000.cluster1.aks1.exp.kompox.dev   135.222.244.115   80      52m
```

これで Gitea が AKS で稼働開始したので、カスタム DNS ドメイン `gitea.custom.exp.kompox.dev` を Ingress の IP アドレス 135.222.244.115 に設定してブラウザで `https://gitea.custom.exp.kompox.dev` を開けば Gitea の初期画面が現れます。 TLS 証明書は Let's Encrypt により自動的に発行されます。

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

[Kompox 設計ドキュメント目次](design/README.ja.md) を参照してください。

## ライセンス

[MIT License](LICENSE)
