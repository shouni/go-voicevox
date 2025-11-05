package voicevox

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// defaultVoicevoxAPIURL は環境変数が未設定の場合のVOICEVOXエンジンのデフォルトURLです。
// 通常、ローカルで実行しているVOICEVOXエンジンのデフォルトポートです。
const defaultVoicevoxAPIURL = "http://127.0.0.1:50021"

// NewEngineExecutor は、VOICEVOXエンジンへの接続、話者データのロードを行い、
// EngineExecutorインターフェースを実装した具象型を組み立てて返します。
func NewEngineExecutor(
	ctx context.Context,
	httpTimeout time.Duration,
	voicevoxOutput bool,
) (EngineExecutor, error) {

	// VOICEVOX機能を使用しない場合はnilを返す
	if !voicevoxOutput {
		slog.Info("VOICEVOX機能は無効です。Executorを構築しません。")
		return nil, nil
	}

	// 1-1. API URLの設定
	voicevoxAPIURL := os.Getenv("VOICEVOX_API_URL")
	if voicevoxAPIURL == "" {
		voicevoxAPIURL = defaultVoicevoxAPIURL
		slog.Warn("VOICEVOX_API_URL 環境変数が設定されていません。", "default_url", voicevoxAPIURL)
	}

	// 1-2. クライアントの初期化 (Client は AudioQueryClient と SpeakerClient を満たすと仮定)
	voicevoxClient := NewClient(voicevoxAPIURL, httpTimeout)

	slog.Info("VOICEVOX話者スタイルデータをロード中...", "api_url", voicevoxAPIURL)

	// 2. SpeakerDataのロード (Engine初期化の必須依存)
	// ロード処理のタイムアウトをhttpTimeoutに設定
	loadCtx, cancel := context.WithTimeout(ctx, httpTimeout)
	defer cancel()

	// LoadSpeakers は speaker_loader.go で定義されていると仮定
	speakerData, loadErr := LoadSpeakers(loadCtx, voicevoxClient)
	if loadErr != nil {
		return nil, fmt.Errorf("VOICEVOXエンジンへの接続または話者データのロードに失敗しました: %w", loadErr)
	}
	slog.Info("VOICEVOX話者スタイルデータのロード完了。", "styles_count", len(speakerData.StyleIDMap))

	// 3. EngineConfigの設定 (const.goなどで定義されていると仮定)
	engineConfig := EngineConfig{
		MaxParallelSegments: DefaultMaxParallelSegments, // const.go
		SegmentTimeout:      DefaultSegmentTimeout,      // const.go
	}

	// 4. Engineの組み立てとExecutorとしての返却
	// NewTextParser は script_parser.go で定義されていると仮定
	parser := NewTextParser()

	// NewEngine は engine.go で定義されており、EngineExecutor を満たす具象型を返す
	voicevoxExecutor := NewEngine(voicevoxClient, speakerData, parser, engineConfig)

	slog.Info("VOICEVOX Executorの初期化が完了しました。",
		"max_parallel", engineConfig.MaxParallelSegments,
		"segment_timeout", engineConfig.SegmentTimeout.String())

	return voicevoxExecutor, nil
}
