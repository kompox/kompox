---
id: 2025-10-12-workspace
title: Domain Service → Workspace への改名
status: completed
updated: 2025-10-12
language: ja
---
# Task: Domain Service → Workspace への改名

関連: [K4x-ADR-006]

## 目的

- ドメイン概念 Service を Workspace に改名する。
- 後方互換は不要。最短手順で `make build`/`make test` をグリーンにする。
- 一時的に `type Service = Workspace` を導入し、段階的にシンボル/ファイル/パッケージ/CLI を改名する。

## スコープ / 非スコープ

- In:
  - domain/model の型改名: `Service` → `Workspace`
  - repository インタフェース改名: `ServiceRepository` → `WorkspaceRepository`
  - adapters/store の実装改名(inmem / rdb)
  - ファイル名の改名(例: `service.go` → `workspace.go`、`service_repository.go` → `workspace_repository.go`)
  - usecase パッケージ改名: `usecase/service` → `usecase/workspace`(import 更新含む)
  - CLI 改名: `--service` → `--workspace`、`admin service` → `admin workspace`(互換 alias なし)
  - テストの参照更新(型名/インポート/期待値)
- Out:
  - Kubernetes の `Service` kind 関連の識別子の改名(対象外)
  - `ServiceAccount` に関する識別子の改名(対象外)
  - 過去の ADR や既存 `_dev/tasks` の書き換え(対象外)
  - ストアのスキーマ/マイグレーション(不要)

## 方針(サマリ)

- 後方互換なし。最短でビルド/テストを通すことを最優先。
- `Workspace` を追加後すぐに `type Service = Workspace` を置き、全体を段階的にリネーム。
- 間違った一括置換を防ぐため、Kubernetes の `Service`/`ServiceAccount` に関する識別子は検索から除外・確認。
- AKS driver においては Azure Resource Manager (ARM) のリソース型に関する識別子は改名しない。
- ドキュメント更新はコードがグリーンになった後で実施(過去の ADR/_dev/tasks は書き換えない)。

## 変更詳細

### フェーズ1: 型/ファイル/パッケージ/CLI(コンパイル最優先)

1) domain/model(型)
- `Workspace` 構造体を追加し、`type Service = Workspace` を一時導入。
- 以後、コード参照を `Workspace` に切替。コンパイルを確認。

2) repository(インタフェース/実装)
- `ServiceRepository` → `WorkspaceRepository` に改名。
- inmem/rdb 実装の型・コンストラクタ名を追随改名。

3) domain, usecase, adapters 構造体型名・フィールド名の改名(例、必要に応じて拡張)
- usecase DTO `ServiceID` → `WorkspaceID`
- ローカル変数 `var service *model.Service` → `var workspace *model.Workspace`

4) ファイル名の改名(例、必要に応じて拡張)
- `domain/model/service.go` → `domain/model/workspace.go`
- `adapters/store/inmem/service.go` → `adapters/store/inmem/workspace.go`
- `adapters/store/rdb/service_repository.go` → `adapters/store/rdb/workspace_repository.go`

5) パッケージの改名
- `usecase/service` → `usecase/workspace` に改名。全インポートを更新。

6) CLI の改名 (cmd/kompoxops)
- フラグ: `--service` → `--workspace`
- コマンド: `admin service` → `admin workspace`
- 互換 alias は作らない(削除/未実装)。
- `cmd/kompoxops/cmd_admin_service.go` → `cmd/kompoxops/cmd_admin_workspace.go`

7) プロバイダドライバ adapters/drivers/provider の改名
- 型参照: `model.Service` → `model.Workspace`
- 定数/タグ名: `tagServiceName` → `tagWorkspaceName`
- 関数/メソッド名: `ServiceName` → `WorkspaceName`
- ローカル変数: `serviceName` → `workspaceName`
- AKS リソースタグ: `kompox-service-name` → `kompox-workspace-name`

8) ビルド/テスト
- `make build` が通るまでコンパイルエラーを解消。
- `make test` が通るまでテスト参照を更新。

