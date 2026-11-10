// Command voicevox-demo is a demo CLI that synthesizes a script with the VOICEVOX engine.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/shouni/go-voicevox/voicevox"
)

// ----------------------------------------------------------------------
// Settings (only those for the application as a whole / running VOICEVOX)
// ----------------------------------------------------------------------

const (
	// HTTP client timeout for the whole application
	appClientTimeout = 60 * time.Second
	// output file name
	outputFilename = "output/demo.wav"
)

// ----------------------------------------------------------------------
// Input script
// ----------------------------------------------------------------------

// inputScriptLines is demo data imitating the []voicevox.ScriptLine an AI (such as Gemini)
// returns as structured output. A real caller passes it to Engine.Run in this same shape.
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
	// The dictionary has no readings for Arabic numerals, so by default they stay as written
	// (8日→8ニチ, 1人→1ニン). Removing WithNumberReading below changes this line's reading.
	{Speaker: "四国めたん", Style: "ノーマル", Text: "収録は8日、参加者は1人です。"},
	{Speaker: "ずんだもん", Style: "ノーマル", Text: "同じタグが連続しても、行ごとにセグメントが分割されることを確認します。"},
	{Speaker: "ずんだもん", Style: "ノーマル", Text: "この挙動が意図通りであることを検証します。"},
}

// readingOverrides supplies, for this demo, readings the rules do not produce.
//
// Numbers and counters are read by rule with WithNumberReading (8日→ヨウカ, 1人→ヒトリ), so
// they are not listed here. What remains are words the morphological analyser has no reading
// for, such as Latin-alphabet words; without an entry, "API" and "VOICEVOX" reach synthesis
// as written. Which words get which reading is application vocabulary, so the library holds
// none and the caller passes them like this.
var readingOverrides = map[string]string{
	"API":      "エーピーアイ",
	"VOICEVOX": "ボイスボックス",
}

func main() {
	// logging setup
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// execution context
	ctx := context.Background()

	slog.Info("initializing the VOICEVOX executor")

	// The caller supplies the HTTP client (requester.go).
	internalClient := newRequester(appClientTimeout)

	// Initialization.
	// With an empty URL, New falls back to http://localhost:50021 and logs a warning.
	// No throughput options are passed: the demo's point is to run on the defaults, and
	// WithX(default) is a call that does nothing. The two reading options differ from the
	// defaults, so they are passed.
	engine, err := voicevox.New(
		ctx,
		internalClient,
		os.Getenv("VOICEVOX_API_URL"),
		// Without a speaker list, every speaker the engine provides is accepted.
		// A real application passes its own saved /speakers response to
		// speaker.NewRegistry and decides which speakers to use itself.
		nil,
		voicevox.WithNumberReading(),
		voicevox.WithReadingOverrides(readingOverrides),
	)
	if err != nil {
		slog.Error("failed to initialize the VOICEVOX engine", "error", err)
		slog.Error("check that the VOICEVOX engine is running and the API URL is correct")
		os.Exit(1)
	}

	// run the synthesis
	slog.Info("starting synthesis")

	wavBytes, err := engine.Run(ctx, inputScriptLines)
	if err != nil {
		slog.Error("synthesis failed", "error", err)
		os.Exit(1)
	}

	// Saving is not the library's job, so the caller writes a local file.
	if err := os.MkdirAll(filepath.Dir(outputFilename), 0o755); err != nil {
		slog.Error("failed to create the output directory", "error", err)
		os.Exit(1)
	}
	if err := os.WriteFile(outputFilename, wavBytes, 0o644); err != nil {
		slog.Error("failed to write the output file", "error", err)
		os.Exit(1)
	}

	absPath, _ := filepath.Abs(outputFilename)
	slog.Info(fmt.Sprintf("✅ synthesis completed. file: %s", absPath))
}
