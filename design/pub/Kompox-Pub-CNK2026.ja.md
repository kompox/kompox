---
id: Kompox-Pub-202603-CNK
title: Cloud Native Kaigi
status: draft
updated: 2026-03-02T12:30:47Z
language: ja
---

# Cloud Native Kaigi

## イベント概要

- タイトル: クラウドネイティブ会議
- 日時: 2026年5月14日(木)・15日(金)
- 場所: 中日ホール & カンファレンス (名古屋)
- CFP締切: 2026年3月1日 25:59 JST
- URL: https://kaigi.cloudnativedays.jp/

## セッション募集要項

- セッション時間: 25分 + Q&A 5分
- 登壇方法: 会場（中日ホール&カンファレンス）での現地登壇のみ
- 一人で複数応募: 可能です。その際、複数のトラックへの応募も可能です。ただし、採択されるのは一人1セッションまでとなります。
- 複数人でのセッション: 可能です。応募の際は代表者の方がお申し込みください。
- 日本語以外での発表: 日本語または英語でご応募いただけます。ただし、通訳の用意はありませんので、あらかじめご了承ください。
- 動画・スライドの公開可否: スライドおよびセッション動画のアーカイブ公開可否は、ご選択いただけます。 ただし、動画アーカイブを希望されない場合でも当日のご講演内容はライブ配信されますので、あらかじめご了承ください。

次の3トラックで開催される。ひとつのプロポーザルを複数のトラックに提出することも可能。

- [CloudNative トラック (Day 1)](https://kaigi.cloudnativedays.jp/cfp/cloudnative/)
- [Platform Engineering トラック (Day 2)](https://kaigi.cloudnativedays.jp/cfp/platform-engineering/)
- [SRE トラック (Day 2)](https://kaigi.cloudnativedays.jp/cfp/sre/)

セッションプロポーザルに提出する必要がある情報:

- タイトル (最大60文字)
- 講演内容 (最大500文字)
- 受講者レベル (初級者、中級者、上級者、全て)
- 実行フェイズ: 以下から複数選択
  - Dev/QA (開発環境)
  - PoC (検証)
  - Production (本番環境)
  - Other

## 2026/03/01 時点の Kompox に関する情報

- ステートフルワークロード運用に特化した Kompox の設計と開発ロードマップをメイントピックとする。
- Kubernetes 界隈ではあまり一般的なトピックではないため、その点をプロポーザルで理解してもらう必要がある。
- まだ本格的なプロダクション運用は始まっていないが、内部組織向けには利用しているので、その経験を共有することは可能。
- 実装状況: AKS (Azure) 向けが当初想定機能のほとんどを実装し、実運用テストも開始している。OKE (Oracle) 向けは設計ドキュメントを作成中で、実装はこれから。
- 次の MkDocs ドキュメントを基本にする。
  - [Kompox ホーム](../../docs/index.ja.md)
  - [Kompox ストーリー](../../docs/stories/index.ja.md)
- 下記の情報・URL はプロポーザル本文で使用してよい。
  - プロジェクトリポジトリ: https://github.com/kompox/kompox
  - MkDocsドキュメントサイト URL: https://docs.kompox.dev/edge/ja/
- 「参照」セクションに記した過去の勉強会やカンファレンスのプロポーザルも参考にする。

## Draft 1

### タイトル (50文字)

Pets on Kubernetes ― RWOボリュームで「飼う」ステートフルアプリ設計の現実解

### 講演内容 (500文字)

「Cattle, not Pets」はKubernetesアプリ設計の常識ですが、その前提となる高可用性サービスに頼れないステートフルアプリ(Git/P4、CI・開発環境、レガシー等)は多くの組織に残っており、それらをVMから脱却させるにはK8sのReadWriteOnce(RWO) PV+シングルレプリカPodで「飼う」しかないケースがあります。

本講演ではこのトピックを掘り下げた次の内容をお話します。

・Kompoxの紹介: マルチクラウドK8sアプリ運用ツール https://docs.kompox.dev/edge/ja/
・K8sの宣言的管理・自動復旧・API駆動インフラを活用したcompose.yml中心のアプリ開発ワークフロー
・HPAやローリングアップデートと無縁なRPO≈0・SLO 99.9%の割り切った可用性設計
・AKSにおけるRWOのAZ制約・デタッチ・スナップショット挙動の現実
・CRD/Operator化とPaaS展開構想

K8sの能力をあえて使わないことがステートフルアプリ設計の現実解になり得ることを、汎用的な設計パターンとして持ち帰りいただけます。

### 受講者レベル

中級者

### 実行フェイズ

Dev/QA, PoC

### 講演者プロフィール (300文字)

Linux・Unix・OSS・Go言語など低レイヤ技術好きのエンジニア。組み込みやゲームサーバ開発を経て、社内IT環境の改善やクラウド移行支援に従事。

クラウドVMとDocker Composeで多数の社内向けステートフルアプリを運用してきた経験から、現在はそのクラウドネイティブ化に取り組み、マルチクラウドK8sアプリ運用ツールKompoxを開発しています。Kompoxの開発の経緯とロードマップについては https://docs.kompox.dev/edge/ja/stories/ を参照してください。

Microsoft MVP for Microsoft Azure (2023-2025)

### 過去の登壇実績がわかるスライド等

- https://www.docswell.com/s/yaegashi/59G36V-pfem13
- https://www.docswell.com/s/yaegashi/544R21-KNT38-Kompox
- https://www.docswell.com/s/yaegashi/59MP69-github-universe-recap-tokyo-2025

## 参照

- [Kubernetes Novice Tokyo #38] - 勉強会
- [CloudNative Days Winter 2025] - 前回のカンファレンス (非採択)

[Kubernetes Novice Tokyo #38]: ./Kompox-Pub-k8snovice38.ja.md
[CloudNative Days Winter 2025]: ./Kompox-Pub-CNDW2025.ja.md
