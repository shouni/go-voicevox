package builder

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/shouni/go-http-kit/httpkit"
	"github.com/shouni/go-remote-io/remoteio"

	"github.com/shouni/go-voicevox/api"
	"github.com/shouni/go-voicevox/parser"
	"github.com/shouni/go-voicevox/ports"
	"github.com/shouni/go-voicevox/runner"
	"github.com/shouni/go-voicevox/speaker"
)

const (
	defaultVoicevoxAPIURL = "http://localhost:50021"
)

// noopEngineRunner は EngineRunner インターフェースを満たすダミー実装です。
type noopEngineRunner struct{}

// Run は、何もしません。
func (n *noopEngineRunner) Run(ctx context.Context, outputURI, content string, opts ...ports.RunOption) error {
	slog.Info("VOICEVOX機能は無効です。Run呼び出しはスキップされました。", "script_length", len(content))
	return nil
}

// New は、エンジンへの接続、話者データのロードを行い、依存関係を注入した Engine を組み立てて返します。
// voicevoxOutput が false の場合、実際の処理を行わない no-op エグゼキューターを返します。
func New(
	ctx context.Context,
	httpClient httpkit.Requester,
	writer remoteio.Writer,
	voicevoxOutput bool,
	opts ...ports.EngineOption,
) (ports.EngineRunner, error) {
	// 1. 機能が無効な場合は早期リターン
	if !voicevoxOutput {
		slog.Info("VOICEVOX機能は無効です。ダミーのExecutorを返します。", "action", "skip_initialization")
		return &noopEngineRunner{}, nil
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
	engine := runner.NewEngine(
		voicevoxClient,
		speakerData,
		parser.NewParser(),
		writer,
		opts...,
	)

	return engine, nil
}
