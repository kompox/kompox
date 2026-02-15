---
id: 2026aa-kompox-box-update
title: Kompox Box Update
version: v1
status: draft
updated: 2026-02-15T05:03:16Z
language: ja
owner: yaegashi
adrs:
  - K4x-ADR-017
tasks:
  - 20260214b-box-kom-loader
  - 20260215a-standalone-box-manifest
---

# Plan: Kompox Box Update

## 目的

- App と Box の責務分離を明確化し、1 App = 1 Namespace の運用境界を維持しつつ、複数の Deployment-like unit を扱えるようにする。
- Docker Compose の挙動との整合を重視し、Compose の topology と Kubernetes の topology のずれを最小化する。
- Box を必須にせず、networkPolicy や ingress など個別設定が必要な場合のみ Box を追加できるようにする。
- Compose とは独立した作業環境 Pod (standalone) を Box としてデプロイできるようにする (既存の `kompoxops box` 相当)。

## 非スコープ

- Compose のすべての機能の完全互換 (Kubernetes へ 1:1 写像できないものは明示的にエラーとする)。
- 高度な NetworkPolicy (IPBlock, FQDN, L7 制御など)。
- 任意の topology 自動推論 (明示ルールとバリデーションを優先)。

## 用語

- App: テナント境界 (1 App = 1 Namespace)。共有インフラ (PV/PVC, 共通ポリシー) の単位。
- Box: App 配下の deployable unit。Kubernetes では Deployment/Pod を表す単位。
- Compose service: Docker Compose の `services.<name>`。
- Primary service: sidecar 同居の参照先となる Compose service。
- Sidecar service: `network_mode: service:<primary>` により primary と同居する Compose service。

## 既存設計との整合

- Box は Deployment-like unit として first-class に扱う (Box は App 配下の粒度)。
- ID は KOM の Resource ID (`metadata.annotations["ops.kompox.dev/id"]`) を canonical とする。
- 既存の v1 仕様では component ごとに Deployment 1個を生成し、Compose services を同一 Pod 内の複数コンテナとして扱う。

## 設計方針

### Box は optional

- Box を常に Compose services の個数ぶん作るのではなく、Box が定義されていない Compose services はデフォルトの component (`app`) に集約する。
- Box を定義した場合のみ、その Box に割り当てられた services は独立した Deployment (Pod) になる。

### Compose の topology を尊重する

- `network_mode` を sidecar 同居のシグナルとして利用する。
- Kompox が受理する `network_mode` は `service:<name>` のみとし、それ以外はエラーとする。
  - 目的: オリジナルの Docker Compose 環境でも同じ意味で動作することを重視する。

## リソースモデル

### App の責務

- Namespace を定める。
- PV/PVC を定義する (App 配下で共有できるボリューム定義)。
- Compose project を定義する (App.spec.compose)。
- App 全体の既定ポリシーを定義できる (例: 既定の NetworkPolicy 許可元など)。

### Box の責務

Box は 2 系統を同じ kind として表現する。

理由:
- Box は Kompox における deployable unit(=component)の単位であり、Compose 由来か Standalone かに関わらず運用対象の主語を揃える。
  - 例: componentName、NetworkPolicy の宛先(=PodSelector)、CLI の接続先選択。
- kind を分けずに済ませることで、共通の横断設定(例: NetworkPolicy)を同じ場所に集約できる。

1. Compose Box
   - App の Compose services の一部を独立した Deployment (Pod) として切り出す。
   - 切り出しにより、その Box だけに ingress/networkPolicy などの個別設定を適用できる。
2. Standalone Box
   - Compose とは独立した作業環境 Pod (例: SSH 可能な runner) を Box としてデプロイする。
   - 既存の `kompoxops box` で提供している作業環境用途を想定する。

Box の種類判定

- `spec.image` が存在する Box を Standalone Box とする。
- `spec.image` が存在しない Box を Compose Box とする。

## Compose Box のサービス割り当て

