---
title: Kompox Home
---

# Kompox (K4x)

## Overview

**An orchestration tool to migrate stateful applications (Git / Perforce / DB, etc.) running on Docker Compose to managed Kubernetes with minimal re-architecture**

Kompox extends the ideas from [Kompose](https://kompose.io) as an orchestration tool.

Beyond converting Docker Compose to Kubernetes manifests, Kompox actively manages the lifecycle of RWO persistent volumes and snapshots — including creation, backup, restore, and migration.

This makes it practically feasible to operate **block-storage-dependent stateful applications** on managed Kubernetes, which are otherwise difficult to handle on managed container platforms.

K4x is the short form of Kompox.

## Key Features

- **compose.yml-based workflow:** Create a single compose.yml that works in both local Docker environments (dev/staging) and Kubernetes clusters (production)
- **Stateful app specialization:** Automatically generates production-ready configurations for stateful applications such as databases and file servers
- **RWO disk & snapshot management:** Automated lifecycle management of cloud-native, high-performance persistent volumes — covering backup, restore, and cross-cluster migration
- **Node pool management:** Configure multiple node pools with different specs and priorities, balancing cloud capacity/quota constraints for optimal Pod scheduling, cost efficiency, and fault tolerance
- **Availability zone support:** Zone-aware placement and management of Pods, disks, and snapshots
- **Multi-cloud support:** AKS (Azure) as the reference implementation, with OKE (OCI), EKS (AWS), GKE (Google), and K3s (self-hosted) planned

## Roadmap

- **v1 (Kompox CLI)**
    - Core feature implementation via the `kompoxops` CLI
    - Definition and implementation of the Kompox Ops Manifest (KOM) format
    - Cloud provider driver implementation with AKS as the reference
    - Sequential support for additional cloud provider drivers (AKS → OKE → EKS → GKE → K3s)
- **v2 (Kompox PaaS)**
    - Kompox CRD: KOM-based Kubernetes-native resource definitions
    - Kompox Operator: Kubernetes controller for managing Kompox resources
    - Kompox PaaS: PaaS layer supporting multi-tenant requirements such as RBAC and billing

As of February 2026, core v1 features and the basic AKS driver implementation are complete. The project is in alpha stage, so each feature is at prototype level and documentation primarily consists of developer-facing design materials. Going forward, we plan to develop operational tools and user-facing documentation and tutorials.

## Resources

- [GitHub Repository](https://github.com/kompox/kompox)
    - [Developer Documentation](https://github.com/kompox/kompox/blob/main/design/README.md)
        - [ADR (Architectural Decision Records)](https://github.com/kompox/kompox/blob/main/design/adr/README.md)
        - [Plans](https://github.com/kompox/kompox/blob/main/design/plans/README.md)
        - [Tasks](https://github.com/kompox/kompox/blob/main/design/tasks/README.md)
        - [v1 Documents](https://github.com/kompox/kompox/blob/main/design/v1/README.md)
    - [Releases](https://github.com/kompox/kompox/releases) (Download the `kompoxops` CLI)
