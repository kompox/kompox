---
id: 2025-11-03-kompox-cli-env
title: Kompox CLI Env の導入と KOM 入力優先順位の実装
status: active
updated: 2025-11-03
language: ja
owner: yaegashi
---
# Task: Kompox CLI Env の導入と KOM 入力優先順位の実装

## 目的

- [K4x-ADR-015] に基づき、プロジェクトディレクトリ `KOMPOX_DIR` と設定ディレクトリ `KOMPOX_CFG_DIR` の導入、および Git ライクなディスカバリと KOM 入力の単一ソース優先順位を実装する。
- 仕様の一次参照は [Kompox-CLI.ja.md]、KOM の補助仕様は [Kompox-KOM.ja.md] を参照。

## スコープ / 非スコープ

- In:
  - CLI 起動時の `-C` 適用、`--kompox-dir`/`KOMPOX_DIR`、`--kompox-cfg-dir`/`KOMPOX_CFG_DIR` の評価と上方探索実装
  - `KOMPOX_DIR` と `KOMPOX_CFG_DIR` の確定とプロセス環境へのエクスポート
  - KOM 入力の優先順位と単一ソース採用の実装
  - `Defaults.spec.komPath` の境界チェック実装 (解決後が `$KOMPOX_DIR` または `$KOMPOX_CFG_DIR` 配下)
  - 文字列展開 `$KOMPOX_DIR` と `$KOMPOX_CFG_DIR` の統一的処理 (フラグ/環境変数/config/Defaults)
  - ディレクトリスキャンの拡張子フィルタ `.yml`/`.yaml`、除外ディレクトリ、シンボリックリンク解決
  - 仕様に依存しない実装上の安全対策の適用 (大規模入力暴走防止、具体値はコード側のみ)
  - ユニット/統合テストの追加・更新
- Out:
  - 仕様ドキュメントの大幅な再編集 (必要最小限の追随のみ)
  - 既存プロバイダドライバの機能拡張 (本タスクでは扱わない)

## 仕様サマリ

- ディレクトリ解決
  - `-C` は作業ディレクトリの一時切替のみ。
  - `KOMPOX_DIR`: `--kompox-dir` > `KOMPOX_DIR` > 作業ディレクトリから `.kompox/` を含む親を上方探索。
  - `KOMPOX_CFG_DIR`: `--kompox-cfg-dir` > `KOMPOX_CFG_DIR` > `$KOMPOX_DIR/.kompox`。
  - 確定した `KOMPOX_DIR`/`KOMPOX_CFG_DIR` を環境変数にエクスポート。
- KOM 入力は次の優先順位で最初に有効な場所からのみ入力する:
  1. `--kom-path` (複数指定可、ファイルまたはディレクトリ)
  2. 環境変数 `KOMPOX_KOM_PATH` (OS 依存のパス区切り)
  3. Kompox アプリファイル内 Defaults の `spec.komPath`
  4. `.kompox/config.yml` の `komPath`
  5. 既定の KOM ディレクトリ `$KOMPOX_CFG_DIR/kom`
- 境界とポリシー
  - `Defaults.spec.komPath` のみ、解決済み実パスが `$KOMPOX_DIR` または `$KOMPOX_CFG_DIR` 配下である必要がある。
  - `--kom-path`/`KOMPOX_KOM_PATH`/`.kompox/config.yml` の `komPath` には上記境界チェックを適用しない。
  - URL は不可。ローカルのみ。ファイル拡張子は `.yml`/`.yaml`。
  - ディレクトリスキャン時は `.git/`, `.github/`, `node_modules/`, `vendor/`, `.direnv/`, `.venv/`, `dist/`, `build/` を除外。
  - 実装上の安全対策は設けるが、仕様には具体値を記載しない。
- 廃止/変更
  - `baseRoot` と `.kompoxroot` を廃止。
  - `-C` は `--cluster-id` の短縮としては使用しない。

