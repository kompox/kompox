---
id: 2025-10-23-protection
title: リソース保護ポリシー導入 Step 1-3
status: active
updated: 2025-10-23
language: ja
---
# Task: リソース保護ポリシー導入 Step 1-3

## 目的

- 誤操作による `kompoxops cluster deprovision` と `kompoxops cluster uninstall` の実行から、クラスタやインフラを保護する。
- ADR [K4x-ADR-013] に基づき、Cluster の宣言に `spec.protection` (scopes: `provisioning`, `installation`) と値 `none` `cannotDelete` `readOnly` を導入し、Kompox 内での強制ガードを提供する。
- 本タスクでは Step 3 までを実装対象とする。

## スコープ / 非スコープ

- In:
  - Step 1: CRD と Domain への追加
    - `spec.protection.provisioning` と `spec.protection.installation`
    - enum: `none` `cannotDelete` `readOnly` (default: `none`)
  - Step 2: UseCase 側の強制ガード
    - deprovision は `provisioning` を参照してブロック
    - uninstall と更新系は `installation` を参照してブロック
    - `--force` は無視し、明示的に `none` に戻してから再試行させるエラーメッセージ
    - 初回作成は `readOnly` でも妨げない (初回成功後に保護が効く)
  - Step 3: CLI 早期ガード
    - deprovision/uninstall コマンドでの事前チェックと統一されたエラー文言
- Out:
  - Step 4: finalizer による CR 削除阻止の厳格化
  - Step 5: Azure Management Lock との同期 (CanNotDelete/ReadOnly)
  - Step 6: 仕様ドキュメントの全面更新や索引再生成の運用 (必要に応じて別タスク) ただし本タスクに必要な最小限の CLI ヘルプ修正は可

## 仕様サマリ

- フィールド
  - `spec.protection.provisioning`: クラウド/インフラのライフサイクル操作を制御 (provision/deprovision)
  - `spec.protection.installation`: クラスタ内のライフサイクル操作を制御 (install/uninstall/updates)
- 値
  - `none`: 制限なし
  - `cannotDelete`: 破壊的操作 (deprovision/uninstall) を禁止
  - `readOnly`: 破壊的操作と更新を禁止 (実質 immutable)
- 初回作成の扱い
  - リソース/インストールの未存在が検出された場合、`readOnly` 指定でも初回の provision/install は許可する
  - 初回成功後に保護が有効化され、以後の更新/削除が制御対象となる
- エラーポリシー
  - UseCase は `--force` を無視する
  - エラーメッセージは scope と値を明示し、解除方法 (`none` に変更) を示す

## 計画 (チェックリスト)

- [x] Step 1: CRD と Domain の拡張
  - [x] CRD 型 (`config/crd/ops/...`) に `spec.protection` を追加し、enum と default を定義
  - [x] Domain model (`domain/model/...`) に対応する型と値を追加
  - [x] 既存 YAML/JSON の後方互換 (default `none`) を確認
- [x] Step 2: UseCase のガード実装
  - [x] `usecase/cluster/deprovision.go` で `provisioning` に基づくブロックを実装
  - [x] uninstall/更新系の UseCase (例: `usecase/cluster/install.go` など) で `installation` に基づくブロックを実装
  - [x] 初回作成は許可するための既存判定 (status と provider 存在確認のいずれか) を導入
  - [x] エラーメッセージの統一
- [x] Step 3: CLI 早期ガード
  - [x] `cmd/kompoxops/cmd_cluster.go` の deprovision/uninstall に早期チェックを追加
  - [x] CLI ヘルプ/メッセージの最小更新

## テスト

- ユニット
  - `default none` で従来通り動作すること
  - `cannotDelete` で deprovision/uninstall がブロックされること
  - `readOnly` で更新系もブロックされること
  - 初回作成時は `readOnly` でも許可されること (作成成功後に保護が効く)
- スモーク
  - `kubectl patch` で `spec.protection` を設定し、`kompoxops cluster deprovision` や `uninstall` 実行時に期待通りのブロック/メッセージになること

## 受け入れ条件

- `spec.protection.provisioning` が `cannotDelete` または `readOnly` のとき、`kompoxops cluster deprovision` が UseCase と CLI の両方で拒否される
- `spec.protection.installation` が `cannotDelete` または `readOnly` のとき、`kompoxops cluster uninstall` と更新系操作が UseCase と CLI の両方で拒否される
- `readOnly` のとき更新系操作が拒否される (実質 immutable)
- 初回作成は `readOnly` 指定でもブロックされない
- `--force` 指定でも拒否され、解除方法がメッセージで案内される

## メモ

- 後方互換性は default `none` で確保される
- 初回作成判定は status と provider 側の存在確認の両面で実装できると堅牢
- Step 4 以降は別タスクで段階導入する (finalizer, Azure Lock 同期, ドキュメント全体更新)

## 進捗

- 2025-10-23: タスク起票 (仕様確定と範囲の明確化)。
- 2025-10-23: Step 1-3 実装完了。
  - CRD と Domain に `spec.protection.provisioning` と `spec.protection.installation` を追加
  - 値: `none` (default), `cannotDelete`, `readOnly`
  - UseCase に保護ガードを実装 (`CheckProvisioningProtection`, `CheckInstallationProtection`)
  - CLI に早期ガードを追加 (deprovision, uninstall)
  - 初回作成時は保護をバイパスする判定を実装
  - ユニットテスト追加、全テスト通過

## 参考

- [K4x-ADR-013]
- [Kompox-CRD.ja.md]
- [Kompox-Arch-Implementation.ja.md]
- [2025-10-23-aks-cr.ja.md]

[K4x-ADR-013]: ../../design/adr/K4x-ADR-013.md
[Kompox-CRD.ja.md]: ../../design/v1/Kompox-CRD.ja.md
[Kompox-Arch-Implementation.ja.md]: ../../design/v1/Kompox-Arch-Implementation.ja.md
[2025-10-23-aks-cr.ja.md]: ./2025-10-23-aks-cr.ja.md
