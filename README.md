# ✍️ Go VOICEVOX

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-voicevox)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-voicevox)](https://github.com/shouni/go-voicevox/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## 🚀 プロジェクトの処理概要

本ツールは、入力されたスクリプトを解析し、VOICEVOXエンジンと連携して並列で音声合成を行い、単一のWAVファイルとして出力するプロセスを自動化します。

1.  **起動と設定の読み込み** (`cmd`): `main.go` が起動し、CLIコマンド構造を実行します。
2.  **話者データのロード** (`speaker_loader.go`): VOICEVOX APIの `/speakers` エンドポイントから利用可能な話者とスタイルIDを取得し、内部の検索用データ構造を構築します。
3.  **スクリプト解析** (`script_parser.go`): 入力スクリプトを話者タグ（例：`[ずんだもん]`）に基づいて複数のセグメントに分割します。
4.  **音声合成処理** (`engine.go`):
    * **Functional Options** を適用し、フォールバックタグなどの設定を決定した後、セグメントごとに並列処理を開始します。
    * `client.go` を利用し、テキストとスタイルIDを元に `/audio_query` を呼び出し、音声クエリJSONを取得します。
    * 取得したクエリJSONとスタイルIDを元に `/synthesis` を呼び出し、個々のWAVデータ（バイトスライス）を取得します。
5.  **WAV結合** (`audio.go`): 並列処理で取得されたすべてのWAVデータを結合し、ヘッダー情報（ファイルサイズ、データサイズ）を再計算して、単一の有効なWAVファイルを構築します。
6.  **ファイル出力** (`engine.go`): 最終的な結合済みWAVファイルを指定されたパスに、**必要に応じてディレクトリを作成**して保存します。

-----

## 🌳 プロジェクト構成ツリー図

このツリー図は、**`go-voicevox`** プロジェクトのコアロジックを格納する **`pkg`** ディレクトリ内の主要なファイル構成を示しています。

```
go-voicevox/
├── cmd/
│   └── main.go       # 実行エントリポイントとCLI構造の実行
├── internal/         # (設定ファイルなど)
├── pkg/
│   └── voicevox/     # VOICEVOXクライアントライブラリ本体
│       ├── audio.go          # WAVデータの結合とヘッダー処理
│       ├── client.go         # VOICEVOX APIクライアント
│       ├── const.go          # 定数定義 (WAVヘッダー, スタイルタグ, etc.)
│       ├── engine.go         # コア処理エンジン/バッチ処理
│       ├── error.go          # カスタムエラー定義
│       ├── model.go          # データモデルとインターフェース (DataFinder, Parser)
│       ├── script_parser.go  # スクリプト解析ロジック
│       └── speaker_loader.go # 話者データのロード
└── README.md
```

-----

## 📄 ファイルごとの役割説明

### 3\. `pkg` ディレクトリ (汎用ライブラリ - 再利用可能)

| 機能カテゴリ（パッケージ名） | 構成ファイル | 役割 |
| :--- | :--- | :--- |
| **`voicevox`** | `engine.go` | **コア処理エンジン**。**Functional Options** を適用し、スクリプト解析、並列音声合成処理の実行、エラー集約、WAV結合を統括します。また、ファイル書き込み時に**出力ディレクトリの作成**も担当します。 |
| | `client.go` | **VOICEVOX APIクライアント**。`/audio_query`、`/synthesis` などのAPIリクエストを処理し、`httpkit.Client` を利用してリトライ機能を適用します。 |
| | `audio.go` | **WAVデータ処理**。複数のWAVファイルバイトスライスからオーディオデータを抽出し、正しいヘッダーを持つ単一のWAVファイルに結合するロジックを提供します。 |
| | `speaker_loader.go` | **話者データローダー**。`/speakers` エンドポイントから話者・スタイルIDを取得し、スタイルID検索のためのデータ構造 (`SpeakerData`) を構築します。 |
| | `model.go` | **データモデル/インターフェース**。`SpeakerData` などのデータ構造と、`DataFinder`、`Parser` などのコアインターフェースを定義し、責務分離を支えます。 |
| | `const.go` | **定数定義**。WAVヘッダーサイズ、サポート対象話者、スタイルタグ、APIタイムアウトなど、すべての静的定数を一元管理します。 |
| | `error.go` | **カスタムエラー**。API通信失敗、JSON解析失敗、WAVヘッダーエラー、バッチ処理エラーなど、パッケージ全体で使用されるカスタムエラー型を定義します。 |

-----

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。
