# ✍️ Go VOICEVOX

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/go-voicevox)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/go-voicevox)](https://github.com/shouni/go-voicevox/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## 🚀 概要 (About) - 多彩な「声」を、Go で自在に操る。

Go VOICEVOX は、**VOICEVOX エンジン**の API を直感的かつ効率的に操作するための Go 言語製クライアントライブラリです。

複雑な音声合成のパラメータ調整、複数話者のスタイル管理、そして WAV データの結合処理までをひとつのパッケージに凝縮。
`httpkit` ベースの堅牢な通信基盤により、デスクトップアプリからバックエンドのバッチ処理まで、あらゆる Go プロジェクトに「命を吹き込む声」を簡単に組み込めます。

---

## ✨ 提供機能 (Features)

* **直感的なエンジン操作**: 煩雑な API エンドポイントの呼び出しを隠蔽し、`Synthesize` メソッドひとつで音声生成を完結させます。
* **高度な話者・スタイル解決 (`package speaker`)**: スタイル ID や話者名を型安全に管理。動的な話者データのロードと検索をサポートします。
* **ストリーミング & バッチ処理**: 大量のテキストをセグメント化して並列生成し、効率的に音声化するパイプラインを提供。
* **WAV オーディオ結合 (`package audio`)**: 複数の音声断片を、ヘッダー情報を維持したままシームレスに結合するロジックを内蔵。
* **柔軟なインプット解析 (`package parser`)**: 長文スクリプトを適切な長さで分割し、エンジンへの負荷を最適化します。
* **プラグイン可能な通信層**: 内部で `go-http-kit` を採用しており、リトライ処理やセキュリティ検証が標準で組み込まれています。

---

## 🚀 プロジェクトの処理概要

本ツールは、入力されたスクリプトを解析し、VOICEVOXエンジンと連携して並列で音声合成を行い、単一のWAVファイルとして出力するプロセスを自動化します。

1.  **起動と設定の読み込み** (`cmd`): `main.go` が起動し、CLIコマンド構造を実行します。
2.  **VOICEVOX Executorの初期化** (`voicevox/factory.go`): VOICEVOX API URLの決定、`api.Client` の初期化、`speaker.DataFinder` のロード、**および `remoteio.OutputWriter` の取得**を統括し、実行に必要な依存関係（`engine.EngineExecutor`）を組み立てます。
3.  **スクリプト解析** (`voicevox/parser`): 入力スクリプトを話者タグ（例：`[ずんだもん]`）に基づいて複数のセグメントに分割します。（**文字数による自動分割ロジックを含む**）
4.  **音声合成処理** (`voicevox/engine`):
    * **Functional Options** を適用し、フォールバックタグなどの設定を決定した後、セグメントごとに並列処理を開始します。
    * **堅牢性向上** 並列処理に際し、**セマフォ**による**同時実行数の制限**に加え、**時間ベースのレートリミッター**を導入しました。これにより、VOICEVOXエンジンへの過負荷を防ぎ、処理の安定性とエラー耐性を向上させています。また、API待機中に親コンテキストがキャンセルされた場合、Goroutineは即座に終了します。
    * `api.Client` を利用し、テキストとスタイルIDを元に `/audio_query` を呼び出し、音声クエリJSONを取得します。
    * 取得したクエリJSONとスタイルIDを元に `/synthesis` を呼び出し、個々のWAVデータ（バイトスライス）を取得します。
5.  **WAV結合** (`voicevox/audio`): 並列処理で取得されたすべてのWAVデータを結合し、ヘッダー情報（ファイルサイズ、データサイズ）を再計算して、単一の有効なWAVファイルを構築します。
6.  **ファイル出力** (`voicevox/engine`): 最終的な結合済みWAVファイルを、注入された`remoteio.OutputWriter` を利用して出力します。これにより、出力先がローカルファイルだけでなく、**Google Cloud Storage (GCS) などのリモートストレージ**にも対応可能となりました。

---

## 🔄 処理シーケンス図

```mermaid
sequenceDiagram
    autonumber
    participant Main as cmd (main.go)
    participant Factory as voicevox/factory
    participant Speaker as speaker/loader
    participant Parser as parser/parser
    participant Engine as voicevox/engine
    participant API as api/client
    participant VV as VOICEVOX Engine
    participant WAV as api/audio
    participant Storage as remoteio.Writer (GCS/Local)

    Note over Main, Storage: 1. 初期化フェーズ
    Main->>Storage: gcs.New(ctx) / Writer()
    Main->>Factory: NewEngineExecutor(ctx, httpClient, writer, voicevoxOutput, opts...)
    activate Factory
    Factory->>API: NewClient(httpClient, apiURL)
    Factory->>Speaker: LoadSpeakers(ctx, apiClient)
    Speaker->>API: GetSpeakers(ctx)
    API->>VV: GET /speakers
    VV-->>API: Speakers JSON
    API-->>Speaker: Speakers JSON
    Speaker-->>Factory: SpeakerData
    Factory-->>Main: EngineExecutor (Engine or No-op)
    deactivate Factory

    Note over Main, Storage: 2. 解析フェーズ
    Main->>Engine: Execute(ctx, script, outputPath)
    activate Engine
    Engine->>Parser: Parse(script, fallbackTag)
    Parser-->>Engine: []Segment
    Engine->>Speaker: GetStyleID / GetDefaultTag (キャッシュ付き解決)

    Note over Main, Storage: 3. 並列音声合成フェーズ (errgroup.SetLimit + rate.Limiter)
    rect rgb(240, 240, 240)
        par 各セグメントの処理
            Engine->>Engine: limiter.Wait + context.WithTimeout
            Engine->>API: RunAudioQuery(text, styleID)
            API->>VV: POST /audio_query
            VV-->>API: Query JSON
            API-->>Engine: Query JSON
            
            Engine->>API: RunSynthesis(query, styleID)
            API->>VV: POST /synthesis
            VV-->>API: WAV Data (bytes)
            API-->>Engine: WAV Data (bytes)
        end
    end

    Note over Main, Storage: 4. 結合・出力フェーズ
    Engine->>WAV: CombineWavData(wavs)
    WAV-->>Engine: Combined WAV bytes
    Engine->>Storage: Write(ctx, outputPath, reader, "audio/wav")
    Storage-->>Engine: Success
    Engine-->>Main: Done
    deactivate Engine
```

---

## 🌳 プロジェクト構成ツリー図

```text
go-voicevox/
├── api/                 # VOICEVOX API 通信と WAV 結合ロジック
├── parser/              # スクリプト解析ロジック
├── speaker/             # 話者データとスタイル ID 管理
└── voicevox/            # 実行エンジンと依存関係の組み立て

```

## 🔩 外部依存ライブラリ (I/O抽象化)

本プロジェクトは、出力処理の柔軟性を高めるため、外部のI/O抽象化ライブラリに依存しています。

| ライブラリ名 | 役割 | GitHubリンク |
| :--- | :--- | :--- |
| `go-remote-io` | GCSおよびローカルファイルシステムへの統一的なI/O操作（`remoteio.Writer`）を提供します。 | [https://github.com/shouni/go-remote-io](https://github.com/shouni/go-remote-io) |

---

## 📜 ライセンス (License)

* デフォルトキャラクター: VOICEVOX:ずんだもん、VOICEVOX:四国めたん
* このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。
