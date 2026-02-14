---
id: K4x-ADR-014
title: Introduce Volume Types
status: accepted
updated: 2025-10-23
language: en
supersedes: []
supersededBy: []
---
# K4x-ADR-014: Introduce Volume Types

## Context

- Kompox currently models volumes around a logical volume → provisioned artifact ("disk") → optional snapshots. This works well for block storage (RWO) but does not natively cover provider-managed network file shares (RWX) such as Azure Files, AWS EFS, or GCP Filestore.
- We need RWX support without breaking existing volume/disk/snapshot semantics, public APIs, or UseCases. The new capability must be provider-neutral and keep Kubernetes/CNI/CSI details out of domain models.
- Consistency goals:
  - Preserve the meaning of "Disk" as "provisioned storage artifact" (not strictly a block device)
  - Keep VolumePort unchanged; drivers map the artifact to provider-native resources
  - Default behavior remains identical for existing apps

## Decision

- Add a canonical type selector to logical volumes: `AppVolume.Type` with values:
  - `"disk"` (default across all drivers): block-device-backed volumes (typically RWO)
  - `"files"`: network file shares for RWX
- Define stable constants in the domain for type safety and public contract:
  - `VolumeTypeDisk = "disk"`, `VolumeTypeFiles = "files"`
- Do not change `VolumePort` signatures or behavior; reinterpret "Disk" as "provisioned artifact". For `Type = "files"`, a "Disk" entry represents a file share.
- Snapshots remain part of the contract but may be unsupported for `Type = "files"`:
  - Drivers return a common `ErrNotSupported` for `Snapshot*` operations when the provider/type does not support snapshots.
- Artifact shape for `Type = "files"`:
  - `VolumeDisk.Name`: share/export name (provider rules apply)
  - `VolumeDisk.Handle`: provider-native identifier (CSI driver format or URI)
    - Azure Files: `{rg}#{account}#{share}#####{subscription}` (CSI volumeHandle format with 6 `#` separators)
    - Other providers may use URIs: `nfs://{host}:/{export}`
  - `VolumeDisk.Size`: share quota (bytes); 0 if not set
  - `VolumeDisk.Zone`: empty for regional services; availability/replication via `Options`
  - `VolumeDisk.Options`: provider-specific attributes (e.g., `protocol=smb|nfs`, `quotaGiB`, `skuName`, `availability`, `mountUID/GID`, `accessPointId`)
- Kubernetes StorageClass provisioner mapping (non-normative, for driver guidance):

  | Platform | `disk` (typically RWO) | `files` (RWX) |
  |---|---|---|
  | AKS | Managed Disk (`disk.csi.azure.com`) | Azure Files (`file.csi.azure.com`; SMB/NFS via parameters) |
  | EKS | EBS (`ebs.csi.aws.com`) | EFS (`efs.csi.aws.com`) |
  | GKE | Persistent Disk (`pd.csi.storage.gke.io`) | Filestore (`filestore.csi.storage.gke.io`) |
  | OKE | Block Volume Service (`blockvolume.csi.oraclecloud.com`) | File Storage Service (`fss.csi.oraclecloud.com`) |
  | K3s | local-path provisioner (`rancher.io/local-path`; RWO) | NFS (`nfs.csi.k8s.io` or `k8s-sigs.io/nfs-subdir-external-provisioner`) |
- Authentication note for Azure Files:
  - Workload Identity authenticates the CSI driver to obtain keys/SAS; SMB/NFS data-plane does not accept OIDC directly. This is internal to drivers and not exposed in domain models. [Kompox-ProviderDriver-AKS.ja.md]
- Defaults and UX:
  - When `AppVolume.Type` is empty, treat as `"disk"`.
  - Only `"disk"` and `"files"` are valid values. Unknown values MUST fail validation early (CRD conversion/domain validation).
  - For `Type = "files"`, drivers should default to RWX access mode (Kubernetes) and pick reasonable protocol/SKU defaults per provider.

## Vocabulary and Extensibility

- Canonical Types and validation
  - Domain accepts only two canonical values: `"disk"` and `"files"`.
  - Empty means `"disk"`. Any other value is invalid and MUST be rejected with a clear error.
  - Drivers MUST NOT introduce new `AppVolume.Type` values; new families require an ADR.
