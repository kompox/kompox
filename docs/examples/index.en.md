---
title: Kompox Usage Example
---

# Kompox Usage Example

Suppose you have a `compose.yml` like the following to run [Gitea](https://about.gitea.com/).

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

You can test this normally in a local development environment using Docker Compose.

```bash
$ docker compose up -d
[+] Running 3/3
 ✔ Network aks-e2e-gitea_default       Created        0.1s
 ✔ Container aks-e2e-gitea-postgres-1  Started        0.3s
 ✔ Container aks-e2e-gitea-gitea-1     Started        0.3d
```

By preparing KOM settings for AKS (for example, `kompoxapp.yml` and Workspace/Provider/Cluster/App YAML files) and using the `kompoxops` CLI, you can do the following:

- Provision an AKS cluster (using Azure CLI authentication)
- Install the Ingress Controller (traefik) and common Kubernetes resources
- Create Azure managed disks (Premium SSD v2, 10GiB) as the backing storage for RWO PVs
- Deploy Kubernetes manifests converted from `compose.yml` and expose the app

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

Execution example

```console
# Provision the AKS cluster
$ kompoxops cluster provision
2025/09/25 06:04:14 INFO provision start cluster=cluster1
2025/09/25 06:04:14 INFO aks cluster provision begin subscription=9473abf6-f25e-420e-b3f2-128c1c7b46f2 resource_group=k4x-50vf7y_cls_cluster1_62mpgv tags="map[kompox-cluster-hash:62mpgv kompox-cluster-name:cluster1 kompox-provider-name:aks1 kompox-service-name:aks-e2e-gitea-20250925-060355 managed-by:kompox]"
2025/09/25 06:10:39 INFO aks cluster provision succeeded subscription=9473abf6-f25e-420e-b3f2-128c1c7b46f2 resource_group=k4x-50vf7y_cls_cluster1_62mpgv
2025/09/25 06:10:39 INFO provision success cluster=cluster1

# Install Ingress Controller (traefik) and related resources into the cluster
$ kompoxops cluster install
2025/09/25 06:10:45 INFO install start cluster=cluster1
2025/09/25 06:10:45 INFO aks cluster install begin cluster=cluster1 provider=aks1
2025/09/25 06:11:01 INFO successfully assigned Key Vault Secrets User role key_vault=l0wdevtls-jpe-prd1 secret_name=cluster1-aks1-exp-kompox-dev cert_name=l0wdevtls principal_id=09331589-56b6-49d0-a440-6515949f2cbf
2025/09/25 06:11:01 INFO Key Vault role assignment summary success_count=1 error_count=0 total_count=1
2025/09/25 06:11:01 INFO applying kind=SecretProviderClass name=traefik-kv-l0wdevtls-jpe-prd1 namespace=traefik force=false
2025/09/25 06:11:03 INFO applying kind=ConfigMap name=traefik namespace=traefik force=false
2025/09/25 06:12:15 INFO aks cluster install succeeded cluster=cluster1 provider=aks1
2025/09/25 06:12:15 INFO install success cluster=cluster1

# Show cluster status
$ ./kompoxops cluster status
{
  "existing": false,
  "provisioned": true,
  "installed": true,
  "ingressGlobalIP": "135.222.244.115",
  "cluster_id": "ccdf75d3320cf5ea",
  "cluster_name": "cluster1"
}

# Deploy Kubernetes manifests converted from compose.yml
# The Azure managed disk defined in App.spec.volumes is automatically created and mounted as an RWO PV
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

# Show deployed app status
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

# Show app container logs
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

# Get kubeconfig and run kubectl
$ ./kompoxops cluster kubeconfig --merge --set-current
$ kubectl get pod -o wide
NAME                        READY   STATUS    RESTARTS   AGE   IP             NODE                              NOMINATED NODE   READINESS GATES
app1-app-5bb7f44495-ckbpt   2/2     Running   0          52m   10.244.1.160   aks-npuser1-33452345-vmss000000   <none>           <none>
$ kubectl get ingress -o wide
NAME               CLASS     HOSTS                                           ADDRESS           PORTS   AGE
app1-app-custom    traefik   gitea.custom.exp.kompox.dev                     135.222.244.115   80      52m
app1-app-default   traefik   app1-13o40q-3000.cluster1.aks1.exp.kompox.dev   135.222.244.115   80      52m
```

At this point, Gitea is running on AKS. Set the custom DNS domain `gitea.custom.exp.kompox.dev` to the Ingress IP address `135.222.244.115`, then open `https://gitea.custom.exp.kompox.dev` in your browser to access the initial Gitea screen. TLS certificates are automatically issued by Let's Encrypt.

With the `kompoxops` CLI, you can also perform operations such as:

- Connect a shell to the app container: `kompoxops app exec -it -c gitea -- /bin/bash`
- Create a disk snapshot: `kompoxops snapshot create -V default`
- Restore a disk from a snapshot: `kompoxops snapshot restore -V default -S <SNAPSHOT-ID>`
- Change the disk assigned to the app: `kompoxops disk attach -V default -D <DISK-ID>`
- Redeploy the app and switch disk: `kompoxops disk deploy`

Gitea repositories and the PostgreSQL database are stored on a single Azure managed disk used as an RWO PV.
The lifecycle of that Azure managed disk is managed by Kompox and is independent from the AKS cluster,
so maintenance and migration tasks such as snapshot creation/restoration and attaching to another AKS cluster are straightforward.