### 基本ルール

- Box は primary service を 1 つ割り当てる。
  - primary service は Box の `metadata.name` とする。
- Sidecar services は `network_mode: service:<primary>` の closure により自動的に同居させる。
- Box が定義されていない services は component `app` の Deployment に集約する。

注記:
- Box の種類は `spec.image` の有無により判定する。
  - `spec.image` が存在する Box を Standalone Box とする。
  - `spec.image` が存在しない Box を Compose Box とする。

### `network_mode: service:<name>` の解釈

- 未指定: 通常の service。
- `service:<primaryService>`: 当該 service は primaryService と同一 Pod (同一 Box) に同居する。
- 受理しない例 (エラー): `host`, `none`, `container:<id>`, 任意文字列。

### バリデーション

- Compose の service 名は DNS-1123 label に準拠していなければならない。
  - 理由: Kompox は Compose service 名を Kubernetes の識別子(例: コンテナ名や Service 名)として使用する。
- `network_mode: service:<name>` の参照先が存在しない場合はエラー。
- cycle はエラー。
  - 例: A -> service:B, B -> service:A
  - 例: A -> service:A
- 1 service は同時に複数 Box に割り当ててはならない (重複はエラー)。
- 同一 Box に同居する services 間で `containerPort` が衝突する構成はエラー。
  - Box が分離していれば同一 `containerPort` は許容され得る。

サービス割り当ての決定

- 入力:
  - App.spec.compose が表す Compose project に含まれる services 集合。
  - Box のうち `spec.image` を持たないもの(Compose Box)。
- ルール:
  - Compose Box は `metadata.name` で示される primary service を 1 つ持つ。
  - 各 primary service について `network_mode: service:<primary>` を辿って得られる closure を、その Box に同居させる。
  - sidecar service は明示的に Box を指定しない。
  - いずれの Compose Box にも割り当てられない services は component `app` に集約する。
- エラー:
  - 2つ以上の Box が同一 primary service を指定する。
  - Box が指定した primary service が存在しない。
  - Box が指定した primary service が sidecar service である(=自身が `network_mode: service:<x>` を持つ)。
  - ある service が異なる primary の closure に同時に含まれる。

Compose/Standalone の区別とバリデーション

- Box の種類は `spec.image` の有無で判定する。
  - `spec.image` が存在する Box を Standalone Box とする。
  - `spec.image` が存在しない Box を Compose Box とする。
- 上記の判定に従い、次のバリデーションを適用する。

共通:
- `metadata.name` は componentName であり、DNS-1123 label でなければならない。
- `metadata.name` は `app` を予約語とし、指定してはならない。
  - 理由: Box が定義されていない services を集約する既定 component が `app` のため。
- `spec.component` を指定する場合、値は `metadata.name` と一致しなければならない。
  - `spec.component` を省略する場合、componentName は `metadata.name` とみなす。

- Compose Box:
  - `metadata.name` は primary service 名であり、App.spec.compose が表す Compose project の service 名のいずれかと一致しなければならない。
  - `spec.image` `spec.command` `spec.args` を指定してはならない。
  - `spec.ingress` を指定してはならない。
- Standalone Box:
  - `spec.image` は必須。
  - `metadata.name` が Compose service 名と一致する場合はエラーとする。
  - 初期実装では ingress は対応しないため、`spec.ingress` を指定してはならない(将来拡張のため予約する)。

### Box 名 (componentName)

- Box の componentName は Box の `metadata.name` と一致する。
- Compose Box の `metadata.name` は primary service 名と一致する。
  - 目的: 宛先 Pod 選択 (NetworkPolicy.spec.podSelector) や運用対象選択を自然にする。
- componentName `app` は予約語であり、Box の `metadata.name` として使用してはならない。

## ingress と Service

