# アーキテクチャ意思決定記録 (ADR)

このディレクトリには Kompox のアーキテクチャ意思決定記録 (Architecture Decision Records; ADR) が含まれます。ADR は重要なエンジニアリング上の意思決定と、その背景、選択肢、結果を記録します。内容は簡潔かつ安定しており、詳細な実装計画は別の場所（例: `_dev/tasks/`）に置きます。

言語ポリシー: ADR は常に英語で記述します。日本語ドキュメントは別途（例: `README.ja.md`）に置かれますが、ADR ファイル（`K4x-ADR-###.md`）は英語のみです。

## 対象範囲と目的

- 公開挙動、CLI の UX、データ契約、プロバイダー/ドライバーのインターフェイス、またはアーキテクチャ境界に影響する意思決定を記録する
- ADR は短く保つ（おおよそ 0.5〜1 ページ）。詳細は仕様やタスク文書へリンクし、重複を避ける

## ファイル命名と構成

- ファイル名: `K4x-ADR-###.md`（数値 ID のみ）。タイトルは文書ヘッダ/フロントマター内に記す
- 1 ファイル 1 ADR、1 ADR 1 決定
- 推奨ヘッダ（フロントマターまたは冒頭セクション）:
  - id（形式: `K4x-ADR-###`。ファイル名の先頭部と同一）
  - title
  - status
  - updated（timestamp）: UTC の ISO 8601 `YYYY-MM-DDTHH:MM:SSZ`
  - tasks（任意）: 関連 task の doc-id
  - plans（任意）: 関連 plan の doc-id

## ステータスのライフサイクル

ADR グループでは次のステータス値を使用します:
- proposed: 検討中で未確定
- accepted: 決定済みで有効
- rejected: 実施しないと判断
- deprecated: 推奨しないが履歴のために残す

継承関係は以下の専用ヘッダ（後述）で表し、`superseded` ステータスは使いません。

ガイダンス:
- まずは `proposed` で開始。意思決定が固まったら `accepted` に変更
- 採用しない場合は `rejected` にする
- 内容は真であるが推奨しない場合は `deprecated` を使う

### 継承（Supersession）ヘッダ

- `supersedes`: この ADR が置き換える ADR の完全な ID（例: `K4x-ADR-001` または `[K4x-ADR-001, K4x-ADR-007]`）
- `supersededBy`: この ADR を置き換える ADR の完全な ID（例: `K4x-ADR-009` または `[K4x-ADR-009]`）

注意:
- 古い ADR と新しい ADR の双方でステータスは `accepted` のままとし、関係は上記ヘッダで表現する
- 可能であれば相互リンクを追加する（例: `K4x-ADR-008` が `supersededBy: K4x-ADR-010` を持ち、`K4x-ADR-010` が `supersedes: K4x-ADR-008` を持つ）
- 明確性と検索性のため、ヘッダでは完全な ID（`K4x-ADR-###`）を用いる

## 執筆ガイドライン

- 「どうやって」よりも「なぜ」に焦点を当てる
- 代替案とトレードオフを簡潔に記録する
- 制約（セキュリティ、コンプライアンス、プロバイダーの制限、後方互換性）を明確にする
- 関連する仕様、タスク、コードへのリンクを提供する
- このルールの対象は、front-matter に `id:` を持つ markdown ドキュメントのみ。
- `README.md`、`README.ja.md`、`GUIDE.md`、`GUIDE.ja.md` は対象外。

## テンプレート

```markdown
---
id: K4x-ADR-<###>
title: <Short decision title>
status: proposed | accepted | rejected | deprecated
updated: 2026-02-14T00:00:00Z
supersedes: <K4x-ADR-### | [K4x-ADR-###, K4x-ADR-###]>
supersededBy: <K4x-ADR-### | [K4x-ADR-###, K4x-ADR-###]>
tasks: <YYYYMMDDa-... | [YYYYMMDDa-..., YYYYMMDDb-...]>
plans: <YYYYaa-... | [YYYYaa-..., YYYYab-...]>
---

## Context

- ...

## Decision

- ...

## Alternatives Considered

- ...

## Consequences

- Pros: ...
- Cons/Constraints: ...

## Rollout

- ... (phased plan if applicable)

## References

- ... (links to tasks/specs/PRs)
```

## インデックス

- `design/gen/README.*` のインデックスを最新に保つ。ADR は ID、タイトル、言語、ステータスとともに ADR セクションに表示される。
