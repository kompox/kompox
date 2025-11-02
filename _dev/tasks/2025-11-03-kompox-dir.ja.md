---
id: 2025-11-03-kompox-dir
title: KOMPOX_DIR/KOMPOX_CFG_DIR と入力優先順位の実装
status: active
updated: 2025-11-03
language: ja
owner:
---
# Task: KOMPOX_DIR/KOMPOX_CFG_DIR と入力優先順位の実装

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

1. **`config/kompoxdir` パッケージ** (新規作成)
   - `Config` 型: KOMPOX_DIR, KOMPOX_CFG_DIR, および `.kompox/config.yml` の内容を保持するデータホルダー
   - `Resolve(kompoxDir, kompoxCfgDir, workDir string) (*Config, error)` 関数: ディレクトリ発見と設定読み込み
   - `Config.ExpandVars(path string) string` メソッド: `$KOMPOX_DIR`/`$KOMPOX_CFG_DIR` の文字列置換
   - `Config.IsWithinBoundary(path string) bool` メソッド: パスが境界内かチェック
   - 定数: `KompoxDirEnvKey`, `KompoxCfgDirEnvKey`, `KompoxCfgDirName`, `ConfigFileName`

2. **`cmd/kompoxops/main.go` でのフラグ/環境変数解決**
   - `-C` フラグ: 作業ディレクトリ変更 (`os.Chdir()`)
   - `--kompox-dir` フラグ: 優先度 フラグ > `KOMPOX_DIR` 環境変数 > nil
   - `--kompox-cfg-dir` フラグ: 優先度 フラグ > `KOMPOX_CFG_DIR` 環境変数 > nil
   - `PersistentPreRunE` で `kompoxdir.Resolve()` を呼び出し
   - 確定した `KOMPOX_DIR`/`KOMPOX_CFG_DIR` を環境変数にエクスポート (`os.Setenv()`)
   - `Config` をコンテキストに保存し、後続処理で使用

3. **`cmd/kompoxops/kom_loader.go` での KOM 入力優先順位実装**
   - 既存の `findBaseRoot()` と `.kompoxroot`/`.git` 探索を除去
   - コンテキストから `Config` を取得
   - 5段階の優先順位で最初に有効なソースのみを採用:
     1. `--kom-path` フラグ (複数ファイル/ディレクトリ)
     2. `KOMPOX_KOM_PATH` 環境変数 (OS依存区切り文字でパース)
     3. `Defaults.spec.komPath` (`Config.IsWithinBoundary()` で境界チェック必須)
     4. `Config.KOMPath` (`.kompox/config.yml` の `komPath`)
     5. デフォルト: `$KOMPOX_CFG_DIR/kom` (`Config.ExpandVars()` で展開)
   - すべてのパスに `Config.ExpandVars()` を適用
   - ディレクトリスキャン時は既存の除外パターン/拡張子フィルタを適用

### パッケージ構成

```
config/
  kompoxdir/          # 新規パッケージ (kompoxcfg からリファクタリング)
    config.go         # Config 型定義、Resolve() 関数
    expand.go         # ExpandVars() メソッド
    boundary.go       # IsWithinBoundary() メソッド
    config_test.go    # ユニットテスト
```

### データフロー

```
CLI起動
  ↓
main.go: -C フラグ処理 (os.Chdir)
  ↓
main.go: --kompox-dir/--kompox-cfg-dir フラグ解決
  ↓
kompoxdir.Resolve(kompoxDir, kompoxCfgDir, workDir)
  ├─ kompoxDir が nil → 作業ディレクトリから .kompox/ を上方探索
  ├─ kompoxCfgDir が nil → $KOMPOX_DIR/.kompox を使用
  └─ .kompox/config.yml を読み込み
  ↓
main.go: KOMPOX_DIR/KOMPOX_CFG_DIR を環境変数にエクスポート
  ↓
main.go: Config をコンテキストに保存
  ↓
kom_loader.go: 5段階優先順位で KOM 入力ソースを決定
  ├─ 1. --kom-path フラグ
  ├─ 2. KOMPOX_KOM_PATH 環境変数
  ├─ 3. Defaults.spec.komPath (境界チェック)
  ├─ 4. Config.KOMPath
  └─ 5. デフォルト $KOMPOX_CFG_DIR/kom
  ↓
kom_loader.go: 選択されたソースから KOM を読み込み
```

