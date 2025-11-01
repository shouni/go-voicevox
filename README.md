# ✍️ Go VOICEVOX

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-voicevox)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-voicevox)](https://github.com/shouni/go-voicevox/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

# VOICEVOX音声ファイル作成ツール

## 🚀 プロジェクトの処理概要

### 1\. 起動と設定の読み込み (`cmd`, `main.go`, `internal/config`)

1.  **起動:** `main.go`が起動し、CLIコマンド構造を実行します。

-----

## 🌳 プロジェクト構成ツリー図 (`git-reviewer-go`)

## 📄 ファイルごとの役割説明

### 3\. `pkg` ディレクトリ (汎用ライブラリ - 再利用可能)

| 機能カテゴリ（パッケージ名） | 構成ファイル | 役割 |
| :--- | :--- | :--- |
| **`httpkit`** | `client.go` `const.go` `error.go` 他 | **堅牢なHTTPクライアント**。`ai/gemini`や`notifier`などの他の`pkg`内クライアントが利用する、リトライ、エラーハンドリング、リクエスト/レスポンス処理など、汎用的なHTTP通信のロジックを提供します。 |
| **`utils/text`** | `sanitize.go` `text_test.go` | **テキスト処理ユーティリティ**。ユーザー入力やAPIレスポンスのテキストを整形・無害化するための関数を定義します。 |
