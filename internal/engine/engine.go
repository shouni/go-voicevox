// Package engine is the core implementation: it splits script lines, synthesizes them in
// parallel and combines the result.
package engine

import (
	"context"

	"golang.org/x/time/rate"
)

// The three interfaces below are the dependencies Engine requires, declared here by their
// consumer. The implementations are api.Client and speaker.Styles, but engine imports
// neither; Go's structural typing connects them as soon as a satisfying value is passed.

// AudioQueryClient is the API client that creates audio queries and synthesizes them.
type AudioQueryClient interface {
	RunAudioQuery(ctx context.Context, text string, styleID int) ([]byte, error)
	RunSynthesis(ctx context.Context, queryBody []byte, styleID int) ([]byte, error)
}

// StyleFinder resolves a speaker/style tag to a style ID.
type StyleFinder interface {
	GetStyleID(combinedTag string) (int, bool)
	GetDefaultTag(speakerToolTag string) (string, bool)
}

// TextConverter converts a segment's text, before synthesis, into a katakana reading so that
// VOICEVOX does not misread it.
type TextConverter interface {
	ConvertToReading(text string) string
}

// Engine is the main controller for speech synthesis with the VOICEVOX engine.
//
// It does not change after construction, so one Engine can be Run from several goroutines at
// once (rate.Limiter synchronizes itself). Engine used to hold the style ID cache in a map
// behind an RWMutex, so whether concurrent use was safe hung on that lock being right. The
// cache has moved to styleResolver, which lives for a single Run.
type Engine struct {
	client    AudioQueryClient
	styles    StyleFinder
	converter TextConverter
	limiter   *rate.Limiter
	config    Config
}

// New builds an Engine from the given dependencies and options.
//
// This is the only constructor. A NewWithConfig taking a prepared Config used to sit beside
// it, but production went through one and the tests only through the other, so it was just
// two doors to the same thing. To build the configuration first, pass the result of
// NewConfig as an option.
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

// Run takes structured ScriptLines and returns the combined WAV bytes.
func (e *Engine) Run(ctx context.Context, lines []ScriptLine) ([]byte, error) {
	segments, preCalcErrors, err := e.prepareSegments(ctx, lines)
	if err != nil {
		return nil, err
	}

	orderedAudioDataList, runtimeErrors := e.runSynthesisBatch(ctx, segments)

	return combineOutput(orderedAudioDataList, preCalcErrors, runtimeErrors)
}
