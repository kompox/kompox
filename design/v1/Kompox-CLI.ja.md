---
id: Kompox-CLI
title: Kompox PaaS CLI
version: v1
status: out-of-sync
updated: 2025-09-26
language: ja
---

# Kompox PaaS CLI

## Overview

次のようなコマンドを実装する。

|コマンド名|説明|
|-|-|
|kompoxops|Kompox 仕様のクラウドリソースデプロイ・運用ツール<br>Kompox PaaS とは独立した設定ファイル `kompoxops.yml` を読み取って動作する CLI|
|kompoxsvc|Kompox PaaS REST API サーバと管理ツール<br>REST API サーバは `kompoxsvc server` で起動するコンテナWebアプリ|
|kompox|Kompox PaaS REST API クライアント CLI|

## kompoxops

### kompoxops Overview

kompoxops は Kompox PaaS 準拠のデプロイツールである。

```
kompoxops init              設定ファイルの雛形作成
kompoxops cluster           クラスタ操作
kompoxops app               アプリ操作
kompoxops secret            シークレット操作
kompoxops disk              ディスク操作
kompoxops snapshot          スナップショット操作
kompoxops admin             管理ツール
```

共通オプション

- `--db-url <URL>` 永続化DBの接続URL。環境変数 KOMPOX_DB_URL で指定可能。

### kompoxops.yml

kompoxops は永続化DBなしで動作することができる。

リソース定義ファイル kompoxops.yml をグローバルオプションで `--db-url file:kompoxops.yml` のように指定すると、
リソース定義をインメモリDBに読み込み、それに対して Kompox PaaS と同様の操作を行うことができる。

kompoxops.yml には service, provider, cluster, app の各定義が 1 つずつ含まれる。
それらは kompoxops の memory データベースストアに読み込まれて
自動的に app → cluster → provider → service の依存関係が設定される。

kompoxops.yml の例:

```yaml
version: v1
service:
  name: ops
provider:
  name: aks1
  driver: aks
  settings:
    AZURE_AUTH_METHOD: azure_cli
    AZURE_SUBSCRIPTION_ID: 34809bd3-31b4-4331-9376-49a32a9616f2
    AZURE_LOCATION: japaneast
cluster:
  name: cluster1
  existing: false
  ingress:
    controller: traefik
    namespace: traefik
    domain: ops.kompox.dev
  settings:
    AZURE_RESOURCE_GROUP_NAME: rg-cluster1
app:
  name: app1
  compose:
    services:
      app:
        image: ghcr.io/kompox/app
        environment:
          TZ: Asia/Tokyo
        ports:
          - "8080:80"
          - "8081:8080"
        volumes:
          - ./data/app:/data
        x-kompox:
          resources:
            cpu: 100m
            memory: 256Mi
          limits:
            cpu: 200m
            memory: 512Mi
      postgres:
        image: postgres
        environment:
          POSTGRES_PASSWORD: secret
        volumes:
          - db/data:/var/lib/postgresql/data
        x-kompox:
          resources:
            cpu: 100m
            memory: 256Mi
          limits:
            cpu: 200m
            memory: 512Mi
  ingress:
    - name: main
      port: 8080
      hosts: [www.custom.kompox.dev]
    - name: admin
      port: 8081
      hosts: [admin.custom.kompox.dev]
  volumes:
    - name: default
      size: 32Gi
    - name: db
      size: 64Gi
```

### kompoxops cluster

K8s クラスタ操作を行う。

```
kompoxops cluster provision --cluster-name <clusterName>     Cluster リソース準拠の K8s クラスタを作成開始 (existingがfalseの場合)
kompoxops cluster deprovision --cluster-name <clusterName>   Cluster リソース準拠の K8s クラスタを削除開始 (existingがfalseの場合)
kompoxops cluster install --cluster-name <clusterName>       K8s クラスタ内のリソースをインストール開始
kompoxops cluster uninstall --cluster-name <clusterName>     K8s クラスタ内のリソースをアンインストール開始
kompoxops cluster status --cluster-name <clusterName>        K8s クラスタのステータスを表示
kompoxops cluster kubeconfig --cluster-name <clusterName>    kubectl 用 kubeconfig を取得/統合
```

共通オプション

- `--cluster-name | -C` クラスタ名を指定 (デフォルト: kompoxops.yml の cluster.name)

provision/deprovision コマンドは service/provider/cluster リソースの設定に従って K8s クラスタを作成・削除する。
既存のクラスタを参照する場合は cluster.existing を true に設定する。
cluster.existing が true の場合 provision/deprovision は常に成功を返す。

