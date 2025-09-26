---
id: Kompox-PaaS-Roadmap
title: Kompox PaaS Roadmap
version: v2
status: draft
updated: 2025-09-26
language: ja
---
# Kompox PaaS Roadmap

注意: 本ドキュメントは Operator ベース PaaS の将来設計です。現行 CLI 実装とは異なります。

この文書は、Kompox を「Kubernetes Operator 利用の軽量 PaaS」として進化させるための設計方針と移行計画をまとめたものです。

## 目的と非目標

- 目的
  - Docker Compose ベースのステートフル Web アプリを、既存のマネージド Kubernetes クラスタ上で簡潔に運用する。
  - 日常運用（デプロイ、ディスク、スナップショット、切替）を宣言的にし、GitOps に適合させる。
  - マルチクラウド/複数クラスタへ段階的に拡張可能なアーキテクチャとする。
- 非目標
  - クラスタのメタ・オーケストレーション（全てのクラスタを単一コントローラで集中管理）は対象外。
  - 既存の Kubernetes 標準（CSI/VolumeSnapshot 等）が担える領域を再実装しない。

## コンポーネントの役割分担

- kompoxops CLI（維持・強化）
  - ターゲットクラスタのライフサイクル管理（作成/削除）
  - クラスタへの必須コンポーネント導入: Traefik、Kompox Operator
  - `kompoxops.yml` → CR(YAML) 生成・適用（KompoxApp/Disk/Snapshot）
  - 複数クラスタにまたがる運用ワークフロー（例: スナップショットから別クラスタへ復元）のオーケストレーション
- Kompox Operator（各ターゲットクラスタに配置）
  - CRD: KompoxApp、KompoxDisk、KompoxSnapshot（いずれも namespaced）
  - Reconcile:
    - Disk: クラウドディスク抽象、PV 静的プロビジョニング、Delete/Orphan ポリシーの順守
    - Snapshot: CSI VolumeSnapshot へ委譲可能なら委譲。未対応クラスタはクラウド API 直実装
    - App: compose→K8s マニフェスト適用、Ingress/Secret/PVC 連携、RWO ディスクの安全な切替

## Namespace とインストール指針

命名規約: Kompox における `k8s-` `kube-` 相当の略称として `k4x-` を使用する。

- `k4x-system`: Kompox Operator（Deployment/SA/RBAC/ConfigMap/Secret/Webhook など）
- `k4x-traefik`: Traefik（Deployment/Service/IngressClass/ConfigMap/TLS Secret 等）
- アプリ Namespace: KompoxApp/Disk/Snapshot、アプリの Service/PVC/Ingress
- ポリシー
  - RBAC は最小権限。監視対象 Namespace は限定（watchNamespaces またはラベル選別）
  - クラウド資格情報は必要な場合のみ Secret で参照（CSI/VolumeSnapshot で足りるなら不要）
  - NetworkPolicy により `k4x-system` と `k4x-traefik` の通信面を最小化

### アプリ Namespace の配置と自動生成

- 前提: KompoxApp/KompoxDisk/KompoxSnapshot（CRD）と、それが生成・管理する実体リソース（Deployment/StatefulSet/Service/PVC/Ingress など）は同一 Namespace に配置する。
  - 理由: 権限・所有・課金/Quota/NetworkPolicy を Namespace 単位で完結させ、PaaS のテナント境界を明瞭にするため。
- アプリ Namespace は衝突回避のため自動生成する。
  - 命名規則（例）: `k4x-<workspace>-app-<compact-id>`
    - `<workspace>` は論理的なプロジェクト/グループ名（省略可）
    - `<compact-id>` は `internal/naming` の Compact ID 生成規則を利用（英数小文字、固定長）
  - 生成時に付与する標準ラベル/アノテーション
    - `app.kubernetes.io/managed-by=kompox`
    - `kompox.dev/tenant=<workspace>`（任意）
    - `kompox.dev/app-id=<compact-id>`
  - 付随して設定するリソース分離
    - ResourceQuota/LimitsRange（任意）
    - NetworkPolicy（Ingress/Egress の最小化、必要に応じて `k4x-traefik` Namespace との疎通のみ許可）
    - Role/RoleBinding（Namespace 内の最小権限）
  - 削除ポリシー
    - Namespace を削除すると当該アプリの全リソースが一括で削除される。
    - データ保持のため、`KompoxDisk.spec.deletionPolicy=Orphan` を選ぶとクラウドディスクは残置し、再アタッチ可能。

## Provider Driver の選択

- 自動検出: CSIDriver 名、StorageClass、ノードラベル、クラウドメタデータ等から `aks/eks/gke/oke/k3s/auto` を推定
- 明示指定: 各 CRD の `spec.provider.type` で上書き。資格情報が必要な場合は Secret 参照
- 原則: 標準 CSI/VolumeSnapshot を優先し、足りない部分のみクラウド API を利用

## CRD（v1alpha1）最小スキーマ案