- Converter は Box(component) ごとに Service(ingress) と Ingress を生成する。
- 生成される Kubernetes リソース名は `<appName>-<componentName>` とする(Deployment/Service/Ingress/NetworkPolicy など)。
- Ingress の設定は App.spec.ingress を一次入力とする。
  - Box ごとに ingress 設定を持たせず、App の ingress ルールを Box へ配賦する。

Ingress ルールの配賦

- App.spec.ingress.rules の各エントリは `port` (hostPort) で外部公開ポートを指定する。
- 各 hostPort に対し、Compose から ingress 対象の service を解決する。
  - `services.<svc>.ports` の `hostPort:containerPort` の hostPort が一致する service を 1つ選ぶ。
  - 一致が 0 件ならエラー。
  - 一致が 2 件以上ならエラー(hostPort の一意性が必要)。
- 解決した service が所属する Box を決定し、その Box の Service(ingress) と Ingress に当該ルールを配賦する。
  - service が Compose Box の closure に含まれる場合はその Box。
  - それ以外は component `app`。
- Service(ingress) は割り当てられた hostPort を列挙し、targetPort は対応する containerPort を指す。

注記:
- 既存 v1 の "同一 Pod 前提" 制約は Box 単位に局所化される。例えば `containerPort` 衝突エラーは同一 Box 内でのみ適用される。
- Standalone Box の ingress は本ドラフトでは扱わない(必要なら別仕様で ports と ingress の入力を定義する)。

## NetworkPolicy

### 適用範囲

- App.spec.networkPolicy は App 配下の component 全体に対する既定値を定義するための入力とする。
  - 対象は component `app` と、すべての Box(Compose Box / Standalone Box)の component を含む。
- Box.spec.networkPolicy は当該 Box(component) だけに適用するための入力とする。
- App と Box の両方に指定がある場合、許可ルールは additive にマージする。
  - 目的: App で共通の許可(例: ingress controller からの到達)を持ちつつ、Box で追加の許可を定義できるようにする。

マージ規則:
- 既定の拒否(default-deny)を前提とし、許可ルールは単純な union とする。
  - `baselineAllow ∪ App.spec.networkPolicy.ingressRules ∪ Box.spec.networkPolicy.ingressRules`
  - deny ルールは持たない(拒否は default-deny と、許可ルールの不足で表現する)。
  - 同一ルールの重複は許容し、実装は必要に応じて重複を除去してよい。

方針:
- ingress controller からの到達可否は、NetworkPolicy ではなく ingress/Service の有無で制御する。
  - 目的: NetworkPolicy の既定を default-deny にしつつ、外部公開の可否は公開設定(=Ingress を生成する入力)に寄せる。

注記:
- App.spec.networkPolicy も Box.spec.networkPolicy も未指定の場合でも、Kompox は NetworkPolicy を生成する。
  - 目的: 既定で namespace 境界の通信制御を有効にし、他 namespace からの到達を拒否する。
  - この既定挙動は ingress を対象とし、egress は既定で制御しない(=許可)。
- networkPolicy が指定された場合も、意図しない穴を避けるため、原則としてすべての component に対して NetworkPolicy を生成する。

### ベースライン許可

- ベースライン許可は、Kompox が NetworkPolicy を生成する場合に自動で追加する許可ルールである。
  - 目的: custom networkPolicy を記述しても、クラスタ運用上の前提(例: ingress controller 経由の到達)を破壊しない。
- NetworkPolicy が生成される場合、ベースライン許可は常に付与し、無効化しない。
  - Compose Box / Standalone Box を含むすべての component に適用する。
- ベースライン許可に含める ingress
  - 同一 namespace からの ingress を常に許可する。
  - `kube-system` namespace からの ingress を常に許可する。
  - ingress controller namespace (`kube.IngressNamespace(cluster)`) からの ingress を常に許可する。

注記:
- ingress controller namespace は `kube.IngressNamespace(cluster)` により決定される。
  - 既定値は `traefik`。
  - 変更する場合は Cluster の ingress 設定(例: `Cluster.spec.ingress.namespace`)で指定する。

