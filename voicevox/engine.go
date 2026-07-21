package voicevox

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shouni/go-http-kit/httpkit"

	"github.com/shouni/go-voicevox/api"
	"github.com/shouni/go-voicevox/internal/contracts"
	internalengine "github.com/shouni/go-voicevox/internal/engine"
	"github.com/shouni/go-voicevox/parser"
	"github.com/shouni/go-voicevox/speaker"
)

const defaultVoicevoxAPIURL = "http://localhost:50021"

// noopEngine は Engine インターフェースを満たすダミー実装です。
type noopEngine struct{}

// Run は、何もしません。
func (n *noopEngine) Run(ctx context.Context, outputURI, content string, opts ...RunOption) error {
	slog.Info("VOICEVOX機能は無効です。Run呼び出しはスキップされました。", "script_length", len(content))
	return nil
}

// RunScript は、何もしません。
func (n *noopEngine) RunScript(ctx context.Context, outputURI string, lines []ScriptLine, opts ...RunOption) error {
	slog.Info("VOICEVOX機能は無効です。RunScript呼び出しはスキップされました。", "lines", len(lines))
	return nil
}

// New は、依存関係を組み立てて Engine を返します。
// writer には出力先の Writer を指定します。ローカルファイルシステムへの書き込みには
// NewLocalWriter() を利用できます。
func New(
	ctx context.Context,
	httpClient httpkit.Requester,
	writer Writer,
	voicevoxAPIURL string,
	voicevoxOutput bool,
	opts ...Option,
) (Engine, error) {
	if !voicevoxOutput {
		slog.Info("VOICEVOX機能は無効です。ダミーのEngineを返します。", "action", "skip_initialization")
		return &noopEngine{}, nil
	}

	if voicevoxAPIURL == "" {
		voicevoxAPIURL = defaultVoicevoxAPIURL
		slog.Warn("VOICEVOX_API_URL が指定されていません。デフォルトを使用します。", "url", voicevoxAPIURL)
	}

	engineConfig := contracts.NewEngineConfig(opts...)

	scriptParser := engineConfig.Parser
	if scriptParser == nil && engineConfig.PhoneticPreprocessing {
		phoneticParser, err := parser.NewPhoneticParser()
		if err != nil {
			return nil, fmt.Errorf("音声合成テキスト前処理の初期化に失敗しました: %w", err)
		}
		scriptParser = phoneticParser
	}
	if scriptParser == nil {
		scriptParser = parser.NewParser()
	}

	voicevoxClient := api.New(httpClient, voicevoxAPIURL)

	slog.Info("VOICEVOX話者スタイルデータをロード中...")
	speakerData, err := speaker.LoadSpeakers(ctx, voicevoxClient)
	if err != nil {
		return nil, fmt.Errorf("VOICEVOXデータのロードに失敗しました: %w", err)
	}
	slog.Info("VOICEVOX話者スタイルデータのロード完了。", "styles_count", len(speakerData.StyleIDMap))

	engine := internalengine.NewWithConfig(
		voicevoxClient,
		speakerData,
		scriptParser,
		writer,
		engineConfig,
	)

	return engine, nil
}