install/uninstall コマンドは K8s クラスタ内のリソースを作成・削除する。
Namespace や Traefik Proxy などの Ingress Controller を含む。

クラスタの作成方法や接続方法は cluster.settings で設定する。
設定内容は provider.driver により異なる。

status コマンドにより K8s クラスタと内部リソースの状態について次の項目が取得できる。

```
existing: bool          cluster.existing の設定値
provisioned: bool       K8s クラスタが存在するとき true (existingがtrueの場合も実際に存在するか調べる)
installed: bool         K8s クラスタ内のリソースが存在するとき true
ingressGlobalIP: string クラスタ Ingress/LoadBalancer のグローバル IP アドレス(存在する場合)
ingressFQDN: string     クラスタ Ingress/LoadBalancer の FQDN(存在する場合)
```

provision/deprovision/install/uninstall は status により実行可否が変わる。
これらのコマンドは時間のかかる操作であるため実行すると即座に終了する。
冪等性を持つため複数回実行してもエラーにはならない。

|コマンド|エラー発生条件|実行内容|
|-|-|-|
|provision||existing=trueなら何もしない<br>provisined=falseならK8sクラスタ作成開始|
|deprovision|installed=true|existing=trueなら何もしない<br>provisioned=trueならK8sクラスタ削除開始|
|install|provisioned=false|installed=falseならK8sクラスタ内リソース作成開始|
|uninstall||installed=trueならK8sクラスタ内リソース削除開始|

#### kompoxops cluster provision

Cluster リソース準拠の AKS/K8s クラスタを作成開始します(idempotent)。`existing=true` の場合は何もしません。

#### kompoxops cluster deprovision

対象クラスタのリソースグループを削除します(idempotent)。`installed=true` の場合はエラーです。

#### kompoxops cluster install

Ingress Controller などのクラスタ内リソースをインストールします。`provisioned=false` の場合はエラーです。

#### kompoxops cluster uninstall

クラスタ内リソースをアンインストールします(best-effort)。

#### kompoxops cluster status

クラスタの `existing`/`provisioned`/`installed` に加えて Ingress のグローバル IP/FQDN を JSON で表示します(利用可能な場合)。

#### kompoxops cluster kubeconfig

Provider Driver からクラスタの kubeconfig(管理者資格)を取得し、標準出力/ファイル保存/既存 kubeconfig への統合を行う。

主なオプション:

- `-o, --out FILE` 出力先(`-` は標準出力。stdout は必ず明示的に `-o -` を指定)
- `--merge` 既存 kubeconfig に統合(既定: `~/.kube/config`)
- `--kubeconfig PATH` 統合先ファイル(既定: `~/.kube/config`)
- `--context NAME` 生成する context 名(既定: `kompoxops-<clusterName>`)
- `--namespace NS` context のデフォルト namespace。未指定で設定ファイルが読み込まれている場合は Service/Provider/Cluster/App 名から導出された内部既定値が自動で入る。
- `--set-current` 統合後に current-context を新しい context に設定
- `--force` 同名エントリがある場合に上書き(未指定時は `-1`, `-2` のように自動ユニーク化)
- `--temp` セキュアな一時ファイルに保存してパスを出力
- `--print-export` `export KUBECONFIG=...` を出力(`--out` か `--temp` と併用)
- `--format yaml|json` 標準出力時のフォーマット(既定: `yaml`)
- `--dry-run` 統合時の差分を要約表示のみ(書き込み無し)
- `--cred admin|user` 要求する資格(現状 `admin` のみサポート)
- `--timeout DURATION` 取得のタイムアウト(既定: `2m`)

注意:

- 何も出力先を指定しない(`--merge` なし、`--temp` なし、`--out` 未指定)の場合はエラーになります。
- stdout 出力は `-o -` を明示したときのみ行われます。

使用例:

```
# 標準出力へ(明示)
kompoxops cluster kubeconfig -C mycluster -o -

# 一時ファイル化と KUBECONFIG エクスポート
eval "$(kompoxops cluster kubeconfig -C mycluster --temp --print-export)"

# 既存へ統合して current-context を切替え
kompoxops cluster kubeconfig -C mycluster --merge --set-current

# context と namespace を指定して保存
kompoxops cluster kubeconfig -C mycluster --merge --context kompox/prod --namespace staging

# namespace 自動設定の例 (config から導出される既定 namespace を利用)
kompoxops cluster kubeconfig -C cluster1 --merge --set-current
```

### kompoxops app

アプリの操作を行う。

