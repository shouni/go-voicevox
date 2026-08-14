// Command go-voicevox は、VOICEVOX エンジンで台本を音声合成する CLI です。
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/shouni/go-http-kit/httpkit"
	"github.com/shouni/go-voicevox/voicevox"
)

// ----------------------------------------------------------------------
// 設定定数 (アプリケーション全体/VOICEVOX実行に関わるもののみ残す)
// ----------------------------------------------------------------------

const (
	// アプリケーション全体のHTTPクライアントタイムアウト
	appClientTimeout = 60 * time.Second
	// 出力ファイル名
	outputFilename = "output/demo.wav"
	// VOICEVOX APIのデフォルトURL
	defaultVoicevoxAPIURL = "http://localhost:50021"
)

// ----------------------------------------------------------------------
// 入力スクリプト
// ----------------------------------------------------------------------

// inputScriptLines は、AI (Gemini など) が構造化出力として返す
// []voicevox.ScriptLine を模したデモ用データです。
// 実際の呼び出し側は Engine.Run にこの形のまま渡します。
var inputScriptLines = []voicevox.ScriptLine{
	{Speaker: "ずんだもん", Style: "ノーマル", Text: "こんにちは、ずんだもんです。"},
	{Speaker: "四国めたん", Style: "あまあま", Text: "テスト用のスクリプトを開始します。"},
	{Speaker: "ずんだもん", Style: "あまあま", Text: "まず、短い文章の合成を確認するのだ。"},
	{
		Speaker: "四国めたん", Style: "ノーマル",
		Text: "これは、文字数制限によるセグメントの強制分割をテストするための非常に長い文章であり、" +
			"その長さは200文字の制限を大きく超えています。パーサーは、この文章を自然な句読点の位置で分割することを" +
			"試みますが、それが見つからない場合は、200文字の制限内で機械的に強制的にセグメントを分割するべきです。" +
			"このテストにより、パーサーがAPIリクエストの安全性を保証し、VOICEVOXエンジンへの過負荷を防ぐことを確認します。" +
			"（この行は220文字以上あることを想定し、最低2セグメントに強制分割されることを期待）。",
	},
	{Speaker: "ずんだもん", Style: "ノーマル", Text: "これは複数行にわたるテストです。"},
	{Speaker: "ずんだもん", Style: "ノーマル", Text: "同じタグが連続しても、行ごとにセグメントが分割されることを確認します。"},
	{Speaker: "ずんだもん", Style: "ノーマル", Text: "この挙動が意図通りであることを検証します。"},
}

func main() {
	// ログ設定
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// 実行コンテキスト
	ctx := context.Background()

	slog.Info("VOICEVOX Executorの初期化を開始します...")

	// 社内APIへのアクセスなど、安全性が保証されている場合は検証をスキップ
	internalClient := httpkit.New(
		appClientTimeout,
		httpkit.WithMaxRetries(1),
		httpkit.WithSkipNetworkValidation(true),
	)

	voicevoxAPIURL := os.Getenv("VOICEVOX_API_URL")
	if voicevoxAPIURL == "" {
		voicevoxAPIURL = defaultVoicevoxAPIURL
		slog.Warn("VOICEVOX_API_URL 環境変数が設定されていないため、デフォルトを使用します。", "url", voicevoxAPIURL)
	}

	// 初期化
	engine, err := voicevox.New(
		ctx,
		internalClient,
		voicevoxAPIURL,
		// 話者一覧を渡さない場合、エンジンが提供する話者をすべて受け入れます。
		// 実際のアプリケーションは自分で保存した /speakers 応答を
		// speaker.NewRegistry へ渡し、使う話者を自分で決めます。
		nil,
		voicevox.WithMaxParallelSegments(voicevox.DefaultMaxParallelSegments),
		voicevox.WithSegmentTimeout(voicevox.DefaultSegmentTimeout),
		voicevox.WithSegmentRateLimit(voicevox.DefaultSegmentRateLimit),
	)
	if err != nil {
		slog.Error("VOICEVOXエンジンの初期化に失敗しました。", "error", err)
		slog.Error("VOICEVOXエンジンが起動しているか、またはAPI URLが正しいか確認してください。")
		os.Exit(1)
	}

	// 音声合成の実行
	slog.Info("音声合成処理を開始します。")

	wavBytes, err := engine.Run(ctx, inputScriptLines)
	if err != nil {
		slog.Error("音声合成の実行に失敗しました。", "error", err)
		os.Exit(1)
	}

	// 保存はライブラリの責務ではないため、呼び出し側でローカルファイルに書き込む。
	if err := os.MkdirAll(filepath.Dir(outputFilename), 0o755); err != nil {
		slog.Error("出力ディレクトリの作成に失敗しました。", "error", err)
		os.Exit(1)
	}
	if err := os.WriteFile(outputFilename, wavBytes, 0o644); err != nil {
		slog.Error("出力ファイルの書き込みに失敗しました。", "error", err)
		os.Exit(1)
	}

	absPath, _ := filepath.Abs(outputFilename)
	slog.Info(fmt.Sprintf("✅ 音声合成が正常に完了しました。ファイル: %s", absPath))
}
