---
id: 2025-11-09-aks-cluster-fix
title: AKS Driver - Cluster 関連の不具合修正
status: done
updated: 2025-11-09
language: ja
owner: yaegashi
---
# Task: AKS Driver - Cluster 関連の不具合修正

## 目的

- AKS Driver の ClusterInstall() における以下の不具合を修正する:
  1. ACR への AcrPull ロール割り当てで誤って Cluster Identity を使用している (正しくは Kubelet Identity を使用すべき)
  2. Traefik Ingress Controller でクライアントのソース IP アドレスが保持されない (アクセスログにクラスタ内 IP が記録される)

## スコープ / 非スコープ

- In:
  - Step 4 (DNS Zone) と Step 5 (ACR) で使用する Principal ID の明確化と修正
    - DNS Zone Contributor: AKS Cluster Identity (`outputAksClusterPrincipalID`)
    - AcrPull: AKS Kubelet Identity (`outputAksKubeletPrincipalID`)
  - 変数名を `aksClusterPrincipalID` と `aksKubeletPrincipalID` に明確化
  - コメントとログメッセージを更新して Identity の用途を明示
  - Traefik Helm values に `service.externalTrafficPolicy: Local` を追加
  - クライアントソース IP 保持のための設定追加
- Out:
  - Bicep テンプレート側の変更 (outputs は既に両方の Principal ID を出力している前提)
  - Traefik の DaemonSet 化や Pod 配置戦略の変更 (既存の Deployment を維持)
  - PROXY Protocol やその他の L7 設定 (不要であることを確認済み)

## 仕様サマリ

### 1. ACR ロール割り当ての修正

Azure AKS では、以下のように Identity を使い分ける必要がある:

- **Cluster Identity** (Control Plane):
  - コントロールプレーン操作 (DNS Zone への DNS レコード更新など)
  - 使用する deployment output: `outputAksClusterPrincipalID`

- **Kubelet Identity** (Data Plane):
  - データプレーン操作 (ACR からのイメージプル、ディスクアタッチなど)
  - 使用する deployment output: `outputAksKubeletPrincipalID`

現在の実装では、Step 5 で誤って `aksPrincipalID` (Cluster Identity) を使用している。

### 2. Traefik でのソース IP 保持

Azure Load Balancer (L4) 経由でクライアントのソース IP を保持するには:

- `service.spec.externalTrafficPolicy: Local` を設定
  - Traefik Helm Chart では `service.spec` 配下に設定する必要がある
  - `values["service"]["spec"]["externalTrafficPolicy"] = "Local"` として設定
  - kube-proxy の SNAT をスキップ
  - トラフィックを受信したノード上の Pod のみにルーティング
  - クライアントの実際の IP アドレスが Pod まで届く

トレードオフ:
- 負荷分散がノード単位になる (Pod が存在しないノードはトラフィックを受けない)
- 各ノードに Pod が均等に配置されていれば問題は最小限

## 計画 (チェックリスト)

- [x] `adapters/drivers/provider/aks/cluster.go` の `ClusterInstall()` を修正:
  - [x] Step 4: 変数名を `aksClusterPrincipalID` に変更、コメント更新
  - [x] Step 5: `outputAksKubeletPrincipalID` から `aksKubeletPrincipalID` を取得
  - [x] Step 5: `ensureAzureContainerRegistryRoles()` に `aksKubeletPrincipalID` を渡す
  - [x] Step 5: ログメッセージを "AKS kubelet principal ID" に更新
  - [x] Step 6: Traefik Helm mutator に `service.externalTrafficPolicy: Local` を追加
- [x] E2E テスト実行と結果確認:
  - [x] 既存の AKS E2E テストで ACR からのイメージプルが成功することを確認
  - [x] Traefik 経由のアクセスでソース IP が正しく記録されることを確認
- [x] ドキュメント/索引:
  - [x] `make gen-index` で索引を更新

## テスト

### ユニット
- 省略 (実装コストに見合わないため)
- Principal ID の取得と Helm values の設定は E2E テストで検証