```
kompoxops app validate --app-name <appName>
kompoxops app deploy   --app-name <appName> [--bootstrap-disks]
kompoxops app destroy  --app-name <appName>
kompoxops app status   --app-name <appName>
kompoxops app exec     --app-name <appName> -- <command> [args...]
```

共通オプション

- `--app-name | -A` アプリ名を指定 (デフォルト: kompoxops.yml の app.name)

validate コマンドは app.compose の内容を検証し Kompose により K8s マニフェストに変換する。
YAML 構文エラーや制約違反が検出された場合はエラーを返す。

- `--out-compose FILE` を指定すると正規化した Docker Compose の YAML ドキュメントを出力する (`-` は stdout)
- `--out-manifest FILE` を指定すると K8s マニフェストの YAML ドキュメントを出力する (`-` は stdout)

#### kompoxops app validate

Compose の検証と K8s マニフェスト生成を行います。`--out-compose`/`--out-manifest` でファイル出力可能です。

#### kompoxops app deploy

検証・変換済みのリソースを対象クラスタに適用します(冪等)。

オプション:

- `--bootstrap-disks` 全 app.volumes で Assigned ディスクが 0 件の場合に限り、各ボリューム 1 件ずつ新規ディスクを自動作成してからデプロイを続行する。部分的に一部ボリュームのみ未初期化 (Assigned=0) / 他は 1 件以上 Assigned の混在状態や、未割当ディスクのみが残っている不整合状態はエラー。

ディスク初期化挙動 (概要):
1. 判定: 全ボリュームで Assigned=0 ?
2. YES -> `disk create --bootstrap` 相当の一括作成を実行 (各ボリューム 1 件)。
3. NO かつ 全ボリュームで Assigned=1 -> そのまま続行 (no-op)。
4. それ以外 -> エラー終了 (ユーザーが手動で assign/delete を整理して再実行)。
5. 以後、通常の manifest 生成・適用フェーズで "各ボリューム Assigned=1" を前提とする。

使用例:
```bash
# 初回: まだどのボリュームにもディスクが無い -> 自動生成してデプロイ
kompoxops app deploy --bootstrap-disks

# 2 回目以降: 既に 1 件ずつあるので作成せず適用のみ
kompoxops app deploy --bootstrap-disks
```

#### kompoxops app destroy

デプロイ済みリソースをクラスタから削除します (冪等)。

- 次のラベルがついたリソースのみ削除する
  - `app.kubernetes.io/instance: app1-inHASH`
  - `app.kubernetes.io/managed-by: kompox`
- 既定で Namespace 以外のリソース(PV/PVC を含む)を削除
- `--delete-namespace` を指定すると Namespace リソースも削除

備考:
- PV/PVC を削除しても、StorageClass/PV の ReclaimPolicy が Retain の場合はクラウドディスク本体は保持されます。

#### kompoxops app status

アプリの状態を JSON で表示します。Namespace、デプロイ状況、Ingress のホスト名一覧などを返します。

出力例

```json
{
  "app_id": "...",
  "app_name": "app1",
  "cluster_id": "...",
  "cluster_name": "cluster1",
  "namespace": "kompox-app1-<idHASH>",
  "deployed": true,
  "ingress_hosts": [
    "app1-<idHASH>-8080.example.com",
    "www.custom.example.com"
  ]
}
```

備考
- `namespace` はアプリの実リソースが存在する Kubernetes Namespace を示します。
- `ingress_hosts` には `app.ingress.rules.hosts` で指定したカスタムドメインに加え、`cluster.ingress.domain` が設定されている場合は `<appName>-<idHASH>-<port>.<domain>` の自動生成ドメインが含まれます。

#### kompoxops app exec

アプリの Namespace 内で稼働中の Pod に対してコマンドを実行します。対話モードにも対応します。

使用法:

```
kompoxops app exec -A <appName> [-i] [-t] [-e ESC] [-c CONTAINER] -- <command> [args...]
```

オプション:

- `-i, --stdin` 標準入力を接続
- `-t, --tty` TTY を割り当て (bash 等の対話時に推奨)
- `-e, --escape` デタッチ用エスケープシーケンス (既定: `~.`、`none` で無効化)
- `-c, --container` 実行対象のコンテナ名 (未指定時は最初のコンテナ)

挙動:

- 対象 Pod の選択はアプリの Namespace 内で実施します。
- `kompox.dev/tool-runner=true` が付与されたメンテナンス用ランナー Pod は除外します。
- 少なくとも 1 つの Ready コンテナを持つ Pod を優先し、無ければ非終了中の Pod を選択します。
- `--tty` 指定時は stderr は stdout にマージされます (TTY の仕様)。
- `--escape` で指定したシーケンスを送るとセッションを切断して終了できます (例: `~.`)。

