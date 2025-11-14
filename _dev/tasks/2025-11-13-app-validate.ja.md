---
id: 2025-11-13-app-validate
title: app validate バリデーション共通化と未割当ディスク警告化
status: done
updated: 2025-11-14
language: ja
owner: yaegashi
---
# Task: app validate バリデーション共通化と未割当ディスク警告化

## 目的

- `kompoxops app validate` / `app deploy` に分散している App バリデーションロジックを共通化し、UseCase 層で一元的に管理できるようにする。
- `Severity` / `Issue` モデルを導入し、INFO/WARN/ERROR レベル付きの検証結果として扱えるようにする。
- 論理ボリューム (app.volumes) の Assigned ディスク未存在状態 (count=0) を `app validate` では WARN 扱いとして exit code 0 で通しつつ、`app deploy` ではブロック要因として扱う。
- 新規 App 作成 → `app validate` → `app deploy --bootstrap-disks` の標準フローを確立する。
- デプロイ (`app deploy`) では従来の厳格要件 (各 volume で Assigned=1) を維持し、`--bootstrap-disks` が初回不足を自動解決する仕組みを明確化する。

## スコープ / 非スコープ

- In:
  - validate / deploy 双方で利用する App バリデーションロジックの共通化
  - Severity / Issue 導入による検証結果の構造化
  - validate 時のボリューム割当数チェック挙動変更 (count=0 → WARN, count>1 → ERROR 維持, count=1 → OK)
  - validate 出力 (Compose 正規化 / Manifest 生成) 継続条件の調整
  - CLI ログ出力 (Warn レベル) の追加
  - 関連テスト修正 / 追加
  - CLI ドキュメント (`Kompox-CLI.ja.md`) 更新
- Out:
  - ディスク/スナップショット管理機能自体の仕様変更
  - Volume Type / オプションスキーマ変更
  - Driver 側の物理リソース検証改善

## 仕様サマリ

### Severity レベル

- `INFO`: 追加情報・注記レベル。validate / deploy ともにブロックしない。
- `WARN`: 仕様上は問題があるが `app validate` では exit code に反映せず、`app deploy` ではブロック対象とする。
- `ERROR`: validate / deploy いずれのフェーズでも即時ブロックする致命的な問題。

### volume assignment チェックへの適用

- `count == 0` → `WARN` (`volume assignment missing`)
- `count == 1` → Issue なし
- `count > 1` → `ERROR` (`volume assignment invalid (count>1)`)

### モード別ポリシー

`kompoxops app validate`:

| Severity | ログレベル | 継続 | 退出コード |
|----------|------------|------|-----------|
| INFO     | Info       | 継続 | 0         |
| WARN     | Warn       | 継続 | 0         |
| ERROR    | Error      | 中断 (エラー集約) | !=0 |

`kompoxops app deploy`:

| Severity | ログレベル | 継続 | 退出コード |
|----------|------------|------|-----------|
| INFO     | Info / 無視 | 続行 | 0         |
| WARN     | Warn       | 中断 | !=0       |
| ERROR    | Error      | 中断 | !=0       |

- `--bootstrap-disks`: deploy 前に全 volume が Assigned=0 のときのみ一括作成し再評価。部分的状態 (混在) は従来通りエラー。
- validate は PV/PVC 名など Assigned ディスク情報が必要な箇所で現在の仕様を保ちつつ、未割当状態で Manifest 生成が成立するか確認 (必要なら内部ハンドル参照を条件付き生成/スキップ)。

## 計画 (チェックリスト)

- [x] `Severity` / `Issue` 型を usecase 層に導入する (INFO/WARN/ERROR)
- [x] UseCase 内部専用の共通バリデーション実装を `usecase/app/validations.go` に追加する
- [x] `validateApp` と `validateAppVolumes` を `validations.go` に実装し、App/Workspace/Provider/Cluster から `[]Issue` と Compose/Converter/K8sObjects を生成できるようにする
- [x] 既存の `Validate` 実装からバリデーションロジックを切り出し、`validateApp` を呼び出して結果を `ValidateOutput.Errors` / `.Warnings` にマッピングするように変更する
- [x] `Deploy` ユースケースからも `validateApp` を呼び出し、`Issue` の Severity に応じてデプロイ可否を判定する (WARN/ERROR をブロック要因とする)
- [x] volume assignment チェックで count==0 を WARN, count>1 を ERROR として `Issue` を生成し、未割当状態は `app validate` では許容・`app deploy` ではエラーになることを確認する
- [x] `--bootstrap-disks` 実行前後で `validateApp` を再評価し、volume assignment 関連の WARN/ERROR が解消された場合にのみ deploy を継続するようにする
- [x] テスト更新: 未割当ケース → WARN Issue のみ + validate 成功; 複数割当ケース → ERROR Issue + validate/deploy 失敗 を確認する
- [x] 新規テスト: 初回 `app deploy --bootstrap-disks` で volume assignment WARN が解消され成功するパスを追加する (tests/aks-e2e-validate/test-run.sh → `_tmp/tests/aks-e2e-validate` で `make run` 実行)
- [x] ドキュメント更新: `app validate` 節とブートストラップ挙動説明補足 (INFO/WARN/ERROR レベルと validate/deploy の扱いの違いを明記)

## 受け入れ条件

- `kompoxops app validate` 実行時に未割当 (全 volume count=0) で volume assignment WARN Issue が生成され、exit code=0 かつ Compose/Manifest 出力が成功する。
- 同一 validate 実行で volume 複数割当 (count>1) がある場合は ERROR Issue が生成され、従来通り exit code!=0 となりメッセージが列挙される。
- `kompoxops app deploy` は未割当状態で `--bootstrap-disks` 非指定なら volume assignment WARN/ERROR を検出して失敗する。
- `kompoxops app deploy` は未割当状態で `--bootstrap-disks` 指定時にディスク自動作成後、WARN/ERROR が解消された状態で成功する。
- 部分的割当 (一部のみ Assigned>0) で `--bootstrap-disks` を指定した場合は従来通りエラーとなる。
- 変更後のテスト (unit/usecase) がグリーンである。
- `design/v1/Kompox-CLI.ja.md` に INFO/WARN/ERROR ベースの警告化仕様が反映され、標準フロー記述が追加されている。

## メモ

- 警告文言例: `volume assignment missing (count=0) volume=<volName>`
- エラー文言は既存の `volume assignment invalid` を再利用 (複数割当時)。
- Warnings は CLI 側で先に表示されるが exit code は 0。

## 進捗

- 2025-11-13: タスク作成
- 2025-11-14: Severity/Issue 共通化と validate/deploy 統合、volume assignment WARN/ERROR 仕様の実装、CLI ドキュメント更新、`make build` `make test` 実行での正常完了を確認
- 2025-11-14: `tests/aks-e2e-validate` にて `make run` を実施し、未割当状態での WARN→`app deploy` ブロック→`--bootstrap-disks --update-dns` での成功、および最終 `app validate`/`app status` の正常化を確認

## 参考

- [Kompox-CLI.ja.md]
- [2025-10-24-volume-types.ja.md]
- [K4x-ADR-014]

[Kompox-CLI.ja.md]: ../../design/v1/Kompox-CLI.ja.md
[2025-10-24-volume-types.ja.md]: ./2025-10-24-volume-types.ja.md
[K4x-ADR-014]: ../../design/adr/K4x-ADR-014.md
