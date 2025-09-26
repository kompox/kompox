---
title: 'Kubernetes Novice Tokyo #38'
version: pub
status: reference
updated: 2025-09-26
language: ja
---

# Kubernetes Novice Tokyo #38

## 概要

- https://k8s-novice-jp.connpass.com/event/365526/
- Kubernetes 初心者向けのオンライン勉強会
- セッション枠 20 分
- LT 枠 10 分

## Draft 1

Title:

Kompox の紹介

Abstract:

個人開発で作ってみた Kubernetes 向けのコンテナ Web アプリホスティング・デプロイツール Kompox について紹介します。
Kompox は Linux VM + Docker Compose による手軽な DevOps 体験をクラウド各社の K8s クラスタ上のステートフルアプリでも再現することを目指しています。
特に力を入れている RWO な PV/PVC/Snapshot のライフサイクル管理及び可用性ゾーン対応設計と、GitHub Copilot Agent による Go 言語ソフト開発についてお話できればと思います。

## Final

Title:

Kompox: クラウドネイティブコンテナ Web アプリ DevOps ツールの紹介

Abstract:

個人開発で作ってみた、クラウドネイティブかつデータ永続化を伴うステートフルなコンテナ Web アプリ向けの DevOps ツールである Kompox について紹介します。
Kompox は従来の Linux VM + Docker Compose による手軽な DevOps 体験を、クラウド各社の Kubernetes クラスタ上のステートフルアプリケーションでも再現することを目指しています。
特に、RWO（ReadWriteOnce）な永続ボリューム（PV/PVC）とスナップショットのライフサイクル管理、可用性ゾーン対応設計、そして GitHub Copilot Agent を活用したソフトウェアアーキテクチャの設計と Go 言語による実装について、これまでの開発経験を元にお話できればと思います。

## 登壇資料

- [Kompox: クラウドネイティブコンテナ Web アプリ DevOps ツールの紹介](https://www.docswell.com/s/yaegashi/544R21-KNT38-Kompox)