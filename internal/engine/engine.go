// Package engine は、スクリプト行の分割・並列合成・結合を行う中核実装です。
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
	converter         contracts.TextConverter
	limiter           *rate.Limiter
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
func New(client contracts.AudioQueryClient, data contracts.DataFinder, converter contracts.TextConverter, opts ...contracts.Option) *Engine {
	return NewWithConfig(client, data, converter, contracts.NewEngineConfig(opts...))
}

// NewWithConfig は、展開済みの設定を使用して新しい Engine インスタンスを作成します。
func NewWithConfig(client contracts.AudioQueryClient, data contracts.DataFinder, converter contracts.TextConverter, cfg contracts.EngineConfig) *Engine {
	engine := &Engine{
		client:       client,
		data:         data,
		converter:    converter,
		config:       cfg,
		styleIDCache: make(map[string]int),
	}
	engine.limiter = rate.NewLimiter(rate.Every(engine.config.SegmentRateLimit), 1)

	return engine
}

// Run は、構造化された ScriptLine を受け取り、結合済みのWAVバイト列を返します。
func (e *Engine) Run(ctx context.Context, lines []contracts.ScriptLine) ([]byte, error) {
	segments, preCalcErrors, err := e.prepareSegments(ctx, lines)
	if err != nil {
		return nil, err
	}

	orderedAudioDataList, runtimeErrors := e.runSynthesisBatch(ctx, segments)

	return combineOutput(orderedAudioDataList, preCalcErrors, runtimeErrors)
}
