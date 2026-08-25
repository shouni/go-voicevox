// Package engine は、スクリプト行の分割・並列合成・結合を行う中核実装です。
package engine

import (
	"context"

	"golang.org/x/time/rate"
)

// 以下の 3 つは Engine が要求する依存です。**使う側であるここで定義します。**
// 実装は api.Client と speaker.Styles ですが、engine はどちらも import しません。
// Go の構造的型付けにより、満たす値を渡すだけで繋がります。

// AudioQueryClient は、音声クエリの作成と合成を行うAPIクライアントです。
type AudioQueryClient interface {
	RunAudioQuery(ctx context.Context, text string, styleID int) ([]byte, error)
	RunSynthesis(ctx context.Context, queryBody []byte, styleID int) ([]byte, error)
}

// StyleFinder は、話者・スタイルのタグからスタイルIDを解決します。
type StyleFinder interface {
	GetStyleID(combinedTag string) (int, bool)
	GetDefaultTag(speakerToolTag string) (string, bool)
}

// TextConverter は、合成前のセグメントテキストを VOICEVOX の誤読を避けるための
// 読み(カタカナ)へ変換します。
type TextConverter interface {
	ConvertToReading(text string) string
}

// Engine は VOICEVOX エンジンを利用した音声合成のメインコントローラーです。
//
// **構築後は書き換わりません。** そのため 1 つの Engine を複数のゴルーチンから同時に
// Run しても安全です（rate.Limiter は自身で同期しています）。以前はスタイル ID の
// キャッシュを Engine が map と RWMutex で抱えており、同時実行の可否が錠前の正しさに
// ぶら下がっていました。キャッシュは 1回の Run に閉じた styleResolver へ移してあります。
type Engine struct {
	client    AudioQueryClient
	styles    StyleFinder
	converter TextConverter
	limiter   *rate.Limiter
	config    Config
}

// New は、指定された依存関係とオプションから Engine を作ります。
//
// **組み立ての口はこれだけです。** 展開済みの Config を取る NewWithConfig も
// 並べていましたが、本番は後者・テストは前者しか通らず、同じものへ 2 つ扉が
// 開いているだけでした。設定を先に組みたい場合は NewConfig の結果を
// オプションとして渡せます。
func New(client AudioQueryClient, styles StyleFinder, converter TextConverter, opts ...Option) *Engine {
	cfg := NewConfig(opts...)
	engine := &Engine{
		client:    client,
		styles:    styles,
		converter: converter,
		config:    cfg,
	}
	engine.limiter = rate.NewLimiter(rate.Every(cfg.SegmentRateLimit), 1)

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
