---
id: README
title: Kompox 設計ドキュメント目次
updated: 2025-09-26
language: ja
---

# Kompox 設計ドキュメント目次

本ディレクトリは Kompox の設計・計画ドキュメントの正本です。v1 は現行 CLI 実装、v2 は将来の PaaS/Operator 設計です。

## v1（現行 CLI 実装）

| ID | Title | Language | Version | Status | Last updated |
|---|---|---|---|---|---|
| [Kompox-KubeClient](./v1/Kompox-KubeClient.ja.md) | Kompox Kube Client ガイド | ja | v1 | out-of-sync | 2025-09-26 |
| [Kompox-KubeConverter](./v1/Kompox-KubeConverter.ja.md) | Kompox Kube Converter ガイド | ja | v1 | synced | 2025-09-26 |
| [Kompox-Arch](./v1/Kompox-Arch.ja.md) | Kompox PaaS Architecture | ja | v1 | out-of-sync | 2025-09-26 |
| [Kompox-CLI](./v1/Kompox-CLI.ja.md) | Kompox PaaS CLI | ja | v1 | out-of-sync | 2025-09-26 |
| [Kompox-Resources](./v1/Kompox-Resources.ja.md) | Kompox PaaS Resources | ja | v1 | archived | 2025-09-26 |
| [Kompox-ProviderDriver-AKS](./v1/Kompox-ProviderDriver-AKS.ja.md) | Kompox Provider Driver AKS ガイド | ja | v1 | out-of-sync | 2025-09-26 |
| [Kompox-ProviderDriver](./v1/Kompox-ProviderDriver.ja.md) | Kompox Provider Driver ガイド | ja | v1 | out-of-sync | 2025-09-26 |
| [Kompox-Spec-Draft](./v1/Kompox-Spec-Draft.ja.md) | Kompox 仕様ドラフト | ja | v1 | archived | 2025-09-26 |

**ステータスの意味:**

- draft: 実装が存在しない、もしくは検討段階のドラフト
- synced: 実装が存在し、文書がその実装内容を正しく反映
- out-of-sync: 実装は存在するが、文書が追随しておらず更新が必要
- archived: 古い参考資料として保管し、今後は更新しない

## v2（将来 PaaS/Operator 設計）

| ID | Title | Language | Version | Status | Last updated |
|---|---|---|---|---|---|
| [Kompox-PaaS-Roadmap](./v2/Kompox-PaaS-Roadmap.ja.md) | Kompox PaaS Roadmap | ja | v2 | draft | 2025-09-26 |

**ステータスの意味:**

- draft: 実装が存在しない、もしくは検討段階のドラフト
- synced: 実装が存在し、文書がその実装内容を正しく反映
- out-of-sync: 実装は存在するが、文書が追随しておらず更新が必要
- archived: 古い参考資料として保管し、今後は更新しない

## ADR

| ID | Title | Language | Version | Status | Last updated |
|---|---|---|---|---|---|
| [K4x-ADR-001](./adr/K4x-ADR-001.md) | Implement Kompox PaaS as a Kubernetes Operator | en |  | proposed | 2025-09-26 |

**ステータスの意味:**

- proposed: 検討中で、まだ採択されていない
- accepted: 採択済みで有効
- rejected: 採択せず不採用
- deprecated: 推奨しない（歴史的経緯として残置）
- superseded: 新しい ADR によって置き換え

## 公開資料（参考）

| ID | Title | Language | Version | Status | Last updated |
|---|---|---|---|---|---|
| [Kompox-Pub-CNDW2025](./pub/Kompox-Pub-CNDW2025.ja.md) | CloudNative Days Winter 2025 | ja |  | draft | 2025-09-26 |
| [Kompox-Pub-k8snovice38](./pub/Kompox-Pub-k8snovice38.ja.md) | Kubernetes Novice Tokyo #38 | ja |  | delivered | 2025-09-26 |

**ステータスの意味:**

- draft: 企画・準備段階で未確定
- scheduled: 実施予定が確定
- delivered: 実施完了（登壇/公開済み）
- archived: 参考資料として保管