## 実装

### アーキテクチャ設計

モジュール境界を明確にし、既存の CLI フラグ解決パターンに合わせた設計:

1. **`config/kompoxenv` パッケージ** (新規作成)
   - `Env` 型: KOMPOX_DIR, KOMPOX_CFG_DIR, および `.kompox/config.yml` の内容を保持する環境データホルダー
   - `Resolve(kompoxDir, kompoxCfgDir, workDir string) (*Env, error)` 関数: ディレクトリ発見と設定読み込み
   - `Env.ExpandVars(path string) string` メソッド: `$KOMPOX_DIR`/`$KOMPOX_CFG_DIR` の文字列置換
   - `Env.IsWithinBoundary(path string) bool` メソッド: パスが境界内かチェック
   - 定数: `KompoxDirEnvKey`, `KompoxCfgDirEnvKey`, `KompoxCfgDirName`, `ConfigFileName`
   - 将来の拡張: `Env.LogDir()`, `Env.CacheDir()` などのパス管理メソッド

2. **`cmd/kompoxops/main.go` でのフラグ/環境変数解決**
   - `-C` フラグ: 作業ディレクトリ変更 (`os.Chdir()`)
   - `--kompox-dir` フラグ: 優先度 フラグ > `KOMPOX_DIR` 環境変数 > nil
   - `--kompox-cfg-dir` フラグ: 優先度 フラグ > `KOMPOX_CFG_DIR` 環境変数 > nil
   - `PersistentPreRunE` で `kompoxenv.Resolve()` を呼び出し
   - 確定した `KOMPOX_DIR`/`KOMPOX_CFG_DIR` を環境変数にエクスポート (`os.Setenv()`)
   - `Env` をコンテキストに保存し、後続処理で使用

3. **`cmd/kompoxops/kom_loader.go` での KOM 入力優先順位実装**
   - 既存の `findBaseRoot()` と `.kompoxroot`/`.git` 探索を除去
   - コンテキストから `Env` を取得
   - 5段階の優先順位で最初に有効なソースのみを採用:
     1. `--kom-path` フラグ (複数ファイル/ディレクトリ)
     2. `KOMPOX_KOM_PATH` 環境変数 (OS依存区切り文字でパース)
     3. `Defaults.spec.komPath` (`Env.IsWithinBoundary()` で境界チェック必須)
     4. `Env.KOMPath` (`.kompox/config.yml` の `komPath`)
     5. デフォルト: `$KOMPOX_CFG_DIR/kom` (`Env.ExpandVars()` で展開)
   - すべてのパスに `Env.ExpandVars()` を適用
   - ディレクトリスキャン時は既存の除外パターン/拡張子フィルタを適用

### パッケージ構成

```
config/
  kompoxenv/          # 新規パッケージ (kompoxcfg からリファクタリング)
    env.go            # Env 型定義、Resolve() 関数
    expand.go         # ExpandVars() メソッド
    boundary.go       # IsWithinBoundary() メソッド
    env_test.go       # ユニットテスト
```

### データフロー

```
CLI起動
  ↓
main.go: -C フラグ処理 (os.Chdir)
  ↓
main.go: --kompox-dir/--kompox-cfg-dir フラグ解決
  ↓
kompoxenv.Resolve(kompoxDir, kompoxCfgDir, workDir)
  ├─ kompoxDir が nil → 作業ディレクトリから .kompox/ を上方探索
  ├─ kompoxCfgDir が nil → $KOMPOX_DIR/.kompox を使用
  └─ .kompox/config.yml を読み込み
  ↓
main.go: KOMPOX_DIR/KOMPOX_CFG_DIR を環境変数にエクスポート
  ↓
main.go: Env をコンテキストに保存
  ↓
kom_loader.go: 5段階優先順位で KOM 入力ソースを決定
  ├─ 1. --kom-path フラグ
  ├─ 2. KOMPOX_KOM_PATH 環境変数
  ├─ 3. Defaults.spec.komPath (境界チェック)
  ├─ 4. Env.KOMPath
  └─ 5. デフォルト $KOMPOX_CFG_DIR/kom
  ↓
kom_loader.go: 選択されたソースから KOM を読み込み
```

