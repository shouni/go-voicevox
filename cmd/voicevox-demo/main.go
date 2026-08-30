// Command voicevox-demo は、VOICEVOX エンジンで台本を音声合成するデモ CLI です。
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
	// 算用数字は辞書が読みを持たないため、既定では字面のまま残ります（8日→8ニチ、1人→1ニン）。
	// 下の WithNumberReading を外すと、この行の読みが変わります。
	{Speaker: "四国めたん", Style: "ノーマル", Text: "収録は8日、参加者は1人です。"},
	{Speaker: "ずんだもん", Style: "ノーマル", Text: "同じタグが連続しても、行ごとにセグメントが分割されることを確認します。"},
	{Speaker: "ずんだもん", Style: "ノーマル", Text: "この挙動が意図通りであることを検証します。"},
}

// readingOverrides は、規則では当たらない読みをこのデモで指定します。
//
// 数と助数詞は WithNumberReading が規則で読むので（8日→ヨウカ、1人→ヒトリ）、ここには
// 置きません。残るのはアルファベットのように形態素解析器が読みを持たない語で、指定しないと
// "API" や "VOICEVOX" が字面のまま合成へ渡ります。どの語をどう読ませるかはアプリケーションの
// 語彙なので、ライブラリは中身を持たず、呼び出し側がこうして渡します。
var readingOverrides = map[string]string{
	"API":      "エーピーアイ",
	"VOICEVOX": "ボイスボックス",
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

	// 初期化。
	// URL が空なら New が http://localhost:50021 へ落とし、警告も出します。
	// 流量のオプションは渡しません。既定のまま動かすのがデモの目的で、
	// WithX(既定値) は何もしない呼び出しです。読みの 2 つは既定と違うので渡します。
	engine, err := voicevox.New(
		ctx,
		internalClient,
		os.Getenv("VOICEVOX_API_URL"),
		// 話者一覧を渡さない場合、エンジンが提供する話者をすべて受け入れます。
		// 実際のアプリケーションは自分で保存した /speakers 応答を
		// speaker.NewRegistry へ渡し、使う話者を自分で決めます。
		nil,
		voicevox.WithNumberReading(),
		voicevox.WithReadingOverrides(readingOverrides),
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