9) 禁止/注意事項
- Kubernetes の `Service` kind と `ServiceAccount` に関連する識別子(関数/変数/ファイル名など)は改名しない。
- Azure Resource Manager (ARM) のリソース型に関する識別子は改名しない。
- 履歴的なコメント(「かつて Service だった」等)は追加しない。止むを得ず `Service` というリテラル/識別子を残す場合のみ、最小限の理由を記述。

### フェーズ2: 関数名/ローカル変数/短縮名(意味の整合)

- Kompox の Workspace 概念を指すものに限り、`svc` → `ws` などの短縮名を改名。
- 誤置換を防ぐため、検索スコープを `domain/`、`adapters/store/`、`usecase/`、CLI 層に限定し、`kube` 実装(K8s Service 関連)には触れない。

### フェーズ3: E2E テスト更新

- テンプレートファイル更新:
  - 全 `tests/*/kompoxops.yml.in`: `service:` → `workspace:`
- 既に生成済みの `kompoxops.yml` は `.gitignore` 対象なので自動的に再生成される。

### フェーズ4: ドキュメント

- 現行ドキュメント（README/コマンドヘルプ等）を「Workspace」表記へ更新。
- 過去の ADR と既存 `_dev/tasks` は書き換えない。

## 計画(チェックリスト)

- [x] フェーズ1: 型/ファイル/パッケージ/CLI
  - [x] `domain/model`: `Workspace` 追加 + `type Service = Workspace`
  - [x] `ServiceRepository` → `WorkspaceRepository`
  - [x] inmem/rdb 実装の改名(型/コンストラクタ)
  - [x] 構造体型名・フィールド名メンバ名の改名(`services` → `workspaces`、`ServiceID` → `WorkspaceID` など)
  - [x] ファイル名改名(model/inmem/rdb の service → workspace)
  - [x] パッケージ改名 `usecase/service` → `usecase/workspace`(import 更新)
  - [x] CLI フラグ `--service` → `--workspace`(Workspace概念に関連するもののみ)
  - [x] CLI コマンド `admin service` → `admin workspace`
  - [x] プロバイダドライバ `adapters/drivers/provider` の改名
    - [x] `model.Service` → `model.Workspace` (型参照)
    - [x] `tagServiceName` → `tagWorkspaceName` (AKS定数)
    - [x] `ServiceName` → `WorkspaceName` (メソッド名)
    - [x] `serviceName` → `workspaceName` (ローカル変数)
    - [x] `kompox-service-name` → `kompox-workspace-name` (AKSリソースタグ)
    - [x] `driverFactory` 関数シグネチャ更新 (registry.go)
  - [x] エラー定義の改名
    - [x] `domain/model/errors.go`: `ErrServiceInvalid` → `ErrWorkspaceInvalid`
    - [x] `domain/model/errors.go`: エラーメッセージ "service not found" → "workspace not found"
    - [x] `usecase/workspace/*.go`: `ErrServiceInvalid` 参照を `ErrWorkspaceInvalid` に更新(3ファイル)
  - [x] usecase パッケージのローカル変数改名
    - [x] `usecase/box/*.go`: `serviceObj` → `workspaceObj` (5ファイル: destroy.go, exec.go, deploy.go, port_forward.go, status.go)
    - [x] `usecase/app/*.go`: `serviceObj` → `workspaceObj` (4ファイル: destroy.go, exec.go, deploy.go, logs.go)
    - [x] `usecase/secret/*.go`: `serviceObj` → `workspaceObj` (2ファイル: env.go, pull.go)
    - [x] `usecase/cluster/logs.go`: `serviceObj` → `workspaceObj`
  - [x] adapters/kube での型参照更新
    - [x] `adapters/kube/converter.go`: `*model.Service` → `*model.Workspace` (Converter構造体フィールドと関数引数、2箇所)
    - [x] `adapters/kube/*_test.go`: `model.Service` → `model.Workspace` (全テストファイル、17箇所)
  - [x] `type Service = Workspace` エイリアスの削除
    - [x] `domain/model/service.go`: エイリアス削除完了
  - [x] `make build` グリーン
  - [x] `make test` グリーン
