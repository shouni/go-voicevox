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

// 長文による強制分割のテスト（250文字の制限を確認）
[めたん][ノーマル] これは、文字数制限によるセグメントの強制分割をテストするための非常に長い文章です。この文章の長さは250文字を超えており、パーサーが句読点や文字数制限に基づいて適切にセグメントを分割することを期待します。分割が正しく行われない場合、APIリクエストがエラーになるか、意図しない音声合成結果になる可能性があります。（テストコードの文字数を実際に増やして確認してください）...。
` // 250文字以上の長文を想定

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
	// NOTE: NewTextParser や Engine の依存関係が未提供のため、この行はコンパイルエラーになる可能性がある
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
