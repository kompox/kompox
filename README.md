# Kompox (K4x)

**An orchestration tool to seamlessly run stateful applications written in Docker Compose on managed Kubernetes in the cloud**

Kompox extends the ideas from [Kompose](https://kompose.io) to solve production operational challenges for stateful workloads. K4x is the short form of Kompox.

**日本語によるプロジェクトの説明: [README.ja.md](README.ja.md)**

## Project Status

**This project is in alpha stage as of September 2025. CLI and internal APIs may undergo breaking changes.**

While Kompox is designed for multi-cloud support, we are currently focusing on Microsoft Azure for individual feature implementation.

## Overview

Kompox addresses the complexity of running stateful workloads on Kubernetes by providing:

- **Simple configuration**: Use `kompoxapp.yml` + KOM (Workspace/Provider/Cluster/App) while reusing existing `compose.yml` assets
- **Cloud abstraction**: Provider Driver architecture that abstracts differences between cloud platforms (AKS, EKS, GKE, OKE)
- **Easy data management**: Cloud-native snapshot capabilities for backup, restore, and cross-zone/region/cloud migration
- **Consistent operations**: Unified CLI experience from local development to production cloud environments

## Quick Example

Transform your Docker Compose application (e.g., Gitea with PostgreSQL) into a production-ready Kubernetes deployment with persistent storage, ingress, and TLS certificates - all with simple CLI commands:

Primary CLI input mode is KOM (`kompoxapp.yml` + Workspace/Provider/Cluster/App manifests). In this mode, app/cluster settings are represented as `App.spec.*` and `Cluster.spec.*` fields. Legacy single-file `kompoxops.yml` mode remains for compatibility and is deprecated for new setups.

```bash
# Provision AKS cluster
kompoxops cluster provision

# Install ingress controller and common resources
kompoxops cluster install

# Create persistent volumes (Azure managed disks)
kompoxops disk create -V default

# Deploy application from compose.yml
kompoxops app deploy
```

## Key Features

- **Stateful workload focus**: RWO volume management with cloud-native snapshots
- **Multi-cloud ready**: Currently supports Azure AKS, with plans for K3s, EKS, GKE, and OKE
- **Production-ready**: Ingress controller, TLS certificates, network isolation
- **Data lifecycle management**: Independent disk lifecycle from cluster, enabling easy migration and maintenance

## Documentation

Comprehensive documentation is available in Japanese:

- **[README.ja.md](README.ja.md)**: Complete project overview, roadmap, and usage examples
- **[design/](design/)**: Kompox Design Document Index: detailed specifications, architecture documents, etc.

The documentation includes:
- Detailed use cases and examples
- Complete CLI reference
- Architecture specifications
- Implementation details

## Community

- **Presentations**:
  - Kubernetes Novice Tokyo #38 (2025/09/25)
  - CloudNative Days Winter 2025 (2025/11/18, planned)

## License

[MIT License](LICENSE)