### 外部公開の許可

- ingress controller からの ingress はベースライン許可により常に許可される。
- 外部公開の可否は、App.spec.ingress の配賦により当該 component に Ingress/Service が生成されるかどうかで決まる。

### 既定(未指定時)の拒否

- App.spec.networkPolicy と Box.spec.networkPolicy の両方が未指定の場合、Kompox は default-deny の ingress を実現する。
  - ベースライン許可(同一 namespace, kube-system, ingress controller namespace)以外の namespace からの ingress は拒否する。
  - これにより、networkPolicy を何も書かなくても、他 namespace からの到達は既定で拒否される。

### 宛先 Pod の絞り込み

- Kubernetes NetworkPolicy の宛先 Pod は `spec.podSelector` で表現する。
- Kompox は Box=Deployment を採用し、Box ごとに NetworkPolicy を生成することで宛先 Pod を component ラベル(`app.kubernetes.io/component`)で絞り込む。

### Box ごとの NetworkPolicy

- Box に networkPolicy 設定がある場合、その Box の Pod だけを対象にする NetworkPolicy を生成する。
- 設定が未指定の場合でも App.spec.networkPolicy が指定されている場合は、その既定値を適用する。

### 既定の考え方

- Compose Box の想定ユースケースには、機密性の高い service を分離し、到達範囲を強く絞ることが含まれる。
  - 例: postgres はどの port も外部から許可したくない。
- このため、networkPolicy を有効化した場合の既定は default-deny(明示許可のみ)を基本とする。
  - 具体例: postgres の Box に対しては ingress ルールを空にした NetworkPolicy を生成し、すべての ingress を拒否する。
  - 必要があれば postgres への到達は `--component app` 相当の component など、最小限の許可元と port だけを許可する。

### 追加許可ルール

- 追加許可は additive とする。
- 許可元は namespaceSelector を指定可能。
- 許可先は protocol と port のリストを指定可能。
- ports 省略は "すべてのポート" を意味する (Kubernetes の NetworkPolicy 仕様に従う)。

## Standalone Box

- Compose とは独立した Deployment を App の Namespace に作成する。
- 目的は作業環境 (VM や Codespaces のような用途) の提供であり、アプリ本体の topology とは独立してよい。
- 既存の `kompoxops box` の操作対象として維持する。
- Standalone Box は `spec.image` などのコンテナ実行設定を持つ。

## アクセスと Pod 選択

- `kompoxops app tunnel` `kompoxops app exec` `kompoxops app logs` は単一 Pod を選んで操作する。
- Box を導入して component が増えると、CLI はどの component に接続するかを指定できる必要がある。
- 宛先 Pod は componentName を主キーとして選択できるべきである。
  - 理由: Box ごとの NetworkPolicy と運用対象の選択を同じ単位に揃える。
- 現状の CLI
  - `kompoxops app tunnel` は `--component` を持つが、指定できる値は `app|box` であり Compose Box を名前で選べない。
  - `kompoxops app exec` `kompoxops app logs` `kompoxops app status` は component を `app` に固定している。
- 提案するセレクタの入力
  - `--component <componentName>`: 接続先 component を指定する。既定は `app`。
  - `--pod <podName>`: Pod を明示指定し、replicas>1 や rollout 中の曖昧さを解消する。
  - `--container <containerName>`: Pod 内のコンテナを明示指定する(複数コンテナがある場合に必要)。
  - `--selector <labelSelector>`: デバッグ用途の上級オプション。通常は利用しない。

選択ポリシー

- `--pod` が指定されていない場合、`--component` (または `--selector`) で候補 Pod を列挙する。
  - 候補が 1 つならそれを使用する。
  - 候補が複数なら、それらを列挙して終了し、`--pod` 指定を促す。
    - 列挙には判断を支援する情報を付加する(例: Ready かどうか、作成からの経過秒数など)。

候補列挙の出力例