例:

```
# デプロイ中のアプリ Pod でシェルを開く (対話)
kompoxops app exec -it -- bash

# コンテナ名を指定してログを確認
kompoxops app exec -t -c app -- sh -c 'tail -n 100 /var/log/app.log'
```

### kompoxops secret

アプリのシークレットの操作を行う。

```
kompoxops secret env set     --app-name <appName> -S <service> -f <file>
kompoxops secret env delete  --app-name <appName> -S <service>
kompoxops secret pull set    --app-name <appName> -f <file>
kompoxops secret pull delete --app-name <appName>
```

共通オプション

- `--app-name | -A` アプリ名を指定 (デフォルト: kompoxops.yml の app.name)

#### kompoxops secret env

アプリの特定コンテナ (Compose service) に対する環境変数を管理します。

Compose の `env_file` で定義された環境変数を上書きする追加の環境変数を設定できます。

##### kompoxops secret env set

環境変数設定を作成または更新します。

使用法:

```
kompoxops secret env set -A <appName> -S <serviceName> -f override.env
```

主なオプション:
- `-S, --service <serviceName>` 対象コンテナ (Compose service 名) を指定 (必須)
- `-f, --file <path>`          読み込むファイル (必須)

サポートファイル形式: `.env` / `.yml` / `.yaml` / `.json`

使用例:

```
# 環境変数設定を作成
kompoxops secret env set -A app1 -S app -f staging.env
```

##### kompoxops secret env delete

環境変数設定を削除します。

使用法:

```
kompoxops secret env delete -A <appName> -S <serviceName>
```

主なオプション:
- `-S, --service <serviceName>` 対象コンテナ (Compose service 名) を指定 (必須)

使用例:

```
# 環境変数設定を削除
kompoxops secret env delete -A app1 -S app
```

#### kompoxops secret pull

プライベートレジストリからイメージを取得するための認証情報を管理します。

##### kompoxops secret pull set

レジストリ認証情報を設定します。

使用法:

```
kompoxops secret pull set -A <appName> -f ~/.docker/config.json
```

主なオプション:
- `-f, --file <path>`  Docker 認証ファイル (`config.json`) を指定 (必須)

使用例:

```
# 認証情報を設定
kompoxops secret pull set -A app1 -f ~/.docker/config.json
```

##### kompoxops secret pull delete

レジストリ認証情報を削除します。

使用法:

```
kompoxops secret pull delete -A <appName>
```
使用例:

```
# 認証情報を削除
kompoxops secret pull delete -A app1
```

### kompoxops disk

app.volumes で定義された論理ボリュームに属するディスク (ボリュームインスタンス) を操作する。

```
kompoxops disk list   --app-name <appName> --vol-name <volName>                     ディスク一覧表示
kompoxops disk create --app-name <appName> --vol-name <volName> [-N <name>] [-S <source>] [--zone <zone>] [--options <json>] [--bootstrap] 新しいディスク作成 (サイズは app.volumes 定義を使用)
kompoxops disk assign --app-name <appName> --vol-name <volName> -N <name>          指定ディスクを <volName> の Assigned に設定 (他は自動的に Unassign)
kompoxops disk delete --app-name <appName> --vol-name <volName> -N <name>          指定ディスク削除 (Assigned 中はエラー)
```

共通オプション

- `--app-name | -A` アプリ名を指定 (デフォルト: kompoxops.yml の app.name)
- `--vol-name | -V` ボリューム名を指定
- `--name | -N` 操作対象ディスク名。`--disk-name` は同義のロングエイリアス。list/create では省略可、assign/delete では必須。

create 専用オプション

- `--source | -S` ディスクの作成元を示す任意文字列。CLI/UseCase は解釈・検証・正規化を一切行わず、そのまま Provider Driver に透過的に渡す。受理形式は Driver の仕様に従う。最低限の共通語彙として `disk:<name>` (Kompox 管理ディスク) と `snapshot:<name>` (Kompox 管理スナップショット) を予約する。省略時は空文字が渡り、Driver の既定挙動(例: 空ディスク作成)に委ねる。
- `--zone | -Z` アベイラビリティゾーンを指定 (app.deployment.zone をオーバーライド)
- `--options | -O` ボリュームオプションをJSON形式で指定 (app.volumes.options をオーバーライド/マージ)
- `--bootstrap` 全ボリュームについて Assigned ディスクが 1 件も存在しない場合に限り、各ボリューム 1 件ずつ新規作成する一括初期化モード。`--vol-name` と同時指定不可。