### 主要な型定義

```go
// config/kompoxenv/env.go
package kompoxenv

// Env represents the resolved Kompox project environment.
// It holds directory paths, configuration from .kompox/config.yml,
// and provides path expansion and boundary checking utilities.
type Env struct {
    KompoxDir    string // 確定した KOMPOX_DIR
    KompoxCfgDir string // 確定した KOMPOX_CFG_DIR
    Version      string // .kompox/config.yml の version
    Store        string // .kompox/config.yml の store
    KOMPath      string // .kompox/config.yml の komPath
}

// Resolve discovers and resolves the Kompox environment.
func Resolve(kompoxDir, kompoxCfgDir, workDir string) (*Env, error)

// ExpandVars replaces $KOMPOX_DIR and $KOMPOX_CFG_DIR in the given path.
func (e *Env) ExpandVars(path string) string

// IsWithinBoundary checks if the resolved path is within KOMPOX_DIR or KOMPOX_CFG_DIR.
func (e *Env) IsWithinBoundary(path string) bool

// Future extensions:
// func (e *Env) LogDir() string
// func (e *Env) CacheDir() string
```

## 計画 (チェックリスト)

- [x] `config/kompoxcfg` → `config/kompoxenv` リファクタリング
  - [x] パッケージリネーム (`kompoxdir` → `kompoxenv`)
  - [x] `Resolver` 型を廃止し、`Resolve()` 関数に統合
  - [x] `Config` 型を `Env` 型に変更 (データホルダーのみ)
  - [x] テストを新 API に合わせて更新
  - [x] すべてのユニットテストが passing
- [x] `cmd/kompoxops/main.go` 更新
  - [x] `-C`, `--kompox-dir`, `--kompox-cfg-dir` フラグ追加
  - [x] `PersistentPreRunE` での解決ロジック実装
  - [x] 環境変数エクスポート (`os.Setenv`)
  - [x] コンテキストへの Env 保存 (`kompoxEnvKey`)
  - [x] ヘルパー関数 `getKompoxEnv()` 実装
- [x] `cmd/kompoxops/kom_loader.go` 更新
  - [x] `findBaseRoot()` と `.kompoxroot`/`.git` 探索を除去
  - [x] 5段階優先順位ロジック実装 (`getKOMPathsWithPriority()`)
  - [x] 境界チェックの適用 (Defaults.spec.komPath のみ)
  - [x] パス展開の統一 (Env.ExpandVars)
  - [x] `initializeKOMMode()` の完全な書き直し
- [x] ユニットテスト: `config/kompoxenv` の各機能
  - [x] `TestSearchForKompoxDir` (4 test cases)
  - [x] `TestResolve` (3 test cases)
  - [x] `TestConfig_ExpandVars` (4 test cases)
  - [x] `TestConfig_IsWithinBoundary` (6 test cases)
  - [x] `TestLoadConfigFile` (2 test cases)
- [x] 統合テスト: CLI からのエンドツーエンド
  - [x] `TestInitializeKOMMode` の更新 (コンテキストセットアップ)
  - [x] `TestKOMPathRecursiveDirectoryScan` の更新
  - [x] `TestKOMPathParentDirectoryReference` の更新
  - [x] すべての統合テストが passing
- [x] ビルド検証
  - [x] `make test` 成功
  - [x] `make build` 成功
- [ ] E2E テスト
  - [ ] 既存 E2E テストへの `.kompox/` ディレクトリ構造の組み込み
  - [ ] `KOMPOX_DIR`/`KOMPOX_CFG_DIR` 環境変数のテスト
  - [ ] KOM 入力の5段階優先順位の E2E 検証
  - [ ] 境界チェックの E2E 検証 (Defaults.spec.komPath)
  - [ ] 実際のプロバイダ (AKS/k3s) での動作確認