```text
multiple pods matched: specify --pod

NAME                                 READY   AGE
example-web-6b7c8d9f7d-abcde         true    12s
example-web-6b7c8d9f7d-fghij         false   8s
```
- `--container` が指定されていない場合、対象 Pod のコンテナ候補を列挙する。
  - 候補が 1 つならそれを使用する。
  - 候補が複数なら、それらを列挙して終了し、`--container` 指定を促す。

影響を受ける CLI の例

- app: `tunnel` `exec` `logs` (単一 Pod に接続するため)
- app: `status` (現在は最初の Deployment/Pod を採用するため、Box 単位の表示を追加するなら `--component` が必要)
- box: `exec` `ssh` `sync` `scp` (現在は component=box を前提に単一 Pod へ接続するため)

## kompoxops app と kompoxops box の役割

`kompoxops app` の役割

- App のデプロイと運用を扱う(Compose project を一次入力とする)。
- App 配下の任意 component に対して、component 単位の汎用運用操作を提供する。
  - 例: `exec` `logs` `tunnel` `status`
- `kompoxops app deploy` `kompoxops app destroy` は Standalone Box のライフサイクル操作(作成/削除)を対象にしない。
- component 選択は将来的に `--component` を基本とし、曖昧な場合は `--pod` で決定できる。

`kompoxops box` の役割

- Standalone Box のライフサイクル管理(Compose とは独立した作業環境)を扱う。
  - 例: `deploy` `destroy` `status` (正規経路)
- 作業用途の ergonomics を提供する(app 汎用コマンドの糖衣)。
  - 例: `ssh` `scp` `rsync`

境界

- Compose Box は本質的には App の component の一つであり、運用操作は `kompoxops app --component <box>` に寄せる方針とする。
  - 現状は `kompoxops app exec` `kompoxops app logs` `kompoxops app status` が component を固定しているため、当面は `kompoxops box` を併用する。
- Standalone Box は Box 固有の入力(例: SSH 公開鍵注入)を持つため `kompoxops box` の責務に残す。

## CLI 整理案

原則

- `kompoxops app` は App 配下の任意 component を対象にできる汎用コマンド群を持つ方針とする。
- `kompoxops box` は Standalone Box のライフサイクル管理と、人間向けの糖衣にスコープを絞る。

`kompoxops box` に残すもの

- `deploy` `destroy` `status`
  - Standalone Box の作成と削除は Box 固有の入力(例: SSH 公開鍵の注入)を持つ。
  - この責務は `kompoxops app deploy` と混ぜない方が分かりやすい。
- `ssh` `scp` `rsync`
  - これらは実体としては port-forward とクライアントコマンド実行の合成であり、作業用途の ergonomics が価値になる。
  - `app tunnel` で代替は可能だが、毎回 `-L` や転送設定を組むのは利用者負担が大きい。

`kompoxops app` に寄せるもの

- `tunnel` `exec` `logs`
  - Box を含む任意 component を対象にできるようにし、Box model の増加に備える。
  - そのために `--component` `--pod` などのセレクタ入力を備える必要がある。

`kompoxops box` から省けるか

- `box exec` は将来的に `app exec --component <boxComponent> --container <boxContainer>` で置換可能になる。
- ただし現状の `app exec` は component を固定で `app` としているため、すぐには代替できない。
- `box ssh` `box scp` `box rsync` は `app tunnel` だけでも実現できるが、利用者体験のために当面残すのが自然である。
  - `app tunnel --component <box> -p :22` を起点に `ssh -p <localPort> ...` を手作業で組むことはできる。
  - それを CLI が肩代わりするのが `box ssh` 系の価値になる。

## KOM スキーマ案 (例)

注記:
- 現行の CRD (ops.kompox.dev/v1alpha1) の BoxSpec は placeholder であり、`spec.component` のみを持つ。
  - 本ドキュメントの Compose/Standalone の入力や NetworkPolicy/Ingress の扱いは、BoxSpec 拡張の設計ドラフトである。

