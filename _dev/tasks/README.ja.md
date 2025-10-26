---
id: README
title: 開発者向けタスク索引
updated: 2025-10-26
language: ja
---

# 開発者向けタスク索引

この索引は、本ディレクトリにある開発者向けの短いタスク文書を一覧化したものです。`updated` 日付（なければ `id` の年）で年ごとにグルーピングしています。

- 開発者向けガイド: [GUIDE.ja.md](./GUIDE.ja.md)
- English version: [README.en.md](./README.en.md)

## 2025 年

| ID | タイトル | ステータス | カテゴリ | 担当 | 更新日 | 言語 |
|---|---|---|---|---|---|---|
| [2025-10-24-log](./2025-10-24-logging.ja.md) | kompoxops ログ標準化（Event/Span/Step パターン適用） | active |  |  | 2025-10-26 | ja |
| [2025-10-23-aks-cr](./2025-10-23-aks-cr.ja.md) | AKS Driver - ACR 権限付与対応 | done |  | yaegashi | 2025-10-23 | ja |
| [2025-10-23-protection](./2025-10-23-protection.ja.md) | リソース保護ポリシー導入 Step 1-3 | done |  |  | 2025-10-23 | ja |
| [2025-10-18-refbase](./2025-10-18-refbase.ja.md) | App.RefBase の導入と参照解決の一元化 | done |  | yaegashi | 2025-10-19 | ja |
| [2025-10-17-defaults](./2025-10-17-defaults.ja.md) | KOM 命名統一(CRD→KOM)と Defaults 実装 | done |  | yaegashi | 2025-10-17 | ja |
| [2025-10-15-kom](./2025-10-15-kom.ja.md) | KOM(Kompox Ops Manifest) 導入と適用 | done |  | yaegashi | 2025-10-15 | ja |
| [2025-10-13-crd-p2](./2025-10-13-crd-p2.ja.md) | CLI への CRD 取り込み統合 | done |  | yaegsahi | 2025-10-13 | ja |
| [2025-10-13-crd-p1](./2025-10-13-crd-p1.ja.md) | CRD DTO とローダーの初期実装（ops.kompox.dev/v1alpha1） | done |  | yaegashi | 2025-10-13 | ja |
| [2025-10-12-workspace](./2025-10-12-workspace.ja.md) | Domain Service → Workspace への改名 | completed |  |  | 2025-10-12 | ja |
| [2025-10-10-configs-secrets](./2025-10-10-configs-secrets.md) | Kube Converter における configs/secrets 対応と volumes ディレクトリ専用化 | done |  |  | 2025-10-11 | ja |
| [2025-10-07-aks-dns](./2025-10-07-aks-dns.ja.md) | AKS Driver - ClusterDNSApply 実装と DNS 権限付与 | active |  |  | 2025-10-08 | ja |
| [2025-10-01-cluster-dns](./2025-10-01-cluster-dns.ja.md) | Cluster DNS 実処理・Usecase/CLI 実装 | done |  |  | 2025-10-06 | ja |
| [2025-09-30-cluster-dns](./2025-09-30-cluster-dns.ja.md) | Cluster DNS 適用 API の追加(Driver/Domain) | done |  |  | 2025-10-06 | ja |
| [2025-09-28-disk-snapshot-cli-flags](./2025-09-28-disk-snapshot-cli-flags.ja.md) | Disk/Snapshot CLI フラグ統一(-N/-S) | done |  | yaegashi | 2025-09-29 | ja |
| [2025-09-27-disk-snapshot-unify](./2025-09-27-disk-snapshot-unify.ja.md) | Disk/Snapshot 機能統合(disk create -S) | done |  | yaegashi | 2025-09-29 | ja |

---

ステータス凡例:
- active: 作業中
- blocked: 依存や判断待ちで停止
- done: 完了（履歴として保持）
- canceled: 取り止め
- superseded: 後続のタスクに置き換え

注意:
- タスクは短く具体的に。設計判断は ADR/仕様に記録し、タスクからリンクします。
- インデックス化のため、YAML フロントマターのフィールドを必ず埋めてください。
