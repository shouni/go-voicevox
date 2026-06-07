package contracts

import "time"

const (
	DefaultMaxParallelSegments = 5
	DefaultSegmentTimeout      = 180 * time.Second
	DefaultSegmentRateLimit    = 1000 * time.Millisecond
	// DefaultFallbackTag は未設定です。
	// フォールバックを使う場合は [話者][スタイル] の完全なタグを明示的に指定してください。
	DefaultFallbackTag = ""
)

type EngineConfig struct {
	MaxParallelSegments   int
	SegmentTimeout        time.Duration
	SegmentRateLimit      time.Duration
	PhoneticPreprocessing bool
}

type Option func(*EngineConfig)

func WithMaxParallelSegments(n int) Option {
	return func(e *EngineConfig) {
		if n > 0 {
			e.MaxParallelSegments = n
		}
	}
}

func WithSegmentTimeout(d time.Duration) Option {
	return func(e *EngineConfig) {
		if d > 0 {
			e.SegmentTimeout = d
		}
	}
}

func WithSegmentRateLimit(d time.Duration) Option {
	return func(e *EngineConfig) {
		if d > 0 {
			e.SegmentRateLimit = d
		}
	}
}

func WithPhoneticPreprocessing(enabled bool) Option {
	return func(e *EngineConfig) {
		e.PhoneticPreprocessing = enabled
	}
}

type RunConfig struct {
	// FallbackTag は、タグなしテキストに適用する完全な [話者][スタイル] タグです。
	// 例: "[ずんだもん][ノーマル]"
	FallbackTag string
}

type RunOption func(*RunConfig)

func NewRunConfig() *RunConfig {
	return &RunConfig{
		FallbackTag: DefaultFallbackTag,
	}
}

func WithFallbackTag(tag string) RunOption {
	return func(cfg *RunConfig) {
		if tag != "" {
			cfg.FallbackTag = tag
		}
	}
}

type WriteConfig struct {
	ContentType  string
	Inline       bool
	CacheControl string
}

type WriteOption func(*WriteConfig)

func NewWriteConfig(opts ...WriteOption) WriteConfig {
	cfg := WriteConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

func WithContentType(contentType string) WriteOption {
	return func(cfg *WriteConfig) {
		if contentType != "" {
			cfg.ContentType = contentType
		}
	}
}

func WithInline(inline bool) WriteOption {
	return func(cfg *WriteConfig) {
		cfg.Inline = inline
	}
}

func WithCacheControl(cacheControl string) WriteOption {
	return func(cfg *WriteConfig) {
		if cacheControl != "" {
			cfg.CacheControl = cacheControl
		}
	}
}