### Box (初期実装/将来予約が分かるスケルトン)

現行 CRD 準拠の最小例 (placeholder)

```yaml
apiVersion: ops.kompox.dev/v1alpha1
kind: Box
metadata:
  name: web
  annotations:
    ops.kompox.dev/id: /ws/ws1/prv/prv1/cls/cls1/app/example/box/web
spec:
  component: web
```

ドラフト上の BoxSpec スケルトン (初期実装で採用するもの/将来実装に予約するもの)

```yaml
apiVersion: ops.kompox.dev/v1alpha1
kind: Box
metadata:
  name: web
  annotations:
    ops.kompox.dev/id: /ws/ws1/prv/prv1/cls/cls1/app/example/box/web
spec:
  # 初期実装で採用
  # - componentName は `metadata.name` を canonical とする。
  # - `spec.component` は現行 CRD 互換のため残す場合がある(値は `metadata.name` と一致させる)。
  component: web

  # 初期実装で採用(Compose Box/Standalone Box 共通)
  networkPolicy:
    ingressRules: []

  # 初期実装で採用(Standalone Box のみ)
  image: ghcr.io/kompox/kompox/box
  command: ["sleep"]
  args: ["infinity"]

  # 将来実装に予約(初期実装では指定禁止)
  ports: []
  ingress:
    certResolver: production
    rules:
      - name: http
        port: 8080
        hosts:
          - standalone-web.example.com
```

### App (Compose project)

```yaml
apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  name: example
  annotations:
    ops.kompox.dev/id: /ws/ws1/prv/prv1/cls/cls1/app/example
spec:
  compose:
    services:
      web:
        image: nginx:stable
        ports:
          - "8080:80"
      metrics:
        image: prom/prometheus
        network_mode: service:web
```

### Compose Box (web を primary として切り出す)

```yaml
apiVersion: ops.kompox.dev/v1alpha1
kind: Box
metadata:
  name: web
  annotations:
    ops.kompox.dev/id: /ws/ws1/prv/prv1/cls/cls1/app/example/box/web
spec:
  networkPolicy:
    ingressRules:
      - from:
          - namespaceSelector:
              matchLabels:
                kubernetes.io/metadata.name: monitoring
        ports:
          - protocol: TCP
            port: 9090
```

### Standalone Box (作業環境)

```yaml
apiVersion: ops.kompox.dev/v1alpha1
kind: Box
metadata:
  name: standalone
  annotations:
    ops.kompox.dev/id: /ws/ws1/prv/prv1/cls/cls1/app/example/box/standalone
spec:
  image: ghcr.io/kompox/kompox/box
  command: ["sleep"]
  args: ["infinity"]
```

### Standalone Box (作業環境 + ingress: 将来案)

注記:
- Compose Box では `spec.ingress` を指定してはならない。
- Standalone Box の ingress は本ドラフトでは非スコープだが、将来 `spec.ports` と `spec.ingress` を導入する場合の記述例を示す。

```yaml
apiVersion: ops.kompox.dev/v1alpha1
kind: Box
metadata:
  name: standalone-web
  annotations:
    ops.kompox.dev/id: /ws/ws1/prv/prv1/cls/cls1/app/example/box/standalone-web
spec:
  image: ghcr.io/kompox/kompox/box

  ports:
    - name: http
      port: 8080
      containerPort: 8080

  ingress:
    certResolver: production
    rules:
      - name: http
        port: 8080
        hosts:
          - standalone-web.example.com
```

## 計画 (チェックリスト)

本セクションは [K4x-ADR-017] の Rollout から移管した実装計画の正本であり、今後は進捗に応じて更新する。

