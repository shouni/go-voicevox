package engine

import (
	"context"
	"sync"

	"github.com/shouni/go-remote-io/remoteio"
	"golang.org/x/time/rate"

	"github.com/shouni/go-voicevox/internal/contracts"
)

// Engine は VOICEVOX エンジンを利用した音声合成のメインコントローラーです。
type Engine struct {
	client            contracts.AudioQueryClient
	data              contracts.DataFinder
	parser            contracts.Parser
	limiter           *rate.Limiter
	writer            remoteio.Writer
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
func New(client contracts.AudioQueryClient, data contracts.DataFinder, p contracts.Parser, writer remoteio.Writer, opts ...contracts.Option) *Engine {
	allOpts := []contracts.Option{
		contracts.WithMaxParallelSegments(contracts.DefaultMaxParallelSegments),
		contracts.WithSegmentTimeout(contracts.DefaultSegmentTimeout),
		contracts.WithSegmentRateLimit(contracts.DefaultSegmentRateLimit),
	}
	allOpts = append(allOpts, opts...)

	engine := &Engine{
		client:       client,
		data:         data,
		parser:       p,
		writer:       writer,
		styleIDCache: make(map[string]int),
	}

	for _, opt := range allOpts {
		opt(&engine.config)
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
