# ✍️ Go VOICEVOX

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-voicevox)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-voicevox)](https://github.com/shouni/go-voicevox/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## 🚀 プロジェクトの処理概要

本ツールは、入力されたスクリプトを解析し、VOICEVOXエンジンと連携して並列で音声合成を行い、単一のWAVファイルとして出力するプロセスを自動化します。

1.  **起動と設定の読み込み** (`cmd`): `main.go` が起動し、CLIコマンド構造を実行します。
2.  **VOICEVOX Executorの初期化** (`voicevox/factory.go`): VOICEVOX API URLの決定、`api.Client` の初期化、`speaker.DataFinder` のロードを統括し、実行に必要な依存関係（`engine.EngineExecutor`）を組み立てます。
3.  **スクリプト解析** (`voicevox/parser`): 入力スクリプトを話者タグ（例：`[ずんだもん]`）に基づいて複数のセグメントに分割します。（**文字数による自動分割ロジックを含む**）
4.  **音声合成処理** (`voicevox/engine`):
    * **Functional Options** を適用し、フォールバックタグなどの設定を決定した後、セグメントごとに並列処理を開始します。
    * **【✨ 堅牢性向上】** 並列処理に際し、**セマフォ**による**同時実行数の制限**に加え、**時間ベースのレートリミッター**を導入しました。これにより、VOICEVOXエンジンへの過負荷を防ぎ、処理の安定性とエラー耐性を向上させています。また、API待機中に親コンテキストがキャンセルされた場合、Goroutineは即座に終了します。
    * `api.Client` を利用し、テキストとスタイルIDを元に `/audio_query` を呼び出し、音声クエリJSONを取得します。
    * 取得したクエリJSONとスタイルIDを元に `/synthesis` を呼び出し、個々のWAVデータ（バイトスライス）を取得します。
5.  **WAV結合** (`voicevox/audio`): 並列処理で取得されたすべてのWAVデータを結合し、ヘッダー情報（ファイルサイズ、データサイズ）を再計算して、単一の有効なWAVファイルを構築します。
6.  **ファイル出力** (`voicevox/engine`): 最終的な結合済みWAVファイルを指定されたパスに、**必要に応じてディレクトリを作成**して保存します。

-----

## 🌳 プロジェクト構成ツリー図

このツリー図は、**`go-voicevox`** プロジェクトのコアロジックを格納する **`pkg`** ディレクトリ内の、リファクタリング後の主要なファイル構成を示しています。

```

go-voicevox/
├── cmd/
│   └── main.go      # 実行エントリポイントとCLI構造の実行
├── internal/        # (設定ファイルなど)
└── pkg/
    └── voicevox/        # VOICEVOXクライアントライブラリ本体
        ├── api/             # API通信とデータモデル
        │   ├── client.go    # VOICEVOX APIクライアント (httpkit依存)
        │   ├── error.go     # API通信、応答、JSON解析のカスタムエラー
        │   └── model.go     # API応答のデータモデル
        ├── audio/           # WAVデータ処理ロジック
        │   ├── audio.go     # WAVデータの結合とヘッダー処理
        │   └── const.go     # WAV構造に関する定数
        ├── parser/          # スクリプト解析ロジック
        │   ├── const.go     # 解析に関する定数
        │   └── parser.go    # スクリプトのセグメント化ロジック
        ├── speaker/         # 話者データとスタイルIDの管理
        │   ├── const.go     # サポート対象話者、スタイルタグの静的定義
        │   ├── error.go     # 必須フィールド不足など、ロード時のカスタムエラー
        │   ├── loader.go    # /speakers エンドポイントからのデータロードロジック
        │   └── model.go     # SpeakerData (DataFinder 実装) などのデータ構造
        ├── engine.go        # コア処理エンジン、バッチ処理、Functional Options定義
        ├── factory.go       # Executorの初期化と依存関係の構築
        └── model.go         # EngineExecutor, EngineConfig などのコアインターフェース/構造体

```

-----

## 📄 ファイルごとの役割説明 (リファクタリング後)

### 3\. `pkg/voicevox` サブパッケージ

| パッケージ名 | 構成ファイル | 役割 |
| :--- | :--- | :--- |
| **`voicevox`** (ルート) | `factory.go` | **初期化ファクトリ**。VOICEVOX URL決定、`api.Client`、`speaker.DataFinder` の初期化・結合を行い、**実行器 (`engine.EngineExecutor`) を組み立て**ます。 |
| | `engine.go` | **コア処理エンジン**。スクリプト解析、並列音声合成の実行、エラー集約、WAV結合、最終的なファイル書き込みを統括します。**レートリミッター制御**と**セマフォ**による堅牢な並行処理ロジックを含みます。`ExecuteOption` もここで定義されます。 |
| | `model.go` | **コアモデル/インターフェース**。`EngineExecutor`、`EngineConfig` などのルートレベルのコアインターフェースと構造体を定義し、責務分離を支えます。 |
| **`api`** | `client.go`, `error.go`, `model.go` | **VOICEVOX API通信層**。`/audio_query`、`/synthesis` などのAPIリクエスト実行、`httpkit.Client` によるリトライ処理、通信/応答/JSON解析エラーの定義を担当します。 |
| **`audio`** | `audio.go`, `const.go` | **WAVデータ処理層**。複数のWAVファイルバイトスライスからオーディオデータを抽出し、正しいヘッダーを持つ単一のWAVファイルに結合するロジックを提供します。 |
| **`parser`** | `parser.go`, `const.go` | **スクリプト解析層**。入力スクリプトを話者タグに基づいて複数のセグメントに分割するロジック、文字数制限に基づく自動分割ロジックを提供します。 |
| **`speaker`** | `loader.go`, `model.go`, `const.go`, `error.go` | **話者データ管理層**。`/speakers` から話者・スタイルIDを取得し、スタイルID検索のためのデータ構造 (`model.SpeakerData` が `engine.DataFinder` を実装) を構築・提供します。 |

-----

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。
