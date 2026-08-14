// Package engine は、スクリプト行の分割・並列合成・結合を行う中核実装です。
package engine

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
)

// 以下の 3 つは Engine が要求する依存です。**使う側であるここで定義します。**
// 実装は api.Client と speaker.Data ですが、engine はどちらも import しません。
// Go の構造的型付けにより、満たす値を渡すだけで繋がります。

// AudioQueryClient は、音声クエリの作成と合成を行うAPIクライアントです。
type AudioQueryClient interface {
	RunAudioQuery(ctx context.Context, text string, styleID int) ([]byte, error)
	RunSynthesis(ctx context.Context, queryBody []byte, styleID int) ([]byte, error)
}

// DataFinder は、話者・スタイルのタグからスタイルIDを解決します。
type DataFinder interface {
	GetStyleID(combinedTag string) (int, bool)
	GetDefaultTag(speakerToolTag string) (string, bool)
}

// TextConverter は、合成前のセグメントテキストを VOICEVOX の誤読を避けるための
// 読み(カタカナ)へ変換します。
type TextConverter interface {
	ConvertToReading(text string) string
}

// Engine は VOICEVOX エンジンを利用した音声合成のメインコントローラーです。
type Engine struct {
	client            AudioQueryClient
	data              DataFinder
	converter         TextConverter
	limiter           *rate.Limiter
	config            Config
	styleIDCache      map[string]int
	styleIDCacheMutex sync.RWMutex
}

// engineSegment は内部処理用のセグメント構造体です。
type engineSegment struct {
	Segment
	StyleID int
	Err     error
}

// New は、指定された依存関係と設定を使用して新しい Engine インスタンスを作成します。
func New(client AudioQueryClient, data DataFinder, converter TextConverter, opts ...Option) *Engine {
	return NewWithConfig(client, data, converter, NewConfig(opts...))
}

// NewWithConfig は、展開済みの設定を使用して新しい Engine インスタンスを作成します。
func NewWithConfig(client AudioQueryClient, data DataFinder, converter TextConverter, cfg Config) *Engine {
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
func (e *Engine) Run(ctx context.Context, lines []ScriptLine) ([]byte, error) {
	segments, preCalcErrors, err := e.prepareSegments(ctx, lines)
	if err != nil {
		return nil, err
	}

	orderedAudioDataList, runtimeErrors := e.runSynthesisBatch(ctx, segments)

	return combineOutput(orderedAudioDataList, preCalcErrors, runtimeErrors)
}