### 主要な型定義

```go
// config/kompoxdir/config.go
package kompoxdir

type Config struct {
    KompoxDir    string // 確定した KOMPOX_DIR
    KompoxCfgDir string // 確定した KOMPOX_CFG_DIR
    Version      string // .kompox/config.yml の version
    Store        string // .kompox/config.yml の store
    KOMPath      string // .kompox/config.yml の komPath
}

func Resolve(kompoxDir, kompoxCfgDir, workDir string) (*Config, error)
func (c *Config) ExpandVars(path string) string
func (c *Config) IsWithinBoundary(path string) bool
```

## 計画 (チェックリスト)

- [ ] `config/kompoxcfg` → `config/kompoxdir` リファクタリング
  - パッケージリネーム
  - `Resolver` 型を廃止し、`Resolve()` 関数に統合
  - `Config` 型を簡素化 (データホルダーのみ)
  - テストを新 API に合わせて更新
- [ ] `cmd/kompoxops/main.go` 更新
  - `-C`, `--kompox-dir`, `--kompox-cfg-dir` フラグ追加
  - `PersistentPreRunE` での解決ロジック実装
  - 環境変数エクスポート (`os.Setenv`)
  - コンテキストへの Config 保存
- [ ] `cmd/kompoxops/kom_loader.go` 更新
  - `findBaseRoot()` と `.kompoxroot`/`.git` 探索を除去
  - 5段階優先順位ロジック実装
  - 境界チェックの適用 (Defaults.spec.komPath のみ)
  - パス展開の統一 (Config.ExpandVars)
- [ ] ユニットテスト: `config/kompoxdir` の各機能
- [ ] 統合テスト: CLI からのエンドツーエンド

## 受け入れ条件

- `--kompox-dir`/`--kompox-cfg-dir` を明示した場合にその値が使用され、未指定時は規定の上方探索/既定により決定されること。
- 確定した `KOMPOX_DIR`/`KOMPOX_CFG_DIR` がプロセス環境に設定され、パス展開に使用されること。
- KOM 入力は優先順位の 5 項目のうち最初に有効な 1 ソースのみが採用され、他は無視されること (統合しない)。
- `Defaults.spec.komPath` のパスは境界内 (`$KOMPOX_DIR` または `$KOMPOX_CFG_DIR`) のみ許容され、境界外はエラーとなること。
- ディレクトリスキャンの拡張子/除外/シンボリックリンク解決が適用されること。
- 実装上の安全対策により異常な大規模入力で暴走しないこと (具体値は仕様非依存)。
- 既存の回帰が無いこと (主要コマンドのスモークテストが成功)。

## メモ

- 仕様の詳細は CLI ドキュメントに集約し、ADR は決定事項を簡潔に保つ。
- 具体的な上限値はコード内の定数として実装し、仕様からは外す。

## 進捗

- 2025-11-03: タスク作成
- 2025-11-03: 実装計画の詳細化
  - パッケージ配置を `config/kompoxcfg` に決定 (新規パッケージ)
  - `kompoxopscfg` はレガシーパッケージなので触らない
- 2025-11-03: 基本機能の実装完了 (config/kompoxcfg)
  - `config/kompoxcfg` パッケージ作成
  - `Resolver` 型: ディレクトリ解決、環境エクスポート、パス展開、境界チェック
  - `.kompox/config.yml` の読み込み機能
  - ユニットテスト追加 (全テスト passing)
- 2025-11-03: 設計見直し
  - モジュール境界の再検討
  - `config/kompoxcfg` を `config/kompoxdir` にリファクタリング予定
  - main パッケージでフラグ/環境変数解決を行う方針に変更
  - `Config` 型で KOMPOX_DIR/KOMPOX_CFG_DIR と .kompox/config.yml の内容を保持
  - `Resolve()` 関数で Config を返すシンプルな API に変更


## 参考

- [K4x-ADR-015]
- [Kompox-CLI.ja.md]
- [Kompox-KOM.ja.md]

[K4x-ADR-015]: ../../design/adr/K4x-ADR-015.md
[Kompox-CLI.ja.md]: ../../design/v1/Kompox-CLI.ja.md
[Kompox-KOM.ja.md]: ../../design/v1/Kompox-KOM.ja.md
