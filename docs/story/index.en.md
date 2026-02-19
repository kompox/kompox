---
title: Kompox Story
---

# Kompox Story

This page explains why the author started the Kompox project, what goals Kompox is aiming for, and its mid- to long-term vision.

## Starting Point: Running Services on Linux VMs + Docker Compose

For many years, the author's service infrastructure consisted of many containers running with Docker Compose on many virtual machines (VMs) in the cloud, with a Traefik reverse proxy at the front routing traffic to backend service containers.

![](diag-compose.svg)

This setup has a proven track record of very stable operation. However, maintenance work such as VM OS updates, security patching, and instance type changes is unavoidable, and operational overhead grows as the number of VMs increases.

The project started from a desire to break away from this situation by using cloud-native technologies such as Kubernetes. The goal is to containerize and cloud-native-enable many internal web-service workloads that previously ran on VMs, so operations can be automated through APIs and operational burden can be significantly reduced.

## The Hard Part: Stateful Workloads

### Kubernetes and Its Stateless-First Design

Kubernetes is widely adopted as a cloud-native orchestration platform for containers, and many managed services are available around it. However, Kubernetes fundamentally assumes **stateless workloads**—that is, workloads where each container instance does not keep persistent local data.

### Stateless vs. Stateful

A **stateless workload** is an application where each instance does not keep persistent data on its local disk. Even if an instance terminates, no persistent data is lost, and another instance can immediately take over. All persistent data is delegated to external systems (such as managed databases or object storage).

- Web frontend servers (serving HTML/CSS/JS)
- REST API servers that delegate all state to external databases
- Reverse proxies and load balancers
- Workers that consume jobs from queues and write results to external systems

A **stateful workload** is an application where each instance keeps persistent data on local disk, and that data must survive Pod restarts and rescheduling.

- Relational databases (PostgreSQL, MySQL) — store data files on local disk
- Git hosting servers (Gitea, GitLab, GHES) — store repositories on local filesystems
- Wiki/document systems — keep uploaded files and databases locally
- Legacy applications with embedded databases or local-file dependencies

Some stateful workloads can be migrated to managed services (for example, replacing self-hosted PostgreSQL with managed PostgreSQL). But workloads such as Git hosting servers, applications dependent on custom local-file storage, or legacy applications that cannot be modified cannot simply move to managed services. These applications must run with their own data on local disks.

### Constraints of RWO Persistent Volumes

Such local-disk-dependent stateful workloads often require **ReadWriteOnce (RWO)** persistent volumes (PVs) for performance reasons.

An RWO PV abstracts VM-attached block devices (Azure Managed Disks, Amazon EBS, Google Persistent Disks, OCI Block Volume, etc.). It delivers high I/O performance, but with a major constraint: it can be attached to **only one node at a time**.

In contrast, **ReadWriteMany (RWX)** PVs abstract network file services (NFS/SMB; Azure Files, Amazon EFS, Google Filestore, OCI File Storage, etc.). They can be mounted from multiple nodes simultaneously, but performance is generally moderate.

Simple managed container services from cloud providers (Azure Container Apps, AWS Fargate, Google Cloud Run, OCI Container Instances, etc.) support only RWX PVs, and therefore cannot run workloads that require RWO PVs. As a result, **for local-disk-dependent workloads that cannot move to managed services, directly using RWO PVs on Kubernetes is effectively the only option**. This is the biggest challenge in cloud-native modernization for stateful applications.

## Kompox Approach: Bring compose.yml to the Cloud

The local development experience with Docker Compose and `compose.yml` is simple and powerful. You can freely combine containers (web servers, databases, etc.) with mounted persistent volumes, and bring up the app with a single `docker compose up -d` command for local validation. The core idea of Kompox is to reproduce that same experience for production services running on cloud Kubernetes clusters.

