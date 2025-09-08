# Kompox Kube Converter ガイド v1

## 概要

本書は `adapters/kube` が提供するコンバータ `kube.Converter` の設計と公開契約を解説します。Docker Compose を入力として、Kubernetes マニフェストへ変換する方針とルールを示します。

- 本書で扱う主な事項:
  - Service/Provider/Cluster/App 定義からのマニフェスト生成規則

## 方針

### リソース

次のリソースを含む Kubernetes マニフェストが作られる。

- Namespace 1個 (アプリごとに作成)
- PV 複数個 (Provider のライフサイクルで管理される静的なクラウドディスクリソースを参照する RWO ボリューム)
- PVC 複数個 (PVを参照する)
- コンポーネント (`app` `box` など) ごとに以下を生成
  - Deployment 1個 (シングルレプリカ、strategy.type:Recreate)
  - Service 1個 (compose の host ポートを列挙)
  - Ingress 0〜2個 (DNSホスト名から Service へのルーティング)
    - デフォルトドメイン用 Ingress: `<appName>-default`
    - カスタムドメイン用 Ingress: `<appName>-custom`
    - 生成条件は後述

### 名前・ラベル・アノテーション

変換時に次のようなコンポーネント名 `<componentName>` を指定する。

- `app` (アプリ)
- `box` (Kompox Box)

リソース命名規則

- Namespace: `kompox-<appName>-<idHASH>`
- PV/PVC: `kompox-<volName>-<idHASH>-<volHASH>`
- Service/Deployment: `<appName>-<componentName>`
- Ingress: デフォルトドメイン用は `<appName>-<componentName>-default`、カスタムドメイン用は `<appName>-<componentName>-custom`

各リソースには次のラベルを設定する。

```yaml
metadata:
  labels:
    app: <appName>-<componentName>
    app.kubernetes.io/name: <appName>
    app.kubernetes.io/instance: <appName>-<inHASH>
    app.kubernetes.io/component: <componentName>
    app.kubernetes.io/managed-by: kompox
    kompox.dev/app-instance-hash: <inHASH>
    kompox.dev/app-id-hash: <idHASH>
```

ラベルの意味

- `app`: セレクタラベル (Deployment/Service などで使用)
- `app.kubernetes.io/name`: 表示名
- `app.kubernetes.io/instance`: インスタンス名
- `app.kubernetes.io/component`: コンポーネント名
- `kompox.dev/app-instance-hash`: クラスタ依存インスタンスハッシュ
- `kompox.dev/app-id-hash`: クラスタ非依存アプリ識別ハッシュ

Pod (Deployment の template) にも同一のラベルを付与する。 Deployment や Service の Pod セレクタでは次のラベルを照合する。

```yaml
app: <appName>-app
```

Namespace には次のアノテーションを設定する。

```yaml
metadata:
  annotations:
    kompox.dev/app: <serviceName>/<providerName>/<clusterName>/<appName>
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

### ハッシュの種類と生成規則

`<inHASH>` (クラスタ依存ハッシュ) 生成方法

```
BASE = service.name + ":" + provider.name + ":" + cluster.name + ":" + app.name
HASH = sha1(BASE) の先頭6文字 (16進)
```

`<idHASH>` (クラスタ非依存ハッシュ) 生成方法

```
BASE = service.name + ":" + provider.name + ":" + app.name
HASH = sha1(BASE) の先頭6文字 (16進)
```

`<volHASH>` (クラウドディスクリソースハッシュ) 生成方法

```
BASE = クラウドディスクリソースのID (/subscriptions/.... など)
HASH = sha1(BASE) の先頭6文字 (16進)
```

各ハッシュの衝突が理論上発生した場合は実装側でハッシュ長 (6→8→10 文字…) を自動延長する。

### ボリューム

app.volumes スキーマ

```yaml
app.volumes:
  - name: <name>
    size: <size>
```

- name: `^[a-z]([-a-z0-9]{0,14})$`
- size: `32Gi` など

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
  - name: default  # PV/PVC kompox-default-<idHASH>-<volHASH>
    size: 32Gi
  - name: data     # PV/PVC kompox-data-<idHASH>-<volHASH>
    size: 32Gi
```

### x-kompox (リソース変換)