```yaml
apiVersion: k8s.kompox.dev/v1alpha1
kind: KompoxDisk
metadata:
  name: default
  namespace: app1
spec:
  provider:
    type: auto           # auto|aks|eks|gke|oke|k3s
    credentialsSecretRef: null
  size: 10Gi
  zone: "1"
  options:
    sku: PremiumV2_LRS
  deletionPolicy: Orphan  # Orphan|Delete
status:
  handle: ""              # クラウドディスクID（例: Azure Disk リソースID）
  pvName: ""
  conditions: []          # Ready/Progressing/Degraded
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
  snapshotHandle: ""      # VolumeSnapshot名 or クラウドSnapshotID
  conditions: []
---
apiVersion: k8s.kompox.dev/v1alpha1
kind: KompoxApp
metadata:
  name: app1
  namespace: app1
spec:
  provider:
    type: auto
  compose:
    configMapRef:
      name: app1-compose
      key: compose.yml
  ingress:
    className: traefik
    annotations: {}
    certResolver: staging
    rules:
      - name: main
        port: 3000
        hosts: ["gitea.example.com"]
  deployment:
    zone: "1"
  volumes:
    - name: default
      diskRef:
        name: default
status:
  ready: false
  ingressHosts: []
  conditions: []
```

## Reconcile 設計要点

- Disk
  - PV を静的作成（`spec.csi.volumeHandle` に `status.handle` を設定、`reclaimPolicy: Retain`）
  - 多重アタッチ防止。Condition で衝突を可視化
  - finalizer で Delete/Orphan を厳守。API コールは冪等・リトライ
- Snapshot
  - VolumeSnapshotClass があれば委譲。`ReadyToUse` を監視
  - 無い場合はクラウド API 直実装で `snapshotHandle` を反映。復元は新 Disk を派生（fromSnapshot）
- App
  - compose 変換 → Namespace 内に適用
  - RWO 切替は順序制御（ScaleDown → PVC 再 bind → ScaleUp）
  - `topology.kubernetes.io/zone` の `nodeAffinity` を自動付与しゾーン整合
  - Ingress は既存 Traefik 前提。アノテーション/hosts を反映

## Compose/Env ファイルの取り扱い

- compose.yml
  - 原則 ConfigMap に格納し、`KompoxApp.spec.compose.configMapRef` で参照する。
  - 理由: 宣言の可視性、監査容易性、差分管理（GitOps）に適しているため。
  - サイズ上限に注意（1 ConfigMap ≒ 1MiB 目安）。大規模な構成は分割またはGit/HTTP参照も検討。
- env_file（.env など、センシティブなキーを含み得る）
  - Secret として扱い、`stringData`/`data` に格納。`KompoxApp` の変換時に環境変数へ射影。
  - KMS/KeyVault 統合を将来検討（Secret Store CSI Driver 等）。当面は Kubernetes Secret を標準とする。
  - ローテーション: Secret のハッシュを Pod Template Annotation に反映し、ローリング更新を自動誘発（Operator 実装で対応）。
- その他参照ファイル（追加設定、証明書チェーン等）
  - 公開可の設定ファイルは ConfigMap、秘密情報は Secret へ。マウントまたは env で注入。

サンプル

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app1-compose
  namespace: app1
data:
  compose.yml: |
    version: "3.9"
    services:
      web:
        image: ghcr.io/example/app:1.0
        env_file:
          - /etc/kompox/env/app1.env
---
apiVersion: v1
kind: Secret
metadata:
  name: app1-env
  namespace: app1
type: Opaque
stringData:
  app1.env: |
    DATABASE_URL=postgres://...
    API_KEY=...
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
  files:
    mounts:
      - path: /etc/kompox/env/app1.env
        secretRef:
          name: app1-env
          key: app1.env
```

注意事項
- Secret 出力のログ/イベント/PR 差分に機密が混入しないようにし、トラブルシューティング時も値を表示しない。
- env_file を Pod にマウントする場合のファイルパーミッション（`defaultMode`）に注意。
- 大きなファイルやバイナリは ConfigMap/Secret に不向き。OCI Artifacts/Git/Blob Storage 等の利用を検討。

## 既知の落とし穴と対策

- VolumeSnapshot 未対応クラスタ: 自動でクラウド API モードにフォールバック
- ゾーン不整合: Disk/Deployment の zone 不一致は `Degraded` にして適用停止
- 複数クラスタ間移行: CLI 側で「detach → snapshot/restore → 新クラスタで Disk 作成 → App の `diskRef` 切替」をワークフロー化

## 段階的移行プラン

1. CRD 定義と Operator スケルトン（`controller-runtime/kubebuilder`）
2. AKS ドライバの実装（CSI 優先。未カバーを Azure API で補完）
3. CLI を CR 生成/適用のラッパーへ拡張（従来機能は互換維持）
4. E2E: 既存 Gitea サンプルを CRD ベースで再現し CI 化
5. 他クラウド（EKS/GKE/OKE/k3s）サポートを順次追加

## 品質と可観測性

- Conditions（Ready/Progressing/Degraded）、observedGeneration、Events を整備
- finalizer によるクラウド資産リーク防止
- 監視: Operator のメトリクス、イベント、ログ。将来は OpenTelemetry 連携を検討

## 今後の拡張の余地（Non-Goals からの発展）

- Crossplane/Cluster API との連携（将来、クラスタ LCM を外部委譲する場合に備えたポート設計）
- GitOps ツール（Argo CD 等）向けの `KompoxApp` ヘルパー（差分可視化や HealthCheck 拡張）