仕様

- `<volName>` は app.volumes に存在しない場合エラー。
- create: ディスク名は自動生成 (例: ULID 等) または `--name`/`--disk-name` 指定 (存在重複はエラー)。
- ディスク名は最大 24 文字。`--name`/`--disk-name` 指定時に 24 文字を超える値はエラー。自動生成名も 24 文字以内。
- ディスク名は Kubernetes の DNS-1123 ラベル準拠であること: 小文字英数字とハイフンのみ、先頭と末尾は英数字、正規表現は `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`。
- create の `--source` は CLI/UseCase で解釈せず不透明(opaque)に扱う。最終的な解決やバリデーションは Provider Driver の責務。
- assign: 1 論理ボリュームにつき同時に Assigned=true は 0 または 1。既に同一ディスクが Assigned なら成功 (冪等)。別ディスクが Assigned の場合は自動でそのディスクを Unassign 後に指定を Assign。
- delete: 対象が存在しなければ成功 (冪等)。Assigned=true のディスクは `--force` 無しで拒否。
- list 出力列例: NAME  ASSIGNED  SIZE  HANDLE(SHORT)  CREATED              UPDATED
- SIZE 表示は Gi 単位 (内部は bytes)。
- manifest 生成 (app deploy) 時: 各 volName で Assigned ディスクがちょうど 1 件でない場合エラー (ただし `app deploy --bootstrap-disks` 指定時は事前に一括初期化を試みる)。

ブートストラップ挙動 (`--bootstrap` / `--bootstrap-disks` 共通):
- 目的: 初期状態 (どのボリュームにも Assigned ディスクが無い) で手動操作無しで 1:1 対応の基盤を用意する。
- 成功条件: 全ボリューム Assigned=0。
- スキップ条件: 全ボリューム Assigned=1 (何も作らず成功)。
- エラー条件: 上記以外 (一部のみ Assigned>0 / Unassigned 残骸が混在)。
- 作成結果: 各ボリュームに 1 つずつ新規ディスク (Assigned=true) を生成し JSON (配列) で返す (disk create の場合)。

例

```
$ kompoxops disk list default
NAME        ASSIGNED  SIZE   HANDLE        CREATED              UPDATED
vol-202401  true      32Gi   1f3ab29 (az)  2024-01-10T12:00Z    2024-01-10T12:05Z
vol-202312  false     32Gi   9ab1c02 (az)  2023-12-31T09:00Z    2024-01-10T12:05Z
```

#### kompoxops disk list

ボリュームインスタンスの一覧を表示します。

#### kompoxops disk create

新しいボリュームインスタンスを作成します (サイズは app.volumes 定義)。

オプション：
- `--name | -N`: 明示的なディスク名を指定 (省略時は Driver が自動生成)。`--disk-name` は同義。最大 24 文字。
- `--source | -S`: ディスク作成元を示す任意文字列。CLI は解釈・検証・正規化を行わず、そのまま Driver に渡す。(予約語: `disk:`/`snapshot:`、省略時は空文字を渡し Driver に委任)
- `--zone | -Z`: デプロイメントゾーンを指定。app.deployment.zone の設定をオーバーライドします。
- `--options | -O`: ボリュームオプションをJSON形式で指定。app.volumes.options の設定をオーバーライド/マージします。
- `--bootstrap`: 全ボリューム未初期化時に 1 件ずつ一括作成。`--vol-name` と同時指定不可。

Source の扱い (パススルー):
- CLI/UseCase は `--source` の値をパースしない。検証や正規化は Provider Driver が行う。
- ドライバ共通の最低限の語彙として `disk:`/`snapshot:` を予約。`disk:<name>` は Kompox 管理ディスク名、`snapshot:<name>` は Kompox 管理スナップショット名を意味する。
- 省略時は `snapshot:` の省略とみなす。
- 例: AKS ドライバでは Kompox 管理名に加え Azure ARM Resource ID 等も受理し、ドライバ内部で解決する。