- [x] フェーズ2: 関数/ローカル/短縮名(`svc` → `ws`)
  - [x] Kompox Workspace 概念の参照のみ対象に改名
  - [x] 誤置換を防ぐスコープ限定とレビュー
- [x] フェーズ3: E2E テスト更新
  - [x] `tests/*/kompoxops.yml.in` の `service:` → `workspace:` 改名(全6ディレクトリ)
  - [x] `make build` グリーン
  - [x] `make test` グリーン
- [x] フェーズ4: ドキュメント
  - [x] README/CLI ヘルプ更新(Workspace 表記)
  - [x] 参照リンク/使用例の更新
    - [x] [Kompox-CLI.ja.md]: `service:` → `workspace:` 更新、`admin service` → `admin workspace` 更新
    - [x] [Kompox-Resources.ja.md]: `Service` → `Workspace` 更新
    - [x] [Kompox-Spec-Draft.ja.md]: `service:` → `workspace:` 更新、型定義の更新
    - [x] [Kompox-Arch-Implementation.ja.md]: パッケージ名、型定義、例の更新
    - [x] [README.ja.md]: サンプル `kompoxops.yml` の `service:` → `workspace:` 更新
    - [x] トップレベル `kompoxops.yml`: `workspace:` 使用済み（既存）

## テスト

- ユニット/ビルド確認
  - `make build` が成功すること
  - `make test` が成功すること
  - `grep -r "usecase/service"` がヒットしないこと(パッケージ改名の確認)
  - `grep -r "\bServiceRepository\b"` がヒットしないこと(インタフェース改名の確認)
  - `grep -r "package service" usecase` がヒットしないこと(パッケージ宣言の確認)
  - `grep -r "\bErrServiceInvalid\b"` がヒットしないこと(エラー定義改名の確認)
  - `grep -r "serviceObj" usecase` がヒットしないこと(ローカル変数改名の確認)
  - `grep -r "model\.Service" --include="*.go"` がヒットしないこと(エイリアス完全削除の確認)

- E2E テスト確認
  - `grep "^service:" tests/*/kompoxops.yml.in kompoxops.yml` がヒットしないこと
  - `grep "^workspace:" tests/*/kompoxops.yml.in kompoxops.yml` が全テンプレートでヒットすること
  - 少なくとも1つのE2Eテスト(例: `tests/aks-e2e-basic`)で `make` が成功し、生成された `kompoxops.yml` に `workspace:` が含まれること

## 受け入れ条件(Acceptance Criteria)

- ✅ `make build` / `make test` がグリーン
- ✅ Kompox の「Workspace」概念に関する型/パッケージ/CLI が改名済み
- ✅ `usecase/service` からの import が残っていない
- ✅ `ServiceRepository` の参照が残っていない
- ✅ `type Service = Workspace` エイリアスが削除され、全参照が `Workspace` に統一済み
- ✅ CLI の `--workspace` フラグと `admin workspace` コマンドが有効
- ✅ E2E テストテンプレート(`kompoxops.yml.in`)が `workspace:` を使用
- ✅ Kubernetes の `Service` kind と `ServiceAccount` に関する識別子は未変更
- ✅ 余計な履歴コメントが追加されていない(必要な場合のみ理由付きで最小限)
- ✅ ドキュメントが最新の Workspace 概念に更新済み

## メモ(リスク/フォローアップ)

- 一括置換による誤改名のリスク(特に kube 実装周辺)。検索スコープとレビューで軽減。✅ 完了
- CRD ローダ(今後導入予定)での kind 命名(`KompoxService` → 将来 `KompoxWorkspace` 追加許容)については別タスクで扱う。
- `spHASH` (service-provider hash) の命名は維持。"service provider" は業界標準用語として適切。
- ドキュメント更新は別 PR に分けるとレビューしやすい。

## 進捗