| キー | 意味 | K8s 出力 |
|------|------|---------|
| x-kompox.resources.cpu | CPU リクエスト (例: 100m) | resources.requests.cpu |
| x-kompox.resources.memory | メモリリクエスト (例: 256Mi) | resources.requests.memory |
| x-kompox.limits.cpu | CPU 上限 | resources.limits.cpu |
| x-kompox.limits.memory | メモリ上限 | resources.limits.memory |

未指定フィールドは出力しない。limits のみ指定時に requests を補完しない。

### 環境変数

Compose `environment` の key/value をそのままコピー。Secret 化やフィルタリングは本仕様範囲外。

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
- 名前は `<appName>-default`
- ingressClassName は `traefik`
- `rules`
  - `app.ingress.rules` の各エントリに対して1つを出力
  - `host` は `<appName>-idHASH-<port>.{cluster.ingress.domain}`
    - ここで `<port>` は `app.ingress.rules.port`（Compose の `hostPort`）
    - 例: `main(8080→80)` は `app1-idHASH-8080.ops.kompox.dev`、`admin(8081→8080)` は `app1-idHASH-8081.ops.kompox.dev`
  - `path: /` および `pathType: Prefix`
- annotations 設定（certresolver を設定せず静的 TLS 証明書を使用する）
```yaml
traefik.ingress.kubernetes.io/router.entrypoints: websecure
traefik.ingress.kubernetes.io/router.tls: "true"
```

カスタムドメイン Ingress 生成の仕様
- `app.ingress.rules` が空配列でないときのみ生成
- 名前は `<appName>-custom`
- ingressClassName は `traefik`
- `rules`
  - `app.ingress.rules` の `hosts` 配列の各要素ごとに1つを出力
  - `path: /` および `pathType: Prefix`
- annotations 設定（certresolver を設定して ACME TLS 証明書を使用する）
```yaml
traefik.ingress.kubernetes.io/router.entrypoints: websecure
traefik.ingress.kubernetes.io/router.tls: "true"
traefik.ingress.kubernetes.io/router.tls.certresolver: {app.ingress.certResolver}
```

補足（ホスト名の重複）
- `app.ingress.rules` 内で同一 FQDN が複数回現れた場合:
  - エントリ内重複は 1 回目のみ採用し警告、異なるエントリ間の重複はエラー
- デフォルトドメインで自動生成されるホストと `app.ingress.rules.hosts` が衝突した場合はエラーとする

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

## 例1

### kompoxops.yml

```yaml
version: v1
service:
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
```

### Kubernetes Manifest

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: kompox-app1-idHASH
  labels:
    app: app1-app
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
    app.kubernetes.io/managed-by: kompox
    kompox.dev/app-instance-hash: inHASH
    kompox.dev/app-id-hash: idHASH
  annotations:
    kompox.dev/app: ops/aks1/cluster1/app1
    kompox.dev/provider-driver: aks
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: kompox-default-idHASH-volHASH
  labels:
    app: app1-app
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
  name: kompox-db-idHASH-volHASH
  labels:
    app: app1-app
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
  name: kompox-default-idHASH-volHASH
  namespace: kompox-app1-idHASH
  labels:
    app: app1-app
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
  volumeName: kompox-default-idHASH-volHASH
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: kompox-db-idHASH-volHASH
  namespace: kompox-app1-idHASH
  labels:
    app: app1-app
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
  volumeName: kompox-db-idHASH-volHASH
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app1-app
  namespace: kompox-app1-idHASH
  labels:
    app: app1-app
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
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
        app.kubernetes.io/managed-by: kompox
        kompox.dev/app-instance-hash: inHASH
        kompox.dev/app-id-hash: idHASH
    spec:
      containers:
      - name: app
        image: ghcr.io/kompox/app
        env:
        - name: TZ
          value: Asia/Tokyo
        ports:
        - containerPort: 80
        - containerPort: 8080
        volumeMounts:
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
        - name: default
          persistentVolumeClaim:
            claimName: kompox-default-idHASH-volHASH
        - name: db
          persistentVolumeClaim:
            claimName: kompox-db-idHASH-volHASH
---
apiVersion: v1
kind: Service
metadata:
  name: app1-app
  namespace: kompox-app1-idHASH
  labels:
    app: app1-app
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
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
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app1-app-default
  namespace: kompox-app1-idHASH
  labels:
    app: app1-app
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
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
  namespace: kompox-app1-idHASH
  labels:
    app: app1-app
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-inHASH
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
