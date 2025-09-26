---
title: Kompox PaaS Resources
version: v1
status: archived
updated: 2025-09-26
language: ja
---

# Kompox PaaS Resources

## Overview

Kompox PaaS では次の kind のリソースを扱う。

- Persistent Resources (DBで永続化するリソース)
  - Service
  - Provider
  - Cluster
  - App

## Persistent Resources

### Service

```yaml
version: v1
kind: Service
name: <service-name>
```

### Provider

```yaml
version: v1
kind: Provider
name: <provider-name>
service: <service-name>
driver: <driver-name>
settings:
  <setting-name>: <setting-value>
```

### Cluster

```yaml
version: v1
kind: Cluster
name: <cluster-name>
provider: <provider-name>
existing: <bool>
ingress:
  controller: <traefik>
  namespace: <ingress-namespace>
  serviceAccount: <sa-name>
  domain: <default-domain>            # 例: apps.example.com
  certResolver: <staging|production>
  certEmail: <email@example.com>
  certificates:                       # 静的証明書の参照
    - name: <cert-name>
      source: <provider-specific-url> # 例(AKS): https://<kv>.vault.azure.net/secrets/<name>
settings:
  <setting-name>: <setting-value>
```

### App

```yaml
version: v1
kind: App
name: <app-name>
cluster: <cluster-name>
compose: <compose-config>
ingress:
  certResolver: <staging|production>  # cluster 側を上書き
  rules:
    - name: <rule-name>
      port: <port-number>
      hosts: [<host1>, <host2>]
volumes:
  - name: <volume-name>
    size: <quantity>                  # 例: 32Gi
resources:                            # Pod 単位リソース
  cpu: <quantity>
  memory: <quantity>
settings:
  <setting-name>: <setting-value>
```
