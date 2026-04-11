package voicevox

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/shouni/go-http-kit/httpkit"
	"github.com/shouni/go-remote-io/remoteio"

	"github.com/shouni/go-voicevox/api"
	"github.com/shouni/go-voicevox/parser"
	"github.com/shouni/go-voicevox/speaker"
)

// EngineExecutor はスクリプトから音声ファイルを生成するためのインターフェースです。
type EngineExecutor interface {
	Execute(ctx context.Context, script string, outputFilename string, opts ...ExecuteOption) error
}

// ----------------------------------------------------------------------
// No-op パターン
// ----------------------------------------------------------------------

// noopEngineExecutor は EngineExecutor インターフェースを満たすダミー実装です。
type noopEngineExecutor struct{}

// Execute は何もしません。
func (n *noopEngineExecutor) Execute(ctx context.Context, script string, outputFilename string, opts ...ExecuteOption) error {
	slog.Info("VOICEVOX機能は無効です。Execute呼び出しはスキップされました。", "script_length", len(script))
	return nil
}

// ----------------------------------------------------------------------
// Factory 関数
// ----------------------------------------------------------------------

// NewEngineExecutor は、VOICEVOX エンジンへの接続、話者データのロードを行い、
// 依存関係を注入した EngineExecutor を組み立てて返します。
//
// voicevoxOutput が false の場合、実際の処理を行わない no-op エグゼキューターを返します。
func NewEngineExecutor(
	ctx context.Context,
	httpClient httpkit.Requester,
	writer remoteio.Writer,
	voicevoxOutput bool,
) (EngineExecutor, error) {
	// 1. 機能が無効な場合は早期リターン
	if !voicevoxOutput {
		slog.Info("VOICEVOX機能は無効です。ダミーのExecutorを返します。", "action", "skip_initialization")
		return &noopEngineExecutor{}, nil
	}

	// 2. API URL の設定
	voicevoxAPIURL := os.Getenv("VOICEVOX_API_URL")
	if voicevoxAPIURL == "" {
		voicevoxAPIURL = defaultVoicevoxAPIURL // 定義済みのデフォルト値
		slog.Warn("VOICEVOX_API_URL 環境変数が設定されていません。デフォルトを使用します。", "url", voicevoxAPIURL)
	}

	// 3. VOICEVOX クライアントの初期化
	voicevoxClient := api.NewClient(httpClient, voicevoxAPIURL)

	// 4. 話者データのロード (エンジン初期化の必須依存)
	slog.Info("VOICEVOX話者スタイルデータをロード中...")
	speakerData, err := speaker.LoadSpeakers(ctx, voicevoxClient)
	if err != nil {
		return nil, fmt.Errorf("VOICEVOXデータのロードに失敗しました: %w", err)
	}
	slog.Info("VOICEVOX話者スタイルデータのロード完了。", "styles_count", len(speakerData.StyleIDMap))

	// 5. Engine の組み立て
	// 以前定義した Functional Options を使用して設定を適用します。
	// ここでは環境変数やデフォルト値に基づいて Option を組み立てることも可能です。
	engine := NewEngine(
		voicevoxClient,
		speakerData,
		parser.NewParser(),
		writer,
		WithMaxParallelSegments(DefaultMaxParallelSegments),
		WithSegmentTimeout(DefaultSegmentTimeout),
		WithSegmentRateLimit(DefaultSegmentRateLimit),
	)

	slog.Info("VOICEVOX Executorの初期化が完了しました。",
		"max_parallel", DefaultMaxParallelSegments,
		"segment_timeout", DefaultSegmentTimeout)

	return engine, nil
}
