---
id: K4x-ADR-001
title: Implement Kompox PaaS as a Kubernetes Operator
status: proposed
date: 2025-09-26
language: en
supersedes: []
supersededBy: []
---
# K4x-ADR-001: Implement Kompox PaaS as a Kubernetes Operator

## Context

Kompox aims to provide a simple path to run stateful Docker Compose applications on managed Kubernetes. We are evolving Kompox into a lightweight PaaS for existing clusters while keeping the kompoxops CLI for cluster LCM (create/delete) and for installing prerequisites (Traefik and Kompox Operator). We need to define where to place responsibilities between the CLI and an in-cluster Operator and how to expose the user-facing API.

## Decision

We will implement a Kompox Operator that reconciles three namespaced CRDs and keeps the cluster orchestration in the CLI:

- API group: `k8s.kompox.dev` (v1alpha1)
- CRDs: `KompoxApp`, `KompoxDisk`, `KompoxSnapshot` (namespaced)
- Operator runs per target cluster in a dedicated namespace `k4x-system`.
- Traefik runs in `k4x-traefik` namespace.
- kompoxops CLI remains responsible for cluster LCM and for installing Traefik and the Operator. It also generates and applies CR manifests from `kompoxops.yml`.

## Scope

- In scope: Application deployment from Compose, persistent disk lifecycle, snapshot/restore, safe disk switching, Ingress and Secrets wiring, status/conditions, GitOps alignment.
- Out of scope: Meta-orchestration of multiple clusters from a single operator, replacing CSI/VolumeSnapshot features, non-Kubernetes resource provisioning beyond what is strictly needed for disks/snapshots.

## Architecture Overview

- Operator: kubebuilder/controller-runtime based. Idempotent reconciliation with finalizers, Conditions (Ready/Progressing/Degraded), observedGeneration, and eventing.
- Provider driver: autodetect (aks/eks/gke/oke/k3s/auto) via CSIDriver/StorageClass/node labels/cloud metadata, with explicit override through CRD spec.provider.type. Prefer CSI/VolumeSnapshot; fall back to cloud APIs where necessary.
- CLI and Operator separation: CLI handles cluster creation/deletion and installs required components; Operator handles day-2 application/data operations declaratively.

## API and CRDs

- Group/Version: `k8s.kompox.dev/v1alpha1`.
- Namespacing: All three CRDs are namespaced; their produced workload resources (Deployments/StatefulSets/Services/PVCs/Ingress) live in the same namespace.
- Namespace creation: PaaS creates namespaces automatically to avoid collisions using prefix `k4x-` (e.g., `k4x-<workspace>-app-<compact-id>`), with labels:
	- `app.kubernetes.io/managed-by=kompox`
	- `kompox.dev/tenant=<workspace>` (optional)
	- `kompox.dev/app-id=<compact-id>`

### KompoxDisk

- Purpose: Abstract a cloud disk and expose a stable handle for static PV provisioning.
- Spec highlights: `size`, `zone`, `options.sku`, `deletionPolicy` (Orphan|Delete), `provider`.
- Status: `handle` (cloud disk ID), `pvName`, `conditions`.
- Behavior: create/lookup cloud disk, tag, zone pin; create a static PV with `spec.csi.volumeHandle=handle`, `reclaimPolicy: Retain`. Enforce single-attach (RWO). Finalizer to clean up when policy=Delete.

### KompoxSnapshot

- Purpose: Snapshot a disk and allow restore workflows.
- Behavior: If VolumeSnapshotClass exists, create and watch VolumeSnapshot (ReadyToUse). Otherwise call cloud snapshot API and store `snapshotHandle`. Restore produces a new KompoxDisk (fromSnapshot flow).

### KompoxApp

- Purpose: Convert Compose to Kubernetes resources and manage Ingress/Secrets/PVC wiring and safe volume switching.
- Compose source: Stored in a ConfigMap and referenced via `spec.compose.configMapRef`. Supporting env files are stored as Secrets. Secret rotation triggers rollout by hashing into Pod template annotations.
- Scheduling: Add nodeAffinity for `topology.kubernetes.io/zone` to match Disk zone when required.

