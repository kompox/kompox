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
provider: <provider-name>
existing: <bool>
domain: <domain-name>
ingress:
  <ingress-name>: <ingress-value>
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
  <ingress-name>: <ingress-value> 
resources:
  <resource-name>: <resource-value>
settings:
  <setting-name>: <setting-value>
```