- 2025-10-12: タスク作成(本ドキュメント)
- 2025-10-12: フェーズ1 完了
  - ✅ 型改名完了: `domain/model/service.go` に `Workspace` 追加、`type Service = Workspace` でエイリアス設定
  - ✅ リポジトリ改名完了: `ServiceRepository` → `WorkspaceRepository` (domain/repository.go)
  - ✅ ストア実装改名完了:
    - `adapters/store/inmem/workspace.go`: `WorkspaceRepository` 実装
    - `adapters/store/rdb/workspace_repository.go`: `WorkspaceRepository` 実装
  - ✅ usecase パッケージ改名完了: `usecase/service` → `usecase/workspace`
  - ✅ ビルド/テスト通過: `make build` および `make test` が成功
  - ✅ 構造体フィールド名改名: 完了
    - ✅ `usecase/workspace/*.go`: `ServiceID` → `WorkspaceID` 改名完了(5ファイル: get.go, update.go, delete.go, list.go, create.go)
    - ✅ `usecase/workspace/*.go`: `Service` → `Workspace` 出力フィールド改名完了
    - ✅ `adapters/drivers/provider/volume_port.go`: `services` → `workspaces` フィールド改名完了
    - ✅ `adapters/drivers/provider/cluster_port.go`: `services` → `workspaces` フィールド改名完了
    - ✅ `usecase/dns/deploy.go`, `usecase/dns/destroy.go`: `var service` → `var workspace` 改名完了
    - ✅ `adapters/drivers/provider/*.go`: 関数引数とローカル変数改名完了
    - ✅ `cmd/kompoxops/cmd_admin_service.go`: usecase 呼び出しの引数更新完了
  - ✅ CLI 改名: 完了
    - ✅ `cmd/kompoxops/cmd_admin.go`: `newCmdAdminService()` → `newCmdAdminWorkspace()` に変更完了
    - ✅ `cmd/kompoxops/cmd_admin_service.go` → `cmd/kompoxops/cmd_admin_workspace.go`: ファイル改名完了
    - ✅ `cmd/kompoxops/cmd_admin_workspace.go`: 全関数名改名完了 (`newCmdAdminWorkspaceList`, `newCmdAdminWorkspaceGet`, `newCmdAdminWorkspaceCreate`, `newCmdAdminWorkspaceUpdate`, `newCmdAdminWorkspaceDelete`)
    - ✅ `cmd/kompoxops/cmd_admin_workspace.go`: 型名・変数名改名完了 (`workspaceSpec`, `spec workspaceSpec`)
    - ✅ `cmd/kompoxops/cmd_admin_workspace.go`: コマンド説明更新完了 ("Create a workspace", "Update a workspace", "Delete a workspace" など)
    - 注: `cmd/kompoxops/cmd_secret_env.go` の `--service` は docker-compose service を指すため対象外
  - ✅ プロバイダドライバ改名: 完了
    - ✅ `adapters/drivers/provider/registry.go`: `Driver.ServiceName()` → `Driver.WorkspaceName()` インターフェース更新
    - ✅ `adapters/drivers/provider/registry.go`: `driverFactory` 関数シグネチャ `*model.Service` → `*model.Workspace` に更新
    - ✅ `adapters/drivers/provider/aks/driver.go`: `serviceName` → `workspaceName` フィールド改名
    - ✅ `adapters/drivers/provider/aks/driver.go`: `ServiceName()` → `WorkspaceName()` メソッド改名
    - ✅ `adapters/drivers/provider/aks/driver.go`: init 関数で `service` → `workspace` パラメータ改名
    - ✅ `adapters/drivers/provider/aks/naming.go`: `tagServiceName` → `tagWorkspaceName` 定数改名
    - ✅ `adapters/drivers/provider/aks/naming.go`: `kompox-service-name` → `kompox-workspace-name` タグ値更新
    - ✅ `adapters/drivers/provider/aks/naming.go`: 全関数内で `d.ServiceName()` → `d.WorkspaceName()` に変更 (6箇所)
    - ✅ `adapters/drivers/provider/k3s/k3s.go`: `serviceName` → `workspaceName` フィールド改名
    - ✅ `adapters/drivers/provider/k3s/k3s.go`: `ServiceName()` → `WorkspaceName()` メソッド改名
    - ✅ `adapters/drivers/provider/k3s/k3s.go`: init 関数で `service` → `workspace` パラメータ改名