使用例：
```bash
# デフォルト設定でディスク作成 (名前は自動生成)
kompoxops disk create -V myvolume

# 名前を指定してディスク作成
kompoxops disk create -V myvolume -N cache-primary

# 特定のゾーンでディスク作成
kompoxops disk create -V myvolume --zone "2"

# オプションを指定してディスク作成
kompoxops disk create -V myvolume --options '{"sku":"PremiumV2_LRS","iops":3000}'

# ゾーンとオプション両方を指定
kompoxops disk create -V myvolume -Z "3" -O '{"sku":"Premium_LRS"}'

# スナップショットから復元
kompoxops disk create -V myvolume -S snapshot:backup-20250927

# 管理ディスクをコピー
kompoxops disk create -V myvolume -S disk:gold-master

# クラウドのリソースIDからインポート (例: Azure ARM Resource ID)
kompoxops disk create -V myvolume -S /subscriptions/.../providers/Microsoft.Compute/disks/d2

# 省略形: 'snapshot:' 省略をドライバが許容する実装例
kompoxops disk create -V myvolume -S daily-20250927
```

#### kompoxops disk assign

指定インスタンスを Assigned=true に設定し、他を自動的に Unassign します。

#### kompoxops disk delete

指定インスタンスを削除します (Assigned 中は `--force` なしで拒否)。

### kompoxops snapshot

app.volumes で定義された論理ボリュームに属するスナップショットを操作する。

スナップショットからのディスク作成は `kompoxops disk create -S` を使用する。

```
kompoxops snapshot list    --app-name <appName> --vol-name <volName>                               スナップショット一覧表示
kompoxops snapshot create  --app-name <appName> --vol-name <volName> [-N <name>] [-S <source>]      スナップショットを作成 (既定は Assigned ディスクを使用)
kompoxops snapshot delete  --app-name <appName> --vol-name <volName> -N <name>                     指定スナップショットを削除 (NotFound は成功)
```

共通オプション

- `--app-name | -A` アプリ名を指定 (デフォルト: kompoxops.yml の app.name)
- `--vol-name | -V` ボリューム名を指定
- `--name | -N` スナップショット名。`--snap-name` は同義。create では任意、delete では必須。最大 24 文字。


create 追加オプション

- `--source | -S` 作成元の識別子。CLI/UseCase は加工せず Driver にそのまま渡す。省略時は空文字となり Driver 既定 (Assigned ディスクの自動選択等) に委ねる。`disk:`/`snapshot:` の接頭辞はドライバ共通で予約。

仕様

- list: `CreatedAt` の降順で返す。出力は JSON 配列。
- create: 指定ソース (省略時は Assigned ディスク) からクラウドネイティブのスナップショットを作成し、JSON で 1 件返す。
- delete: 存在しない場合も成功 (冪等)。
- スナップショット名は最大 24 文字。24 文字を超える値はエラー。
- スナップショット名は Kubernetes の DNS-1123 ラベル準拠であること: 小文字英数字とハイフンのみ、先頭と末尾は英数字、正規表現は `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`。

使用例

```
# 一覧
kompoxops snapshot list -V db

# 省略時は Assigned ディスクから作成
kompoxops snapshot create -V db

# ディスク名を指定して作成元を明示
kompoxops snapshot create -V db -S disk:active-disk

# スナップショット名を明示
kompoxops snapshot create -V db -N daily-20250928

# スナップショット削除
kompoxops snapshot delete -V db -N 01J8WXYZABCDEF1234567890
```

#### kompoxops snapshot list

スナップショットの一覧を表示します。

#### kompoxops snapshot create

指定ディスクからスナップショットを作成します。

#### kompoxops snapshot delete

指定スナップショットを削除します (NotFound の場合も成功)。

### kompoxops admin

### kompoxops box

アプリの Namespace に Kompox Box (Deployment/Pod) をデプロイ・操作する。

```
kompoxops box deploy  --app-name <appName> [--image IMG] [-V vol:disk:/path]... [-c CMD]... [-a ARG]...   Kompox Box のデプロイ
kompoxops box destroy --app-name <appName>                                                                 Kompox Box の削除
kompoxops box status  --app-name <appName>                                                                 Kompox Box の状態表示
kompoxops box exec    --app-name <appName> [-i] [-t] [-e ESC] -- <command> [args...]                     Kompox Box 内でコマンド実行
kompoxops box ssh     --app-name <appName> -- <ssh args...>                                               Kompox Box への SSH 接続
kompoxops box scp     --app-name <appName> -- <scp args...>                                               Kompox Box とのファイル転送 (SCP)
kompoxops box rsync   --app-name <appName> -- <rsync args...>                                             Kompox Box とのファイル同期 (rsync)
```

共通オプション

- `--app-name | -A` 対象アプリ名 (デフォルト: kompoxops.yml の app.name)

仕様