- Provider flavors via options (non-normative keys)
  - Use `VolumeClass.Attributes` and/or `VolumeDisk.Options` to express provider flavors:
    - `backend`: e.g., `managed-disk`, `elastic-san`, `azurefiles`, `anf`, `azureblob`, `efs`, `filestore`
    - `protocol`: `smb` | `nfs` | `fuse` (for blobfuse/azureblob)
    - `skuName`, `performanceClass`, `availability` (e.g., `Premium_ZRS`, `Standard_LRS`, `zrs`|`lrs`|`regional`)
    - `quotaGiB`, `mountUID`, `mountGID`, `fileMode`, `dirMode`, `accessPointId`, `encryptInTransit`
- Coverage of Azure storage families under this vocabulary
  - `disk`: Azure Managed Disk, Elastic SAN (exposed as block to nodes)
  - `files`: Azure Files (SMB/NFS), Azure NetApp Files, Azure Managed Lustre, Azure Blob via FUSE (azureblob) with caveats
  - Note (FUSE/object-like backends): while mountable as a filesystem, POSIX/consistency semantics may differ. Keep under `files` with `backend=azureblob` for now; consider a future `"object"` type only if domain-level differences become material.

## Alternatives Considered

- Drive RWX solely via access modes/StorageClass
  - Rejected: leaky abstraction into domain; couples domain with Kubernetes specifics and undermines provider neutrality.
- Introduce a new logical resource (e.g., `FileShare`) separate from `Volume`
  - Rejected: splits the logical volume concept and complicates UX and UseCases; most semantics overlap with volumes.
- Encode provider choices through `VolumeClass` only (no `Type`)
  - Rejected: unclear intent and weak guardrails; drivers must infer RWX from opaque attributes, increasing drift.

## Consequences

- Pros
  - Minimal change surface: public APIs and UseCases remain stable; drivers opt-in by honoring `Type`.
  - Clear, provider-neutral vocabulary (`disk` vs `files`) with stable constants.
  - Keeps Kubernetes/CSI details out of domain; mapping lives in drivers/adapters.
- Cons/Constraints
  - Snapshots commonly unsupported for `files`; callers must handle `ErrNotSupported` for `Snapshot*`.
  - Provider-specific naming/quotas (e.g., share name rules) surface as validation in drivers.
  - Azure Files SMB/AAD intricacies (e.g., Kerberos) are not modeled; recommended path is MI/WI for driver-side key/SAS retrieval.
  - k3s/local-path provides node-local persistence only; HA/failover is out of scope.

## Rollout

1) Domain and spec
   - Add `AppVolume.Type` to spec/docs with default `"disk"`; introduce constants `VolumeTypeDisk`/`VolumeTypeFiles` and `ErrNotSupported`.
   - Clarify `VolumeDisk` semantics for `files` (Name/Handle/Options) in docs.
2) Drivers/adapters
   - AKS driver: implement `files` with Azure Files (one Storage Account per App by default), share-as-disk, URI handles, RWX PVCs.
   - Keep `Snapshot*` returning `ErrNotSupported` for `files`.
  - k3s driver: treat local-path provisioner as `Type="disk"` with `backend=localpath` (RWO). `Disk*` operations are no-op/`ErrNotSupported`; rely on dynamic provisioning. Optionally, a dev-only `backend=hostpath` may manage directories on a single node, with explicit scheduling constraints.
3) Kubernetes conversion
   - For `files`, emit RWX PVCs and provider CSI parameters; support SMB/NFS options. [Kompox-KubeConverter.ja.md]
4) Tests/docs
   - Add unit/E2E for `files` type (list/create/assign; snapshot non-support). Update design docs and indices.

## References

- [Kompox-KubeConverter.ja.md]
- [Kompox-ProviderDriver-AKS.ja.md]

[Kompox-KubeConverter.ja.md]: ../v1/Kompox-KubeConverter.ja.md
[Kompox-ProviderDriver-AKS.ja.md]: ../v1/Kompox-ProviderDriver-AKS.ja.md
