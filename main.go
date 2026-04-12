package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shouni/go-http-kit/httpkit"
	"github.com/shouni/go-remote-io/remoteio/gcs"
	"github.com/shouni/go-voicevox/builder"
	"github.com/shouni/go-voicevox/runner"
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

const inputScript = `
[ずんだもん][ノーマル] [呼びかけ] こんにちは、ずんだもんです。
[めたん][あまあま] テスト用のスクリプトを開始します。

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

	// GCS Client Factoryの初期化、GCSクライアントのリソース管理はここで開始
	gcsFactory, err := gcs.New(ctx)
	if err != nil {
		slog.Error("GCS Client Factoryの初期化に失敗しました。", "error", err)
		os.Exit(1)
	}
	// GCSクライアントのリソースを解放
	defer func() {
		if closeErr := gcsFactory.Close(); closeErr != nil {
			slog.Error("GCS Client Factoryのクローズに失敗しました。", "error", closeErr)
		}
	}()

	writer, err := gcsFactory.Writer()
	if err != nil {
		slog.Error("Writerの初期化に失敗しました。", "error", err)
		return
	}

	slog.Info("VOICEVOX Executorの初期化を開始します...")

	// 社内APIへのアクセスなど、安全性が保証されている場合は検証をスキップ
	internalClient := httpkit.New(
		appClientTimeout,
		httpkit.WithMaxRetries(1),
		httpkit.WithSkipNetworkValidation(true),
	)

	// 2. Executorの初期化
	voicevoxExecutor, err := builder.New(
		ctx,
		internalClient,
		writer,
		true,
		runner.WithMaxParallelSegments(runner.DefaultMaxParallelSegments),
		runner.WithSegmentTimeout(runner.DefaultSegmentTimeout),
		runner.WithSegmentRateLimit(runner.DefaultSegmentRateLimit),
	)

	// voicevoxOutput が true なので、voicevoxExecutor は nil でないはず
	if err != nil {
		slog.Error("VOICEVOX Executorの初期化に失敗しました。", "error", err)
		slog.Error("VOICEVOXエンジンが起動しているか、またはAPI URLが正しいか確認してください。")
		os.Exit(1)
	}

	// 3. 音声合成の実行
	slog.Info("音声合成処理を開始します。", "output", outputFilename)

	err = voicevoxExecutor.Run(ctx, inputScript, outputFilename)
	if err != nil {
		slog.Error("音声合成の実行に失敗しました。", "error", err)
		os.Exit(1)
	}

	// GCSの場合、ファイルパスの表示は変更
	if strings.HasPrefix(outputFilename, "gs://") {
		slog.Info(fmt.Sprintf("✅ 音声合成が正常に完了しました。GCSオブジェクト: %s", outputFilename))
	} else {
		absPath, _ := filepath.Abs(outputFilename)
		slog.Info(fmt.Sprintf("✅ 音声合成が正常に完了しました。ファイル: %s", absPath))
	}
}
