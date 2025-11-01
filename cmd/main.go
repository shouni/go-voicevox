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

	// 出力ファイル名
	outputFilename = "ai_partner_intro.wav"
)

// ----------------------------------------------------------------------
// 入力スクリプト
// ----------------------------------------------------------------------

const inputScript = `
[ずんだもん][ノーマル] [呼びかけ] 開発者の皆さん、日々のコードレビューでこんな悩みはありませんか？
[めたん][ノーマル] [解説] 時間がかかる、見落としがある、フィードバックの質にばらつきがあるなど、コードレビューはチームの生産性を左右する重要な課題ですよね。
[ずんだもん][ノーマル] [解説] 今日は、そんな課題を解決し、開発チームの生産性を飛躍的に高めるAIパートナー、「Git Gemini Reviewer Go」をご紹介するのだ。
`

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
	speakerData, err := voicevox.LoadSpeakers(loadCtx, client, voicevoxAPIURL)
	if err != nil {
		slog.Error("話者データのロードに失敗しました。VOICEVOXエンジンが起動しているか確認してください。", "error", err)
		os.Exit(1)
	}

	// 3. パーサーの初期化と Engine への依存性注入
	parser := voicevox.NewTextParser() // script_parser.go で実装されたパーサー
	engine := voicevox.NewEngine(client, speakerData, parser)

	slog.Info("VOICEVOX エンジンの初期化が完了しました。")

	// 4. 音声合成の実行
	slog.Info("音声合成処理を開始します。", "output", outputFilename)

	// engine.Execute の処理は SegmentTimeout を内部で利用するため、ここでは長めのコンテキストを渡す
	err = engine.Execute(ctx, inputScript, outputFilename, voicevox.VvTagNormal)
	if err != nil {
		slog.Error("音声合成の実行に失敗しました。", "error", err)
		os.Exit(1)
	}

	absPath, _ := filepath.Abs(outputFilename)
	slog.Info(fmt.Sprintf("✅ 音声合成が正常に完了しました。ファイル: %s", absPath))
}