### E2E / スモーク
- ACR からのイメージプル:
  - Private ACR に存在するイメージを使用するアプリをデプロイ
  - Pod が正常に起動し、イメージプルに成功することを確認
- ソース IP の記録:
  - Traefik 経由で HTTP リクエストを送信
  - Traefik のアクセスログに実際のクライアント IP が記録されることを確認
  - クラスタ内 IP (10.x.x.x など) ではないことを検証

## 受け入れ条件

- `kompoxops cluster install` 実行時:
  - Step 4 で DNS Zone Contributor ロールが AKS Cluster Identity に割り当てられる
  - Step 5 で AcrPull ロールが AKS Kubelet Identity に割り当てられる
  - 両ステップのログメッセージで使用している Identity の種類が明確に区別できる
- Traefik Ingress Controller 経由のアクセス:
  - アクセスログに実際のクライアント IP アドレスが記録される
  - クラスタ内 IP ではなく、外部クライアントの実 IP が表示される
- 既存の E2E テストが全て成功する (リグレッションなし)

## メモ

### なぜ externalTrafficPolicy: Local が必要か

Azure Load Balancer は L4 (TCP/UDP) レベルで動作するため:

1. **externalTrafficPolicy: Cluster** (デフォルト):
   - kube-proxy が SNAT を実行
   - 送信元 IP がクラスタ内 IP に変換される
   - Traefik は実際のクライアント IP を認識できない

2. **externalTrafficPolicy: Local**:
   - kube-proxy が SNAT をスキップ
   - 送信元 IP が保持される
   - Traefik が実際のクライアント IP を認識し、アクセスログに記録できる

### kube-proxy の動作について

- kube-proxy は各ノードで DaemonSet として動作
- L4 レベル (iptables/IPVS) でトラフィックを処理
- HTTP を認識しないため、X-Forwarded-For ヘッダーは設定できない
- Direct Server Return (DSR) は Kubernetes では未実装

### 負荷分散の偏りについて

`externalTrafficPolicy: Local` では:
- Azure Load Balancer がノード単位でトラフィックを分散
- 各ノードの kube-proxy はローカルの Pod のみにルーティング
- Pod が存在しないノードは Health Check で除外される

対策 (本タスクのスコープ外、将来の改善として検討):
- DaemonSet を使用して各ノードに必ず1 Pod を配置
- Pod Topology Spread Constraints で均等配置
- Pod Anti-Affinity でノード間分散

## 進捗

- 2025-11-09: タスク作成、問題の整理と修正方針の確定
- 2025-11-09: 実装完了
  - Step 4 で DNS Zone Contributor ロールを AKS Cluster Identity に割り当てるように変数名とコメントを明確化
  - Step 5 で AcrPull ロールを AKS Kubelet Identity に割り当てるように修正
  - Step 6 で Traefik に `service.externalTrafficPolicy: Local` を設定してクライアントソース IP を保持
  - ビルドとユニットテスト成功を確認
- 2025-11-09: 初回デプロイで externalTrafficPolicy が反映されないことを発見
  - Traefik Helm Chart の公式 values.yaml を確認
  - `externalTrafficPolicy` は `service.spec` 配下に設定する必要があることが判明
  - 実装を修正: `values["service"]["spec"]["externalTrafficPolicy"] = "Local"`
  - 再ビルド・再テスト成功
- 2025-11-09: E2E テスト完了、全ての受け入れ条件を満たすことを確認
  - AKS クラスタへの再デプロイ成功
  - Traefik アクセスログで実際のクライアント IP (217.178.137.20) が記録されることを確認
  - クラスタ内 IP (10.224.0.x) ではなく外部クライアントの実 IP が表示されることを確認
  - タスク完了

## 参考

- [Azure AKS Managed Identities]
- [Kubernetes Service External Traffic Policy]
- [Traefik Helm Chart Configuration]

[Azure AKS Managed Identities]: https://learn.microsoft.com/azure/aks/use-managed-identity
[Kubernetes Service External Traffic Policy]: https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/#preserving-the-client-source-ip
[Traefik Helm Chart Configuration]: https://github.com/traefik/traefik-helm-chart/blob/master/traefik/values.yaml
