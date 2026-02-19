---
title: Kompox ホーム
---

# Kompox (K4x)

## 概要

**Docker Compose で動いていたステートフルアプリ （データベースやリポジトリホスティングなど） を、リアーキテクチャ最小でマネージド Kubernetes に移行するためのオーケストレーションツール**

Kompox は [Kompose](https://kompose.io) のアイデアを拡張したオーケストレーションツールです。

Docker Compose を Kubernetes マニフェストに変換するだけでなく、RWO 永続ボリュームとスナップショットのライフサイクル管理（作成・バックアップ・リストア・移行）までを積極的に担います。

これにより、マネージドコンテナ基盤では扱いづらい **ブロックストレージ前提のステートフルアプリの運用** を、マネージドK8s上で現実的に運用可能にします。

K4x は Kompox の略称表記です。

## 特徴

- **compose.yml ベースのワークフロー:** ローカル Docker 環境 (開発・検証) と Kubernetes クラスタ (本番) の両方で動く compose.yml が作成可能
- **ステートフルアプリ特化:** データベースやファイルサーバーなど、状態を持つアプリの本番運用に適した構成を自動生成
- **RWO ディスク・スナップショット管理:** クラウドネイティブで高性能な永続ボリュームのライフサイクルを自動管理、バックアップ・リストア・クラスタ間移行まで対応
- **ノードプール管理:** クラウド側のキャパシティ・クオータ制約と折り合いながら、スペックや優先度の異なる複数のノードプールを構成し、Pod の適切なスケジュールとコスト最適化・耐障害性を実現
- **可用性ゾーン対応:** Pod・ディスク・スナップショットをゾーンを意識して配置・管理
- **マルチクラウド対応:** リファレンス実装として AKS (Azure) をサポート、OKE (OCI)、EKS (AWS)、GKE (Google)、K3s (セルフホスト) にも順次対応予定

## ロードマップ

- **v1 (Kompox CLI)**
    - CLI `kompoxops` によるコア機能の実装
    - Kompox Ops Manifest (KOM) フォーマットの定義と実装
    - AKS をリファレンスとするクラウドプロバイダドライバの実装
    - 各クラウドのプロバイダドライバの順次対応 (AKS → OKE → EKS → GKE → K3s)
- **v2 (Kompox PaaS)**
    - Kompox CRD: KOM ベースの Kubernetes ネイティブリソース定義
    - Kompox Operator: Kompox リソースを管理する Kubernetes コントローラー
    - Kompox PaaS: RBAC・ビリングなどマルチテナント要件に対応した PaaS レイヤー

2026年2月現在、v1 のコア機能と AKS ドライバの基本実装が完了しています。アルファ段階のため各機能はプロトタイプレベルであり、ドキュメントも開発者向けの設計資料が中心です。今後、運用補助ツールやユーザー向けドキュメント・チュートリアルを整備していく予定です。

## リソース

- [GitHub リポジトリ](https://github.com/kompox/kompox)
    - [開発者向けドキュメント](https://github.com/kompox/kompox/blob/main/design/README.md)
        - [ADR (Architectural Decision Records)](https://github.com/kompox/kompox/blob/main/design/adr/README.md)
        - [プラン](https://github.com/kompox/kompox/blob/main/design/plans/README.md)
        - [タスク](https://github.com/kompox/kompox/blob/main/design/tasks/README.md)
        - [v1ドキュメント](https://github.com/kompox/kompox/blob/main/design/v1/README.md)
    - [リリース](https://github.com/kompox/kompox/releases) (CLI `kompoxops` のダウンロード)