- Kompox Box はアプリの Namespace に `kompox.dev/box=true` ラベル付きでデプロイされる。
- リソース名は固定で `kompox-box`。
- PV/PVC は必要に応じて自動生成される。
- PV/PVC をアプリ定義のボリュームにバインドしてマウントでき、メンテナンスや開発の環境として利用できる。

#### kompoxops box deploy

Kompox Box のリソースをデプロイします (冪等)。

オプション:

- `--image IMG` ランナーのコンテナイメージ (既定: `ghcr.io/kompox/kompox/box`)
- `-V, --volume volName:diskName:/mount/path` マウント指定 (複数指定可能)
  - `volName` は `app.volumes` のボリューム名
  - `diskName` はそのボリューム配下の既存ディスク名
  - `/mount/path` はコンテナ内の絶対パス
- `-c, --command TOKEN` エントリポイント (複数指定でトークン分割、image の ENTRYPOINT を上書き)
- `-a, --args TOKEN` 引数 (複数指定でトークン分割、image の CMD を上書き)
- `--ssh-pubkey FILE` SSH公開鍵ファイル (既定: `~/.ssh/id_rsa.pub`)
- `--always-pull` コンテナイメージを常に pull する

使用例:

```
# 基本的なデプロイ
kompoxops box deploy

# カスタムイメージとボリュームマウント
kompoxops box deploy --image debian:stable -V data:disk1:/mnt/data

# コマンドと引数を指定
kompoxops box deploy -c sleep -a infinity
```

備考:
- アプリ定義のボリュームと既存ディスクを指定してマウントできます。
- SSH公開鍵はコンテナ内で `/etc/ssh/authorized_keys` に登録されます。

#### kompoxops box destroy

Kompox Box のリソースを削除します (冪等)。

#### kompoxops box status

Kompox Box の状態を JSON で表示します。

出力例:

```json
{
  "ready": true,
  "image": "ghcr.io/kompox/kompox/box",
  "namespace": "kompox-app1-2ebe3c",
  "node": "aks-npsystem-32536790-vmss000001",
  "deployment": "kompox-box",
  "pod": "kompox-box-5dbc9cc965-hzvp5",
  "container": "box",
  "command": null,
  "args": null
}
```

#### kompoxops box exec

稼働中の Kompox Box 内でコマンドを実行します。対話モードにも対応します。

使用法:

```
kompoxops box exec -A <appName> [-i] [-t] [-e ESC] -- <command> [args...]
```

オプション:

- `-i, --stdin` 標準入力を接続
- `-t, --tty` TTY を割り当て (bash 等の対話時に推奨)
- `-e, --escape` デタッチ用エスケープシーケンス (既定: `~.`、`none` で無効化)

使用例:

```
# 対話シェル
kompoxops box exec -it -- bash

# ワンライナー実行
kompoxops box exec -t -- ls -la /mnt
```

備考:

- `kompox.dev/box=true` ラベルが付与された Pod を対象とします。
- Ready 状態の Pod を優先し、無ければ非終了中の Pod を選択します。
- `--tty` 指定時は stderr は stdout にマージされます (TTY の仕様)。
- `--escape` で指定したシーケンスを送るとセッションを切断して終了できます (例: `~.`)。

#### kompoxops box ssh

稼働中の Kompox Box に SSH 接続します。

使用法:

```
kompoxops box ssh -- <ssh args...>
```

使用例:

```
# user@host 形式
kompoxops box ssh -- kompox@a
kompoxops box ssh -- root@dummy

# -l オプション形式
kompoxops box ssh -- -l kompox hostname
kompoxops box ssh -- hostname -l root

# ポートフォワード付き
kompoxops box ssh -- -L 8080:localhost:8080 kompox@host
```

備考:

- Kubernetes API でポートフォワードを設定し、localhost にコンテナに接続するポートを開きます。
- SSH コマンドを起動し、`-o Hostname=localhost -p <port>` を自動設定してコンテナに接続します。
- 接続終了後、ポートフォワードを自動的に閉じます。

注意:

- ユーザー名は明示的に指定する必要があります (`user@host` 形式または `-l user` オプション)。
- `host` 部分は任意の文字列で構いません (例: `a`、`dummy`、`example.com`)。
- デフォルトイメージ (`ghcr.io/kompox/kompox/box`) では一般ユーザーは `kompox` です。`root` でも接続可能です。

#### kompoxops box scp

稼働中の Kompox Box と scp でファイル転送します。

使用法:

```
kompoxops box scp -- <scp args...>
```

使用例:

```
# ローカルファイルをリモートにコピー
kompoxops box scp -- localfile kompox@host:/path/to/remotefile

# リモートファイルをローカルにコピー
kompoxops box scp -- kompox@host:/path/to/remotefile localfile

# ディレクトリを再帰的にコピー
kompoxops box scp -- -r localdir kompox@host:/path/to/remotedir

# 複数ファイルの転送
kompoxops box scp -- file1.txt file2.txt kompox@host:/tmp/

# 圧縮転送（大きなファイル向け）
kompoxops box scp -- -C largefile.zip kompox@host:/data/

# 詳細モードでの転送
kompoxops box scp -- -v localfile kompox@host:/path/to/remotefile
```

主要なSCPオプション:

- `-r` ディレクトリを再帰的にコピー
- `-C` 圧縮を有効にして転送速度を向上
- `-v` 詳細な転送情報を表示
- `-p` ファイルの権限とタイムスタンプを保持
- `-q` 静音モード（進行状況を表示しない）

備考:

- SSH ポートフォワードを通じて SCP プロトコルでファイル転送を行います。
- 標準的な scp コマンドと同じオプションが使用できます。
- `host` 部分は任意の文字列で構いません（SSH と同様）。
- バイナリファイルやテキストファイルを問わず転送可能です。

#### kompoxops box rsync

稼働中の Kompox Box と rsync でファイル同期します。

使用法:

```
kompoxops box rsync -- <rsync args...>
```

使用例:

```
# 基本的な同期（アーカイブモード、詳細表示、圧縮）
kompoxops box rsync -- -avz localdir/ root@host:/path/to/remotedir/

# リモートからローカルへの同期
kompoxops box rsync -- -avz root@host:/path/to/remotedir/ localdir/

# 削除も含む完全同期（ミラーリング）
kompoxops box rsync -- -avz --delete localdir/ root@host:/path/to/remotedir/

# 特定のファイルを除外
kompoxops box rsync -- -avz --exclude='*.log' --exclude='tmp/' localdir/ root@host:/remotedir/

# ドライラン（実際の転送前に変更内容を確認）
kompoxops box rsync -- -avzn localdir/ root@host:/remotedir/

# 進行状況を表示
kompoxops box rsync -- -avz --progress localdir/ root@host:/remotedir/

# 帯域制限付きの転送
kompoxops box rsync -- -avz --bwlimit=1000 largedir/ root@host:/remotedir/

# バックアップ作成（既存ファイルを .bak で保存）
kompoxops box rsync -- -avz --backup --suffix=.bak localdir/ root@host:/remotedir/
```

主要なrsyncオプション:

- `-a, --archive` アーカイブモード（-rlptgoD と同等）
- `-v, --verbose` 詳細情報を表示
- `-z, --compress` 転送時にデータを圧縮
- `-r, --recursive` ディレクトリを再帰的に処理
- `-l, --links` シンボリックリンクをそのまま転送
- `-p, --perms` ファイル権限を保持
- `-t, --times` ファイルのタイムスタンプを保持
- `-g, --group` グループ所有権を保持
- `-o, --owner` ファイル所有者を保持
- `-D` デバイスファイルと特殊ファイルを保持
- `--delete` 送信先の余分なファイルを削除
- `--exclude=PATTERN` 指定パターンのファイルを除外
- `--progress` 転送進行状況を表示
- `-n, --dry-run` 実際の変更は行わず、何が変更されるかのみ表示
- `--bwlimit=RATE` 帯域制限（KB/s単位）
- `--backup` 既存ファイルをバックアップ

SCPとrsyncの使い分け:

- **SCP**: シンプルなファイル転送。一回限りのコピーに適している
- **rsync**: 差分同期、継続的な同期。大量のファイルや定期的な同期に適している

備考:

- SSH ポートフォワードを通じて rsync プロトコルでファイル同期を行います。
- rsync は差分転送により効率的な同期が可能です。
- 大量のファイルや定期的な同期作業に適しています。
- ディレクトリの末尾の `/` の有無で動作が変わるため注意が必要です。
  - `localdir/` → ディレクトリの**内容**を転送
  - `localdir` → ディレクトリ**自体**を転送

### kompoxops admin

管理用途 CLI を提供する。
一般ユーザー向け REST API とは異なり認証認可を無視した OOB の接続により CRUD 操作を直接呼び出すことができる。

```
kompoxops admin [--db-url <URL>] <KIND> <VERB> <options...>
kompoxops admin service list
kompoxops admin service get svc-a
kompoxops admin service create -f svc-a.yml
kompoxops admin service update svc-a -f svc-a.yml
kompoxops admin service delete svc-a
```