- 2025-10-12: フェーズ2 完了
  - ✅ ID プレフィックス改名: `svc-` → `ws-`
    - ✅ `adapters/store/inmem/workspace.go`: ID 生成 `fmt.Sprintf("ws-%d-%d", ...)` に変更
    - ✅ `adapters/store/rdb/workspace_repository.go`: ID 生成 `"ws-" + uuid.NewString()` に変更
  - ✅ ローカル変数改名: Workspace 概念の `svc` → `ws`
    - ✅ `usecase/app/validate.go`: 変数 `svc` → `ws` (4箇所改名)
    - ✅ `usecase/app/status.go`: 変数 `svc` → `ws` (3箇所改名)
    - 注: `usecase/app/deploy.go` の `svc` は Kubernetes Service オブジェクトを指すため対象外(意図通り残存)
  - ✅ ビルド/テスト通過: `make build` および `make test` が成功
- 2025-10-12: フェーズ3 完了
  - ✅ E2E テストテンプレート更新: `service:` → `workspace:`
    - ✅ `tests/aks-e2e-basic/kompoxops.yml.in`: 更新完了
    - ✅ `tests/aks-e2e-easyauth/kompoxops.yml.in`: 更新完了
    - ✅ `tests/aks-e2e-gitea/kompoxops.yml.in`: 更新完了
    - ✅ `tests/aks-e2e-gitlab/kompoxops.yml.in`: 更新完了
    - ✅ `tests/aks-e2e-redmine/kompoxops.yml.in`: 更新完了
    - ✅ `tests/aks-e2e-volume/kompoxops.yml.in`: 更新完了
  - ✅ ビルド/テスト通過: `make build` および `make test` が成功
  - ✅ 検証: `grep "^service:" tests/*/kompoxops.yml.in kompoxops.yml` がヒットしないことを確認
  - ✅ 検証: `grep "^workspace:" tests/*/kompoxops.yml.in kompoxops.yml` が全7ファイルでヒット
- 2025-10-12: cmd層の最終クリーンアップ完了
  - ✅ `cmd/kompoxops/cmd_cluster.go`: 変数・コメント改名完了
    - ✅ `serviceObj` → `workspaceObj` (2箇所)
    - ✅ `serviceName` → `workspaceName` (5箇所)
    - ✅ コメント内の "service" → "workspace" (naming.NewHashes 関連)
  - ✅ `cmd/kompoxops/cmd_config.go`: 出力フォーマット更新完了
    - ✅ `"service=%s"` → `"workspace=%s"`
  - ✅ `cmd/kompoxops/usecase_builder.go`: コメント更新完了
    - ✅ "service" → "workspace" (WorkspaceRepository の説明)
  - ✅ ビルド/テスト通過: `make build` および `make test` が成功
  - ✅ 検証: コード内の Workspace 概念に関する "service" 参照が残っていないことを確認
- 2025-10-12: エラー定義とusecaseローカル変数の改名完了
  - ✅ `domain/model/errors.go`: エラー定義更新完了
    - ✅ `ErrServiceInvalid` → `ErrWorkspaceInvalid`
    - ✅ エラーメッセージ "service not found" → "workspace not found"
  - ✅ `usecase/workspace/*.go`: エラー参照更新完了(3ファイル: get.go, create.go, update.go)
  - ✅ `usecase/box/*.go`: ローカル変数 `serviceObj` → `workspaceObj` 改名完了(5ファイル: destroy.go, exec.go, deploy.go, port_forward.go, status.go)
  - ✅ `usecase/app/*.go`: ローカル変数 `serviceObj` → `workspaceObj` 改名完了(4ファイル: destroy.go, exec.go, deploy.go, logs.go)
  - ✅ `usecase/secret/*.go`: ローカル変数 `serviceObj` → `workspaceObj` 改名完了(2ファイル: env.go, pull.go)
  - ✅ `usecase/cluster/logs.go`: ローカル変数 `serviceObj` → `workspaceObj` 改名完了
  - ✅ ビルド/テスト通過: `make build` および `make test` が成功
  - ✅ 検証: `grep -r "ErrServiceInvalid"` がヒットしないことを確認
  - ✅ 検証: `grep -r "serviceObj" usecase` がヒットしないことを確認