- [x] Phase 1: 本ドキュメントを実装の設計正本として扱う。
- [x] Phase 2: BoxSpec を placeholder から最小 v1 フィールドへ拡張する。
- [x] Phase 3: ローダー時バリデーションと、Compose services → component の決定規則を実装する。
- [ ] Phase 4: Standalone Box を優先して K8s Manifest 化できる状態にする。
  - [ ] Box を domain model/repository で扱えるようにする。
  - [ ] `spec.image` を持つ Box(Standalone Box)を converter 入力へ反映する。
- [ ] Phase 5: Standalone Box の deploy/destroy は `kompoxops box` を維持しつつ、`kompoxops app` 側で component 単位の適用/運用を可能にする。
  - [ ] Deployment/Service/NetworkPolicy の component 出力を Standalone Box まで拡張する。
  - [ ] `kompoxops app deploy/destroy` が Standalone Box の作成/削除を行わない境界を実装・検証する。
  - [ ] Standalone Box のライフサイクル操作は `kompoxops box deploy/destroy` を正規経路として維持する。
  - [ ] App 既定と Box 個別の NetworkPolicy 追加許可を最小実装でマージする。
- [ ] Phase 6: CLI の単一ターゲット操作を `--component/--pod/--container` へ統一する。
  - [ ] `kompoxops app` 系コマンドで Box component を明示選択できるようにする。
  - [ ] 既定値 `app` を維持して後方互換を保つ。
- [ ] Phase 7: Compose Box の services 割り当てを実装する。
  - [ ] primary service + `network_mode: service:<name>` closure による同居解決を実装する。
  - [ ] cycle/重複割り当て/primary 不正参照などのエラー検出を実装する。
- [ ] Phase 8: Ingress ルールの Box 配賦を実装する。
  - [ ] hostPort から service を一意解決し、所属 component へ配賦する。
  - [ ] component ごとの Service(ingress)/Ingress 生成を整備する。
- [ ] Phase 9: NetworkPolicy の component 全面適用を実装する。
  - [ ] baselineAllow + App + Box の additive マージを全 component へ適用する。
  - [ ] default-deny 前提と ingress 制御方針(公開可否は ingress/Service 入力)を実装に反映する。
- [ ] Phase 10: CLI 境界整理と移行完了を実施する。
  - [ ] Compose Box 運用を `kompoxops app` 側へ寄せ、`kompoxops box` は Standalone Box の deploy/destroy と作業系サブコマンド中心へ整理する。
  - [ ] 互換モード/移行ガイド/回帰テストを整備し、段階的移行を完了する。

## 互換性と移行

- Box を定義しない App は、既定 component `app` の単一デプロイを継続する。
- `kompoxops app deploy/destroy` は Standalone Box の deploy/destroy を実行しない。
- 移行期間中の Standalone Box 操作は `kompoxops box` を維持し、Compose 由来 component は `kompoxops app` の selector で操作する。
- Box を定義しない場合の既定挙動は現行 v1 の "component=app に集約" を維持する。
- Box を追加した場合のみ topology が変化するため、段階的に Box 導入が可能である。

## 参照

- [20260214b-box-kom-loader]
- [Kompox-KubeConverter]
- [Kompox-CLI]
- [K4x-ADR-008]
- [K4x-ADR-009]
- [K4x-ADR-017]
- [ops/v1alpha1 types.go]
- [Kubernetes NetworkPolicy]
- [Compose specification]

[20260214b-box-kom-loader]: ../tasks/2026/02/14/20260214b-box-kom-loader.ja.md
[Kompox-KubeConverter]: ./Kompox-KubeConverter.ja.md
[Kompox-CLI]: ./Kompox-CLI.ja.md
[K4x-ADR-008]: ../adr/K4x-ADR-008.md
[K4x-ADR-009]: ../adr/K4x-ADR-009.md
[K4x-ADR-017]: ../adr/K4x-ADR-017.md
[ops/v1alpha1 types.go]: ../../config/crd/ops/v1alpha1/types.go
[Kubernetes NetworkPolicy]: https://kubernetes.io/docs/concepts/services-networking/network-policies/
[Compose specification]: https://compose-spec.io/
