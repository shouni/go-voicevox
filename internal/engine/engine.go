package engine

import (
	"context"
	"sync"

	"golang.org/x/time/rate"

	"github.com/shouni/go-voicevox/internal/contracts"
)

// Engine は VOICEVOX エンジンを利用した音声合成のメインコントローラーです。
type Engine struct {
	client            contracts.AudioQueryClient
	data              contracts.DataFinder
	parser            contracts.Parser
	limiter           *rate.Limiter
	writer            contracts.Writer
	config            contracts.EngineConfig
	styleIDCache      map[string]int
	styleIDCacheMutex sync.RWMutex
}

// engineSegment は内部処理用のセグメント構造体です。
type engineSegment struct {
	contracts.Segment
	StyleID int
	Err     error
}

// New は、指定された依存関係と設定を使用して新しい Engine インスタンスを作成します。
func New(client contracts.AudioQueryClient, data contracts.DataFinder, p contracts.Parser, writer contracts.Writer, opts ...contracts.Option) *Engine {
	return NewWithConfig(client, data, p, writer, contracts.NewEngineConfig(opts...))
}

// NewWithConfig は、展開済みの設定を使用して新しい Engine インスタンスを作成します。
func NewWithConfig(client contracts.AudioQueryClient, data contracts.DataFinder, p contracts.Parser, writer contracts.Writer, cfg contracts.EngineConfig) *Engine {
	if cfg.Parser != nil {
		p = cfg.Parser
	}

	engine := &Engine{
		client:       client,
		data:         data,
		parser:       p,
		writer:       writer,
		config:       cfg,
		styleIDCache: make(map[string]int),
	}
	engine.limiter = rate.NewLimiter(rate.Every(engine.config.SegmentRateLimit), 1)

	return engine
}

// Run は、音声合成プロセスを一貫して実行します。
func (e *Engine) Run(ctx context.Context, outputURI, content string, opts ...contracts.RunOption) error {
	cfg := contracts.NewRunConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	segments, preCalcErrors, err := e.prepareSegments(ctx, content, cfg)
	if err != nil {
		return err
	}

	orderedAudioDataList, runtimeErrors := e.runSynthesisBatch(ctx, segments)

	return e.finalizeOutput(ctx, orderedAudioDataList, outputURI, preCalcErrors, runtimeErrors)
}

// RunScript は、構造化された ScriptLine を直接受け取り、テキストパーサーを経由せずに
// 音声合成プロセスを一貫して実行します。
func (e *Engine) RunScript(ctx context.Context, outputURI string, lines []contracts.ScriptLine, opts ...contracts.RunOption) error {
	segments, preCalcErrors, err := e.prepareScriptSegments(ctx, lines)
	if err != nil {
		return err
	}

	orderedAudioDataList, runtimeErrors := e.runSynthesisBatch(ctx, segments)

	return e.finalizeOutput(ctx, orderedAudioDataList, outputURI, preCalcErrors, runtimeErrors)
}
