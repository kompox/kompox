---
id: README
title: Kompox Design Document Index
updated: 2025-10-29
language: en
---

# Kompox Design Document Index

This directory holds the canonical design and planning documents for Kompox. v1 is the current CLI implementation; v2 is the future PaaS/Operator design.

## v1 (Current CLI)

| ID | Title | Language | Version | Status | Last updated |
|---|---|---|---|---|---|
| [Kompox-ProviderDriver-AKS](./v1/Kompox-ProviderDriver-AKS.ja.md) | AKS Provider Driver 実装ガイド | ja | v1 | synced | 2025-10-29 |
| [Kompox-CRD](./v1/Kompox-CRD.ja.md) | Kompox CRD-style configuration | ja | v1 | archived | 2025-10-18 |
| [Kompox-Arch-Implementation](./v1/Kompox-Arch-Implementation.ja.md) | Kompox Implementation Architecture | ja | v1 | synced | 2025-10-12 |
| [Kompox-KOM](./v1/Kompox-KOM.ja.md) | Kompox KOM configuration | ja | v1 | synced | 2025-10-19 |
| [Kompox-KubeClient](./v1/Kompox-KubeClient.ja.md) | Kompox Kube Client ガイド | ja | v1 | out-of-sync | 2025-09-26 |
| [Kompox-KubeConverter](./v1/Kompox-KubeConverter.ja.md) | Kompox Kube Converter ガイド | ja | v1 | synced | 2025-10-12 |
| [Kompox-CLI](./v1/Kompox-CLI.ja.md) | Kompox PaaS CLI | ja | v1 | synced | 2025-10-19 |
| [Kompox-Resources](./v1/Kompox-Resources.ja.md) | Kompox PaaS Resources | ja | v1 | archived | 2025-10-12 |
| [Kompox-ProviderDriver](./v1/Kompox-ProviderDriver.ja.md) | Kompox Provider Driver ガイド | ja | v1 | synced | 2025-10-12 |
| [Kompox-Logging](./v1/Kompox-Logging.ja.md) | Kompox ロギング仕様 | ja | v1 | synced | 2025-10-27 |
| [Kompox-Spec-Draft](./v1/Kompox-Spec-Draft.ja.md) | Kompox 仕様ドラフト | ja | v1 | archived | 2025-10-12 |

**Status definitions:**

- draft: No implementation yet or still under discussion
- synced: Implementation exists and document reflects it correctly
- out-of-sync: Implementation exists but document needs updates
- archived: Kept as historical reference; no longer maintained

## v2 (Future PaaS/Operator)

| ID | Title | Language | Version | Status | Last updated |
|---|---|---|---|---|---|
| [Kompox-PaaS-Roadmap](./v2/Kompox-PaaS-Roadmap.ja.md) | Kompox PaaS Roadmap | ja | v2 | draft | 2025-09-26 |

**Status definitions:**

- draft: No implementation yet or still under discussion
- synced: Implementation exists and document reflects it correctly
- out-of-sync: Implementation exists but document needs updates
- archived: Kept as historical reference; no longer maintained

## ADR

| ID | Title | Language | Version | Status | Last updated |
|---|---|---|---|---|---|
| [K4x-ADR-001](./adr/K4x-ADR-001.md) | Implement Kompox PaaS as a Kubernetes Operator | en |  | proposed | - |
| [K4x-ADR-002](./adr/K4x-ADR-002.md) | Unify snapshot restore into disk create | en |  | accepted | - |
| [K4x-ADR-003](./adr/K4x-ADR-003.md) | Unify Disk/Snapshot CLI flags and adopt opaque Source contract | en |  | accepted | - |
| [K4x-ADR-004](./adr/K4x-ADR-004.md) | Cluster ingress endpoint DNS auto-update | en |  | accepted | - |
| [K4x-ADR-005](./adr/K4x-ADR-005.md) | Support Compose configs/secrets and make bind volumes directory-only | en |  | accepted | - |
| [K4x-ADR-006](./adr/K4x-ADR-006.md) | Rename domain model Service to Workspace | en |  | accepted | - |
| [K4x-ADR-007](./adr/K4x-ADR-007.md) | Introduce CRD-style configuration | en |  | accepted | - |
| [K4x-ADR-008](./adr/K4x-ADR-008.md) | Introduce Box as a first-class ops kind | en |  | proposed | - |
| [K4x-ADR-009](./adr/K4x-ADR-009.md) | Kompox Ops Manifest Schema | en |  | proposed | - |
| [K4x-ADR-010](./adr/K4x-ADR-010.md) | Rename CRD to KOM | en |  | accepted | - |
| [K4x-ADR-011](./adr/K4x-ADR-011.md) | Introduce Defaults pseudo-resource for KOM ingestion | en |  | accepted | - |
| [K4x-ADR-012](./adr/K4x-ADR-012.md) | Introduce App.RefBase for external references | en |  | accepted | - |
| [K4x-ADR-013](./adr/K4x-ADR-013.md) | Resource protection policy for Cluster operations | en |  | accepted | - |
| [K4x-ADR-014](./adr/K4x-ADR-014.md) | Introduce Volume Types | en |  | accepted | - |

**Status definitions:**

- proposed: Under discussion and not yet accepted
- accepted: Decided and in effect
- rejected: Decided not to implement
- deprecated: No longer recommended; kept for historical reference

## Public materials (reference)

| ID | Title | Language | Version | Status | Last updated |
|---|---|---|---|---|---|
| [Kompox-Pub-CNDW2025](./pub/Kompox-Pub-CNDW2025.ja.md) | CloudNative Days Winter 2025 | ja |  | rejected | 2025-10-01 |
| [Kompox-Pub-k8snovice38](./pub/Kompox-Pub-k8snovice38.ja.md) | Kubernetes Novice Tokyo #38 | ja |  | delivered | 2025-09-26 |

**Status definitions:**

- draft: Planned or under preparation; not yet scheduled
- scheduled: Confirmed for a future date
- delivered: Completed and delivered (slides/article published)
- archived: Historical reference only