- 2025-10-12: usecaseコメント・文字列内の"service"更新完了
  - ✅ `usecase/dns/types.go`: コメント "application services" → "application logic" に更新
  - ✅ `usecase/workspace/types.go`: コメント "service use cases" → "workspace use cases" に更新(2箇所)
  - ✅ `usecase/app/status.go`: エラーメッセージ "failed to get service" → "failed to get workspace"
  - ✅ `usecase/app/validate.go`: エラーメッセージ "failed to get service" → "failed to get workspace"
  - ✅ `usecase/app/validate.go`: 警告メッセージ "service not found" → "workspace not found"
  - ✅ `usecase/app/deploy.go`: コメント "cluster/provider/service" → "cluster/provider/workspace"
  - ✅ ビルド/テスト通過: `make build` および `make test` が成功
  - 注: `usecase/secret/env.go` の "compose service" は docker-compose service を指すため対象外
  - 注: `usecase/app/deploy.go` の "headless Services" は Kubernetes Service リソースを指すため対象外
- ✅ フェーズ4: ドキュメント更新完了
  - ✅ [Kompox-CLI.ja.md]: 更新完了
    - ✅ `kompoxops.yml` サンプル: `service:` → `workspace:` 更新
    - ✅ 説明文: "service/provider/cluster" → "workspace/provider/cluster" 更新
    - ✅ CLI コマンド例: `admin service` → `admin workspace` 更新 (`ws-a`, `ws-a.yml` 等)
    - ✅ kubeconfig コマンドの説明: "Service/Provider/Cluster/App" → "Workspace/Provider/Cluster/App" 更新
  - ✅ [Kompox-Resources.ja.md]: 更新完了
    - ✅ リソース一覧: `Service` → `Workspace` 更新
    - ✅ リソース定義例: `kind: Service` → `kind: Workspace` 更新
    - ✅ Provider定義: `service:` フィールド → `workspace:` フィールド更新
  - ✅ [Kompox-Spec-Draft.ja.md]: 更新完了
    - ✅ `kompoxops.yml` サンプル: `service:` → `workspace:` 更新
    - ✅ 型定義: `type Service` → `type Workspace` 更新
    - ✅ Provider型: `Service string` → `Workspace string` 更新
    - ✅ コメント: "Serviceに所属" → "Workspaceに所属" 更新
    - ✅ タグ仕様: `{Service.Name}/...` → `{Workspace.Name}/...` 更新
  - ✅ [Kompox-Arch-Implementation.ja.md]: 更新完了
    - ✅ パッケージ構造: `service/` → `workspace/` 更新
    - ✅ リソース一覧: `Service` → `Workspace` 更新
    - ✅ DTO例: `Service *model.Service` → `Workspace *model.Workspace` 更新
    - ✅ Repos構造体: `ServiceRepository` → `WorkspaceRepository` 更新
    - ✅ 全サンプルコード: Service関連 → Workspace関連 更新
  - ✅ [Kompox-KubeConverter.ja.md]: 更新完了
    - ✅ 概要: "Service/Provider/Cluster/App" → "Workspace/Provider/Cluster/App" 更新
    - ✅ アノテーション: `<serviceName>/...` → `<workspaceName>/...` 更新
    - ✅ ハッシュ計算: `service.name` → `workspace.name` 更新（3箇所: spHASH, inHASH, idHASH）
    - ✅ `kompoxops.yml` サンプル: `service:` → `workspace:` 更新
    - 注: Kubernetes の Service, ServiceAccount リソースは未変更（混同回避）
    - 注: docker-compose の service 参照 (`containerName=serviceName`) は未変更（対象外）
  - ✅ [Kompox-ProviderDriver.ja.md]: 更新完了
    - ✅ Driver インターフェース: `ServiceName()` → `WorkspaceName()` メソッド更新
    - ✅ コメント: "service name" → "workspace name" 更新
    - ✅ ファクトリ関数: `func(service *model.Service, ...)` → `func(workspace *model.Workspace, ...)` 更新
    - ✅ 生成説明: `factory(service, provider)` → `factory(workspace, provider)` 更新
  - ✅ [Kompox-ProviderDriver-AKS.ja.md]: 更新完了
    - ✅ Deployment Stack 名: `kompox_<ServiceName>_...` → `kompox_<WorkspaceName>_...` 更新
    - ✅ クラスタタグ: `<ServiceName>/<ProviderName>/...` → `<WorkspaceName>/<ProviderName>/...` 更新
    - ✅ 説明文: "ServiceName はサービスが nil" → "WorkspaceName はワークスペースが nil" 更新
    - 注: Azure ARM の `Microsoft.ContainerService` などのリソース型は未変更（対象外）
  - ✅ [README.ja.md]: 更新完了
    - ✅ `kompoxops.yml` サンプル: `service:` → `workspace:` 更新
  - ✅ ビルド/テスト通過: `make build` および `make test` が成功
  - ✅ 検証: `grep "^service:" tests/*/kompoxops.yml.in kompoxops.yml` がヒットしない
  - ✅ 検証: `grep "^workspace:" tests/*/kompoxops.yml.in kompoxops.yml` が7ファイルでヒット