## Namespaces and Installation

- Operator namespace: `k4x-system`
- Traefik namespace: `k4x-traefik`
- App namespaces: auto-generated per app/tenant; apply ResourceQuota, LimitRange, NetworkPolicy, and constrained RBAC.

### Operator Deployment Model

- Default: One Operator instance per cluster (single-controller pattern). CRDs are cluster-scoped and installed once per cluster.
- Watch scope: The Operator watches all namespaces by default or a label-selected subset (configurable). Multi-instance-per-cluster is not required for the baseline design.
- Namespace defaults: We fix the default namespaces to `k4x-system` (Operator) and `k4x-traefik` (Ingress) to simplify install guides, RBAC, and automation (e.g., SA subjects like `system:serviceaccount:k4x-traefik:traefik`).
- Overrides: For special environments, both namespaces may be overridden via Helm/CLI values. This is not the default and should be used only when organizational constraints require custom naming.

## Security and RBAC

- Minimize privileges; watch limited namespaces or label-selected subsets.
- Cloud credentials are referenced from the app namespace only when needed; prefer CSI/VolumeSnapshot to avoid cloud API credentials.
- Ensure no secret values are printed in logs/events/status.

## CLI Responsibilities

- Create/delete clusters via provider drivers (initially AKS).
- Install Traefik and Kompox Operator.
- Generate and apply CR manifests from `kompoxops.yml` (App/Disk/Snapshot). Provide high-level workflows across clusters (e.g., detach → snapshot/restore → reattach in another cluster → switch app diskRef).

## Alternatives Considered

1) Full meta-orchestration Operator managing providers/clusters (Provider/Cluster CRDs). Rejected for overreach and operational complexity.
2) Crossplane/Cluster API from day-one. Deferred; we keep a driver abstraction to enable future integration.

## Consequences

- Pros: Scope minimization, clearer tenancy boundaries, GitOps-friendly APIs, lower blast radius, reusable Kubernetes primitives (CSI/VolumeSnapshot).
- Cons: Cross-cluster workflows are handled outside of the Operator (in CLI or external orchestration). Some providers require direct API calls for snapshots/disks when CSI gaps exist.

## Risks and Mitigations

- Heterogeneous CSI/VolumeSnapshot support: Autodetect and fall back to cloud APIs; document compatibility matrix.
- Zone mismatches: Validate and mark `Degraded` until reconciled; auto-apply node affinity.
- Data safety during disk switch: Enforce safe sequence (scale down → unbind/rebind → scale up) with timeouts and backoffs.

## Sample CRs (v1alpha1)

```yaml
apiVersion: k8s.kompox.dev/v1alpha1
kind: KompoxDisk
metadata:
  name: default
  namespace: app1
spec:
  provider:
    type: auto
  size: 10Gi
  zone: "1"
  options:
    sku: PremiumV2_LRS
  deletionPolicy: Orphan
status:
  handle: ""
  pvName: ""
  conditions: []
---
apiVersion: k8s.kompox.dev/v1alpha1
kind: KompoxSnapshot
metadata:
  name: default-20250925
  namespace: app1
spec:
  diskRef:
    name: default
status:
  snapshotHandle: ""
  conditions: []
---
apiVersion: k8s.kompox.dev/v1alpha1
kind: KompoxApp
metadata:
  name: app1
  namespace: app1
spec:
  compose:
    configMapRef:
      name: app1-compose
      key: compose.yml
  ingress:
    className: traefik
    rules:
      - name: main
        port: 3000
        hosts: ["gitea.example.com"]
  volumes:
    - name: default
      diskRef:
        name: default
status:
  conditions: []
```

## Open Questions

- Do we need admission webhooks for CRD validation/defaulting in v1alpha1?
- Should we ship an official Helm chart for the Operator and CRDs from the start?
- How to structure the compatibility matrix across CLI version, Operator image, and CRD versions?

