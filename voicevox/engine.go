package voicevox

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/shouni/go-http-kit/httpkit"
	"github.com/shouni/go-remote-io/remoteio"

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

type remoteIOWriter struct {
	writer remoteio.Writer
}

func (w remoteIOWriter) Write(ctx context.Context, path string, contentReader io.Reader, opts ...contracts.WriteOption) error {
	cfg := contracts.NewWriteConfig(opts...)
	remoteOpts := make([]remoteio.WriteOption, 0, 3)
	if cfg.ContentType != "" {
		remoteOpts = append(remoteOpts, remoteio.WithContentType(cfg.ContentType))
	}
	if cfg.Inline {
		remoteOpts = append(remoteOpts, remoteio.WithInline())
	}
	if cfg.CacheControl != "" {
		remoteOpts = append(remoteOpts, remoteio.WithCacheControl(cfg.CacheControl))
	}

	return w.writer.Write(ctx, path, contentReader, remoteOpts...)
}

// New は、依存関係を組み立てて Engine を返します。
func New(
	ctx context.Context,
	httpClient httpkit.Requester,
	writer remoteio.Writer,
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

	voicevoxClient := api.New(httpClient, voicevoxAPIURL)

	slog.Info("VOICEVOX話者スタイルデータをロード中...")
	speakerData, err := speaker.LoadSpeakers(ctx, voicevoxClient)
	if err != nil {
		return nil, fmt.Errorf("VOICEVOXデータのロードに失敗しました: %w", err)
	}
	slog.Info("VOICEVOX話者スタイルデータのロード完了。", "styles_count", len(speakerData.StyleIDMap))

	engine := internalengine.New(
		voicevoxClient,
		speakerData,
		parser.NewParser(),
		remoteIOWriter{writer: writer},
		opts...,
	)

	return engine, nil
}
