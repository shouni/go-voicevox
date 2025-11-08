package voicevox

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/shouni/go-voicevox/pkg/voicevox/api"
	"github.com/shouni/go-voicevox/pkg/voicevox/parser"
	"github.com/shouni/go-voicevox/pkg/voicevox/speaker"
)

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

// NewEngineExecutor は、VOICEVOXエンジンへの接続、話者データのロードを行い、
// EngineExecutorインターフェースを実装した具象型を組み立てて返します。
func NewEngineExecutor(
	ctx context.Context,
	httpTimeout time.Duration,
	voicevoxOutput bool,
) (EngineExecutor, error) {
	// VOICEVOX機能を使用しない場合はダミーのExecutorを返す (No-opパターン)
	if !voicevoxOutput {
		slog.Info("VOICEVOX機能は無効です。ダミーのExecutorを返します。", "action", "skip_initialization")
		return &noopEngineExecutor{}, nil
	}

	// 1-1. API URLの設定
	voicevoxAPIURL := os.Getenv("VOICEVOX_API_URL")
	if voicevoxAPIURL == "" {
		voicevoxAPIURL = defaultVoicevoxAPIURL
		slog.Warn("VOICEVOX_API_URL 環境変数が設定されていません。", "default_url", voicevoxAPIURL)
	}

	// 1-2. クライアントの初期化 (api.NewClient は api.Client を返す)
	voicevoxClient := api.NewClient(voicevoxAPIURL, httpTimeout)

	slog.Info("VOICEVOX話者スタイルデータをロード中...")

	// 2. SpeakerDataのロード (Engine初期化の必須依存)
	speakerData, loadErr := speaker.LoadSpeakers(ctx, voicevoxClient)
	if loadErr != nil {
		return nil, fmt.Errorf("VOICEVOXエンジンへの接続または話者データのロードに失敗しました: %w", loadErr)
	}
	slog.Info("VOICEVOX話者スタイルデータのロード完了。", "styles_count", len(speakerData.StyleIDMap))

	// 3. EngineConfigの設定
	engineConfig := EngineConfig{
		MaxParallelSegments: DefaultMaxParallelSegments,
		SegmentTimeout:      DefaultSegmentTimeout,
	}

	// 4. Engineの組み立てとExecutorとしての返却
	textParser := parser.NewParser()

	// NewEngine を呼び出す (engine.go で定義)
	voicevoxExecutor := NewEngine(voicevoxClient, speakerData, textParser, engineConfig)
	slog.Info("VOICEVOX Executorの初期化が完了しました。",
		"max_parallel", engineConfig.MaxParallelSegments,
		"segment_timeout", engineConfig.SegmentTimeout.String())

	return voicevoxExecutor, nil
}
