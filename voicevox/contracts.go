package voicevox

import (
	"time"

	"github.com/shouni/go-voicevox/internal/contracts"
)

type Engine = contracts.Engine
type AudioQueryClient = contracts.AudioQueryClient
type SpeakerClient = contracts.SpeakerClient
type APIClient = contracts.APIClient
type DataFinder = contracts.DataFinder
type Parser = contracts.Parser
type Writer = contracts.Writer
type Segment = contracts.Segment
type ScriptLine = contracts.ScriptLine
type EngineConfig = contracts.EngineConfig
type Option = contracts.Option
type RunConfig = contracts.RunConfig
type RunOption = contracts.RunOption
type WriteConfig = contracts.WriteConfig
type WriteOption = contracts.WriteOption

const (
	DefaultMaxParallelSegments = contracts.DefaultMaxParallelSegments
	DefaultSegmentTimeout      = contracts.DefaultSegmentTimeout
	DefaultSegmentRateLimit    = contracts.DefaultSegmentRateLimit
)

func WithMaxParallelSegments(n int) Option {
	return contracts.WithMaxParallelSegments(n)
}

func WithSegmentTimeout(dur time.Duration) Option {
	return contracts.WithSegmentTimeout(dur)
}

func WithSegmentRateLimit(dur time.Duration) Option {
	return contracts.WithSegmentRateLimit(dur)
}

// WithPhoneticPreprocessing は合成前テキストをカタカナ読みへ変換する前処理を有効化します。
func WithPhoneticPreprocessing(enabled bool) Option {
	return contracts.WithPhoneticPreprocessing(enabled)
}

// WithParser はスクリプト解析に使用する Parser 依存を差し替えます。
func WithParser(parser Parser) Option {
	return contracts.WithParser(parser)
}

func NewRunConfig() *RunConfig {
	return contracts.NewRunConfig()
}

func NewWriteConfig(opts ...WriteOption) WriteConfig {
	return contracts.NewWriteConfig(opts...)
}

// WithFallbackTag はタグなしテキストに適用する完全な [話者][スタイル] タグを設定します。
// 例: "[ずんだもん][ノーマル]"
func WithFallbackTag(tag string) RunOption {
	return contracts.WithFallbackTag(tag)
}

func WithContentType(contentType string) WriteOption {
	return contracts.WithContentType(contentType)
}

func WithInline(inline bool) WriteOption {
	return contracts.WithInline(inline)
}

func WithCacheControl(cacheControl string) WriteOption {
	return contracts.WithCacheControl(cacheControl)
}
