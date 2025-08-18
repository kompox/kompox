# Kompox Converter: Docker Compose to Kubernetes Manifest

## 概要

Kompox の Service/Provider/Cluster/App リソースがどのように Kubernetes マニフェストに変換されるかを説明します。

## 方針

### Kubernetes マニフェスト

次のリソースを含む Kubernetes マニフェストが作られる

- Namespace 1個
- PVC 1個 (別のライフサイクルで管理される静的なPVを参照する)
- Deployment 1個 (シングルレプリカのPod)
- Service 1個 (compose の host ポートを列挙)
- Ingress 1個 (DNSホスト名からServiceへのルーティングを列挙)

命名規約

- Namespace: `kompox-<appName>-<HASH>`
- Service/Deployment/Ingress/PVC: `<appName>`
  - 当面は固定とする。将来的にはバージョン管理のために `<appName>-<version>` などの形式を導入。
- PV: `kompox-<appName>-<HASH>`
  - Azure Disk リソースを参照する。

`<HASH>` は以下で生成する:

```
BASE = service.name + ":" + provider.name + ":" + cluster.name + ":" + app.name
HASH = sha1(BASE) の先頭6文字 (16進)
```

PV 名にも同じ `<HASH>` を用いて一貫性を保つ
(Azure リソース ID 由来の別アルゴリズムは採用しない／採用する場合は `<PVHASH>` として別記する)。

### ボリューム

- Compose でサポートする形式 `./<subpath>:<mountpoint>`
- 単一の PVC に `subPath` で割り当てる。

subPath 正規化ルール

1. 先頭の `./` を除去  
2. `..` を含む場合エラー  
3. 連続 `/` を 1 個に畳み込み  
4. 末尾 `/` を除去 (結果空ならエラー)  

initContainers により subPath ディレクトリを自動作成する。

静的な PV リソースの例

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  annotations:
    pv.kubernetes.io/provisioned-by: disk.csi.azure.com
  name: kompox-app1-HASH
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
- プロトコルはHTTPのみをサポートする。
- `hostPort:containerPort` の形式のみサポートする。
- 複数のサービスが同じ `containerPort` を使用する設定は明示的なエラーとする (コンテナは同一Podで稼働するため)。

app.ingress スキーマ

```yaml
app.ingress:
  - name: <portName>
    port: <hostPort:int>
    hosts: [<fqdn>, ...]   # 1件以上
```

- name: `^[a-z]([-a-z0-9]{0,14})$` (Kubernetes Service port 名制約)
- port: Compose `hostPort` のいずれか。未定義ならエラー。
- 同一 port を複数エントリが参照することは禁止 (エラー)。
- hosts: 各要素 DNS-1123 subdomain。エントリ内重複は 1 回目のみ採用し警告。異なるエントリ間で同一 FQDN 再出現はエラー。
- app.ingress が空 (または未指定) の場合 Ingress を生成しない。

Service 生成の仕様
- `ports` は app.ingress の定義順。
- `port` = app.ingress.port, `targetPort` = 対応する `containerPort`。
- 複数サービス (Compose) が同一 containerPort を公開 (ports に含める) する構成はエラー。

Ingress 生成の仕様
- `rules` 出力順: app.ingress 定義順、各エントリ内 host 配列順。
- 各 host 1 rule, path は常に `/` (Prefix)。
- `annotations.traefik.ingress.kubernetes.io/router.entrypoints: websecure`
- TLS セクションは生成しない (Traefik 側 ACME 自動取得前提)。

### ラベル・セレクタ

各リソースには次のラベルを設定する。

```yaml
metadata:
  labels:
    app: <appName>
    app.kubernetes.io/name: <appName>
    app.kubernetes.io/instance: <appName>-<HASH>
    app.kubernetes.io/managed-by: kompox
```

セレクタとしては `app: <appName>` を使用する。

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
  domain: ops.kompox.dev
  ingress:
    controller: traefik
    namespace: traefik
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
          - ./data/postgres:/var/lib/postgresql/data
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
```

### Kubernetes Manifest

```yaml
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: app1
  namespace: kompox-app1-HASH
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 32Gi
  volumeName: kompox-app1-HASH
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app1
  namespace: kompox-app1-HASH
  labels:
    app: app1
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-HASH
    app.kubernetes.io/managed-by: kompox
spec:
  replicas: 1
  selector:
    matchLabels:
      app: app1
  template:
    metadata:
      labels:
        app: app1
        app.kubernetes.io/name: app1
        app.kubernetes.io/instance: app1-HASH
        app.kubernetes.io/managed-by: kompox
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
        - name: default
          mountPath: /var/lib/postgresql/data
          subPath: data/postgres
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
            set -eu
            for d in data/app data/postgres; do
              mkdir -p /work/$d
            done
        volumeMounts:
          - name: default
            mountPath: /work
      volumes:
        - name: default
          persistentVolumeClaim:
            claimName: app1
---
apiVersion: v1
kind: Service
metadata:
  name: app1
  namespace: kompox-app1-HASH
  labels:
    app: app1
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-HASH
    app.kubernetes.io/managed-by: kompox
spec:
  ports:
  - name: main
    port: 8080
    targetPort: 80
  - name: admin
    port: 8081
    targetPort: 8080
  selector:
    app: app1
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app1
  namespace: kompox-app1-HASH
  labels:
    app: app1
    app.kubernetes.io/name: app1
    app.kubernetes.io/instance: app1-HASH
    app.kubernetes.io/managed-by: kompox
  annotations:
    traefik.ingress.kubernetes.io/router.entrypoints: websecure
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
                name: app1
                port:
                  name: main
    - host: admin.custom.kompox.dev
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: app1
                port:
                  name: admin
```
