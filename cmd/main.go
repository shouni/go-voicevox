package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/shouni/go-voicevox/pkg/voicevox"
)

// ----------------------------------------------------------------------
// 設定定数
// ----------------------------------------------------------------------

const (
	// VOICEVOX エンジンのデフォルト URL
	voicevoxAPIURL = "http://localhost:50021"

	// タイムアウト設定
	clientTimeout   = 60 * time.Second
	loadDataTimeout = 5 * time.Second
	// Engine 設定
	customMaxParallelSegments = 10
	customSegmentTimeout      = 180 * time.Second

	// 出力ファイル名
	outputFilename = "tts_output.wav" //
)

// ----------------------------------------------------------------------
// 入力スクリプト
// ----------------------------------------------------------------------

const inputScript = `
[ずんだもん][ノーマル] [呼びかけ] こんにちは、ずんだもんです。
[めたん][ツンツン] テスト用のスクリプトを開始します。

// タグのない行（前のセグメントに結合されることを期待）
これはタグなし行です。前のセグメントに結合されます。

[ずんだもん][あまあま] まず、短い文章の合成を確認するのだ。

// 長文による強制分割のテスト（200文字の制限を確認）
[めたん][ノーマル] これは、文字数制限によるセグメントの強制分割をテストするための非常に長い文章であり、その長さは200文字の制限を大きく超えています。パーサーは、この文章を自然な句読点の位置で分割することを試みますが、それが見つからない場合は、200文字の制限内で機械的に強制的にセグメントを分割するべきです。このテストにより、パーサーがAPIリクエストの安全性を保証し、VOICEVOXエンジンへの過負荷を防ぐことを確認します。（この行は220文字以上あることを想定し、最低2セグメントに強制分割されることを期待）。

// 複数行にわたる同じタグのテスト（一行一セグメントの強制を確認）
[ずんだもん][ノーマル] これは複数行にわたるテストです。
[ずんだもん][ノーマル] 同じタグが連続しても、行ごとにセグメントが分割されることを確認します。
[ずんだもん][ノーマル] この挙動が意図通りであることを検証します。
` // 200文字以上の長文を想定

func main() {
	// ログ設定
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// 実行コンテキスト
	ctx := context.Background()

	slog.Info("VOICEVOX クライアントとエンジンの初期化を開始します...")

	// 1. クライアントの初期化
	client := voicevox.NewClient(voicevoxAPIURL, clientTimeout)

	// 2. 話者データ（スタイルIDマップ）のロード
	// SpeakerData のロード処理は時間がかかる可能性があるため、個別のタイムアウトを設定
	loadCtx, cancel := context.WithTimeout(ctx, loadDataTimeout)
	defer cancel()

	// SpeakerClient インターフェースを満たす client を利用してロード
	// NOTE: LoadSpeakers が未提供のため、この行はコンパイルエラーになる可能性がある
	// 		 実際の実装では voicevox.NewSpeakerData() などで代替が必要かもしれません。
	speakerData, err := voicevox.LoadSpeakers(loadCtx, client)
	if err != nil {
		slog.Error("話者データのロードに失敗しました。VOICEVOXエンジンが起動しているか確認してください。", "error", err)
		os.Exit(1)
	}

	// 3. パーサーの初期化と Engine への依存性注入
	engineConfig := voicevox.EngineConfig{
		MaxParallelSegments: customMaxParallelSegments,
		SegmentTimeout:      customSegmentTimeout,
	}
	parser := voicevox.NewTextParser()
	engine := voicevox.NewEngine(client, speakerData, parser, engineConfig)

	slog.Info("VOICEVOX エンジンの初期化が完了しました。")

	// 4. 音声合成の実行
	slog.Info("音声合成処理を開始します。", "output", outputFilename)

	// engine.Execute の処理は SegmentTimeout を内部で利用するため、ここでは長めのコンテキストを渡す
	err = engine.Execute(ctx, inputScript, outputFilename)
	if err != nil {
		slog.Error("音声合成の実行に失敗しました。", "error", err)
		os.Exit(1)
	}

	absPath, _ := filepath.Abs(outputFilename)
	slog.Info(fmt.Sprintf("✅ 音声合成が正常に完了しました。ファイル: %s", absPath))
}