- 2025-10-12: エイリアス削除とadapters/kube更新完了
  - ✅ `domain/model/service.go`: `type Service = Workspace` エイリアス削除
  - ✅ `adapters/kube/converter.go`: `*model.Service` → `*model.Workspace` 更新 (Converter構造体と NewConverter関数)
  - ✅ `adapters/kube/*_test.go`: `model.Service` → `model.Workspace` 一括更新 (17箇所)
  - ✅ ビルド/テスト通過: `make build` および `make test` が成功
  - ✅ 検証: `grep -r "model\.Service"` がヒットしないことを確認 (エイリアス完全削除)

## 完了サマリ

2025-10-12: **タスク完了** 🎉

すべてのフェーズ（1-4）が完了し、受け入れ条件をすべて満たしました。

### 実施内容
- ✅ フェーズ1: 型/ファイル/パッケージ/CLI の改名
- ✅ フェーズ2: 関数/ローカル変数/短縮名の改名
- ✅ フェーズ3: E2E テストテンプレートの更新
- ✅ フェーズ4: ドキュメントの更新（10ファイル）

### 更新されたドキュメント
1. [Kompox-CLI.ja.md]
2. [Kompox-Resources.ja.md]
3. [Kompox-Spec-Draft.ja.md]
4. [Kompox-Arch-Implementation.ja.md]
5. [Kompox-KubeConverter.ja.md]
6. [Kompox-ProviderDriver.ja.md]
7. [Kompox-ProviderDriver-AKS.ja.md]
8. [README.ja.md]
9. `kompoxops.yml`
10. `tests/*/kompoxops.yml.in` (6ファイル)

### 品質保証
- ✅ `make build` 成功
- ✅ `make test` 成功
- ✅ Kubernetes リソース（Service, ServiceAccount）との混同なし
- ✅ Docker Compose の service 概念との混同なし
- ✅ Azure ARM リソース型との混同なし
- ✅ 履歴コメント最小化

[K4x-ADR-006]: ../../design/adr/K4x-ADR-006.md
[Kompox-CLI.ja.md]: ../../design/v1/Kompox-CLI.ja.md
[Kompox-Resources.ja.md]: ../../design/v1/Kompox-Resources.ja.md
[Kompox-Spec-Draft.ja.md]: ../../design/v1/Kompox-Spec-Draft.ja.md
[Kompox-Arch-Implementation.ja.md]: ../../design/v1/Kompox-Arch-Implementation.ja.md
[Kompox-KubeConverter.ja.md]: ../../design/v1/Kompox-KubeConverter.ja.md
[Kompox-ProviderDriver.ja.md]: ../../design/v1/Kompox-ProviderDriver.ja.md
[Kompox-ProviderDriver-AKS.ja.md]: ../../design/v1/Kompox-ProviderDriver-AKS.ja.md
[README.ja.md]: ../../README.ja.md
