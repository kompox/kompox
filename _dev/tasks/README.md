---
id: README
title: Developer Tasks Index
updated: 2025-10-13
language: en
---

# Developer Tasks Index

This index lists short, action-oriented developer tasks found in this folder. Tasks are grouped by year (based on the `updated` date or `id` prefix).

- Developer's guide: [GUIDE.en.md](./GUIDE.en.md)
- 日本語版: [README.ja.md](./README.ja.md)

## 2025

| ID | Title | Status | Category | Owner | Updated | Language |
|---|---|---|---|---|---|---|
| [2025-10-13-crd](./2025-10-13-crd.ja.md) | CRD DTO とローダーの初期実装（ops.kompox.dev/v1alpha1） | done |  | yaegashi | 2025-10-13 | ja |
| [2025-10-12-workspace](./2025-10-12-workspace.ja.md) | Domain Service → Workspace への改名 | completed |  |  | 2025-10-12 | ja |
| [2025-10-10-configs-secrets](./2025-10-10-configs-secrets.md) | Kube Converter における configs/secrets 対応と volumes ディレクトリ専用化 | done |  |  | 2025-10-11 | ja |
| [2025-10-07-aks-dns](./2025-10-07-aks-dns.ja.md) | AKS Driver - ClusterDNSApply 実装と DNS 権限付与 | active |  |  | 2025-10-08 | ja |
| [2025-10-01-cluster-dns](./2025-10-01-cluster-dns.ja.md) | Cluster DNS 実処理・Usecase/CLI 実装 | done |  |  | 2025-10-06 | ja |
| [2025-09-30-cluster-dns](./2025-09-30-cluster-dns.ja.md) | Cluster DNS 適用 API の追加(Driver/Domain) | done |  |  | 2025-10-06 | ja |
| [2025-09-28-disk-snapshot-cli-flags](./2025-09-28-disk-snapshot-cli-flags.ja.md) | Disk/Snapshot CLI フラグ統一(-N/-S) | done |  | yaegashi | 2025-09-29 | ja |
| [2025-09-27-disk-snapshot-unify](./2025-09-27-disk-snapshot-unify.ja.md) | Disk/Snapshot 機能統合(disk create -S) | done |  | yaegashi | 2025-09-29 | ja |

---

Status legend:
- active: Work in progress
- blocked: Waiting on dependency or decision
- done: Completed; kept for history
- canceled: Stopped intentionally
- superseded: Replaced by a newer task

Notes:
- Tasks are intentionally short and specific. Decisions should be captured in ADRs and specifications.
- Use the YAML front matter fields to ensure consistent indexing.