There is a predecessor project called [Kompose](https://kompose.io). Kompose converts Docker Compose files into Kubernetes manifests, but it stops at conversion and is insufficient as a production operations tool.

Kompox significantly extends the Kompose idea. It goes beyond `compose.yml` conversion and into operations of cloud-native resources such as managed Kubernetes clusters, RWO PVs, and disk snapshots across cloud providers, aiming to be practical for production service operation.

The diagram below shows the full architecture that Kompox enables. Container groups defined in `compose.yml` are deployed as Pods on a Kubernetes cluster spanning multiple availability zones (AZs). Persistent data for each Pod is stored on AZ-local RWO disks, with backup and cross-AZ migration through snapshots. In the diagram, "Kubernetes-managed" represents responsibilities handled by Kubernetes itself (Pod scheduling, service routing, etc.), and "Kompox-managed" represents responsibilities handled by Kompox (lifecycle management for RWO disks and snapshots).

![](diag-kompox.svg)

## What Kompox Aims to Achieve

Kompox is an orchestration tool to streamline DevOps for stateful containerized web applications built around RWO PVs. Its goals can be summarized as follows:

**Run a compose.yml-based workflow seamlessly from local to cloud:**
`compose.yml` works as-is with local `docker compose`, and can also be deployed to managed Kubernetes clusters across cloud providers via the `kompoxops` CLI. By preparing `compose.yml` and KOM (Kompox Ops Manifest) config files, developers can auto-generate Kubernetes resource configurations suitable for stateful production workloads and deploy them.

**Absorb cross-cloud differences:**
Through the Provider Driver architecture layer, Kompox absorbs differences in RWO PV and snapshot capabilities across AKS (Azure), OKE (OCI), EKS (AWS), GKE (Google), and K3s (self-hosted). A single set of definitions can be reused in multi-cloud environments.

**Manage disk and snapshot lifecycles:**
From RWO persistent volume creation to backup/restore and migration across AZs, regions, and clouds during failures, Kompox leverages cloud-native snapshot features for automated lifecycle management. Pods, disks, and snapshots are placed and managed with AZ awareness.

**Optimize node pools and scheduling:**
While working within cloud-side capacity and quota constraints, Kompox can configure multiple node pools with different specs and priorities to realize appropriate Pod scheduling, cost optimization, and resilience.

**Build toward a PaaS:**
In the future, Kompox aims to become an internal container-app hosting platform, including RBAC and billing capabilities as a full PaaS.

## Availability Targets Assumed by Kompox

### Intentionally Not Using Core Kubernetes HA Features

When using Kubernetes, it is common to rely on features such as horizontal autoscaling, high availability through multi-replica deployment, and zero-downtime deployment via rolling updates. Kompox, however, **assumes none of these**.

| Typical Kubernetes Capability | Kompox Policy |
|---|---|
| Horizontal scaling (HPA/VPA) | Not used. Each workload is fixed as a single replica |
| High availability via multi-replica | Not used. No standby failover replicas |
| Rolling updates | Not feasible. Zero-downtime deployment is fundamentally impossible with a single replica |
| Stateless-oriented design | Not followed. State is kept locally on RWO disks instead of being delegated to external services |

The Kubernetes ecosystem evolved around the philosophy of "Cattle, not Pets" (treat instances as replaceable cattle, not precious pets). Kompox takes the opposite stance: it treats each workload as a "Pet" with unique data, while using only Kubernetes infrastructure capabilities—declarative configuration, health checks with auto-restart, and API-driven operational automation. In other words, Kompox uses **Kubernetes as a programmable VM manager**.

### Availability with a Single-Replica Assumption

Kompox targets **stateful workloads** such as web apps traditionally described and operated with Docker Compose, where all persistent data is contained within the application stack. Each workload is operated as a **single replica** by design. Databases are also kept in containers instead of being moved to managed services, taking advantage of RWO storage performance.

Under this assumption, Kompox sets the following availability targets:

| Metric | Target | Rationale |
|------|--------|--------|
| **RPO** (node failure) | 0 | Even if a node fails, the RWO disk is preserved by the cloud platform, so no data loss occurs<br>Database journaling and similar mechanisms preserve committed data after crashes |
| **RPO** (AZ failure) | At the latest snapshot point | When migrating to another AZ after an AZ outage, recovery point depends on the latest cross-AZ snapshot<br>RPO depends on snapshot frequency, so periodic snapshot operations are critical<br>However, RPO=0 is achievable when using zone-redundant storage (described below) |
| **RTO** (planned maintenance) | Tens of seconds to a few minutes | Planned downtime including node drain, disk reattachment, and Pod startup |
| **RTO** (unplanned failure / auto-recovery) | Tens of minutes to several hours | In addition to Kubernetes node-failure detection (default 5–6 minutes), forced disk detach/reattach and Pod restart are required<br>Large incidents such as AZ outages can take even longer |
| **SLO** | 99.9% | Even with a single replica and RTO>0, this level is targeted by establishing service monitoring and auto-recovery via Kubernetes (annual downtime budget: 8.75 hours) |
| **SLA** | 99.0% | Acceptable level for internal services<br>Provides sufficient buffer against SLO for application-layer defects and maintenance time (annual downtime budget: 87.5 hours) |

Kompox takes responsibility for mechanisms that reduce downtime caused by the RWO constraint of "single-node attachment only."

### Achieving RPO=0 with Zone-Redundant Storage

The table above assumes snapshot-point RPO for AZ failures, but some cloud providers offer **zone-redundant** block storage. With this, RPO=0 can be achieved even during AZ failures.

| Cloud | Product | Summary |
|---------|--------|------|
| **Azure** | Managed Disks ZRS (Premium SSD v1, Standard SSD) | Synchronous replication across 3 AZs in a region. During AZ outages, disks can be reattached in another AZ without data loss. Premium SSD v2 and Ultra Disk do not support ZRS |
| **Google Cloud** | Regional Persistent Disks (pd-ssd, pd-balanced) | Synchronous replication across 2 zones in a region. Failover is possible to the paired zone on zone failure |
| **AWS** | — | EBS has no zone-redundant configuration and is always redundant only within a single AZ. AZ-failure recovery depends on snapshot-based restore |
| **OCI** | — | Block Volume has no cross-AD redundant configuration and is redundant only within a single AD |

Zone-redundant storage is expensive (typically about 2x the cost of regular block storage) and availability is limited by region, but it is an effective option for workloads requiring RPO=0. Kompox supports zone-redundant storage, with Provider Drivers absorbing provider-specific differences.

## Kompox Development Roadmap

Kompox has the following roadmap:

- v1 (Kompox CLI)
	- Implement the core `kompoxops` CLI and Provider Driver, enabling deployment and snapshot management for simple Compose apps with RWO PV on AKS.
	- Define and implement the KOM (Kompox Ops Manifest) format for layered configuration management across Workspace, Provider, Cluster, and App.
	- Add support for other cloud providers such as OKE, EKS, GKE, and K3s to enable multi-cloud reuse of the same definition files.
	- Implement auto-recovery from large incidents such as AZ outages to significantly reduce RTO.
- v2 (Kompox PaaS)
	- Based on the Kompox CLI implementation, add the following Kubernetes-targeted components:
		- Kompox CRD: Enable definition and management of Compose apps as Kubernetes-native resources.
		- Kompox Operator: Manage application lifecycle as a Kubernetes controller.
	- Implement multi-tenant PaaS capabilities to realize an internal container application hosting platform.
		- Allow multiple teams/projects to host applications independently on a single Kompox instance.
		- Implement billing based on cloud resource usage to support cost management.