## 受け入れ条件

- ✅ `--kompox-dir`/`--kompox-cfg-dir` を明示した場合にその値が使用され、未指定時は規定の上方探索/既定により決定されること。
- ✅ 確定した `KOMPOX_DIR`/`KOMPOX_CFG_DIR` がプロセス環境に設定され、パス展開に使用されること。
- ✅ KOM 入力は優先順位の 5 項目のうち最初に有効な 1 ソースのみが採用され、他は無視されること (統合しない)。
- ✅ `Defaults.spec.komPath` のパスは境界内 (`$KOMPOX_DIR` または `$KOMPOX_CFG_DIR`) のみ許容され、境界外はエラーとなること。
- ✅ ディレクトリスキャンの拡張子/除外/シンボリックリンク解決が適用されること。
- ✅ 実装上の安全対策により異常な大規模入力で暴走しないこと (具体値は仕様非依存)。
- ⏳ 既存の回帰が無いこと (主要コマンドのスモークテストが成功) - E2E テストで検証予定

## メモ

- 仕様の詳細は CLI ドキュメントに集約し、ADR は決定事項を簡潔に保つ。
- 具体的な上限値はコード内の定数として実装し、仕様からは外す。
- パッケージ名は `kompoxenv` を採用 (環境変数と .kompox/ ディレクトリ管理の両方を表現)。

## 進捗

- 2025-11-03: タスク作成
- 2025-11-03: 実装計画の詳細化
  - パッケージ配置を `config/kompoxenv` に決定 (新規パッケージ)
  - `kompoxopscfg` はレガシーパッケージなので触らない
- 2025-11-03: 基本機能の実装完了 (config/kompoxenv)
  - `config/kompoxenv` パッケージ作成
  - `Env` 型: KOMPOX_DIR, KOMPOX_CFG_DIR, および .kompox/config.yml の保持
  - `Resolve()` 関数: ディレクトリ発見と設定読み込み
  - `ExpandVars()` メソッド: $KOMPOX_DIR/$KOMPOX_CFG_DIR の文字列置換
  - `IsWithinBoundary()` メソッド: 境界チェック
  - ユニットテスト追加 (全テスト passing)
- 2025-11-03: CLI 統合実装完了
  - `cmd/kompoxops/main.go`: フラグ追加、解決ロジック、環境変数エクスポート、コンテキスト保存
  - `cmd/kompoxops/kom_loader.go`: 5段階優先順位実装、境界チェック、パス展開
  - `findBaseRoot()` と `.kompoxroot`/`.git` 探索を除去
  - 統合テスト更新 (全テスト passing)
- 2025-11-03: パッケージ名の最終決定とリネーム
  - `kompoxdir` → `kompoxenv` にリネーム完了
  - `Config` 型 → `Env` 型に変更
  - 理由: 環境変数と .kompox/ ディレクトリ管理の両方を表現
  - 将来の拡張性: logs/, cache/ などのパス管理に対応
  - すべてのテストとビルドが成功
- 2025-11-03: **コア実装完了**
  - すべてのチェックリスト項目完了 (E2E テスト除く)
  - ユニットテストと統合テストで受け入れ条件を満たす
  - `make test` および `make build` が成功
  - K4x-ADR-015 のコア実装完了
  - 残タスク: E2E テストへの組み込みと実プロバイダでの動作確認


## 参考

- [K4x-ADR-015]
- [Kompox-CLI.ja.md]
- [Kompox-KOM.ja.md]

[K4x-ADR-015]: ../../design/adr/K4x-ADR-015.md
[Kompox-CLI.ja.md]: ../../design/v1/Kompox-CLI.ja.md
[Kompox-KOM.ja.md]: ../../design/v1/Kompox-KOM.ja.md
