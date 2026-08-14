package engine

import "time"

// エンジンの既定値です。呼び出し側で再定義せず、ここを唯一の出所とします。
const (
	// DefaultMaxParallelSegments は、セグメント合成の既定の並列数です。
	DefaultMaxParallelSegments = 5
	// DefaultSegmentTimeout は、セグメント1件あたりの既定のタイムアウトです。
	DefaultSegmentTimeout = 180 * time.Second
	// DefaultSegmentRateLimit は、セグメント合成の既定のレート制限間隔です。
	DefaultSegmentRateLimit = 1000 * time.Millisecond
)

// Config は、合成エンジンの動作設定です。
type Config struct {
	MaxParallelSegments int
	SegmentTimeout      time.Duration
	SegmentRateLimit    time.Duration
}

// Option は、Config を変更する関数型です。
type Option func(*Config)

// NewConfig は、既定値へオプションを適用した設定を返します。
func NewConfig(opts ...Option) Config {
	cfg := Config{
		MaxParallelSegments: DefaultMaxParallelSegments,
		SegmentTimeout:      DefaultSegmentTimeout,
		SegmentRateLimit:    DefaultSegmentRateLimit,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// WithMaxParallelSegments は、セグメント合成の並列数を設定します（0以下は無視）。
func WithMaxParallelSegments(n int) Option {
	return func(e *Config) {
		if n > 0 {
			e.MaxParallelSegments = n
		}
	}
}

// WithSegmentTimeout は、セグメント1件あたりのタイムアウトを設定します（0以下は無視）。
func WithSegmentTimeout(d time.Duration) Option {
	return func(e *Config) {
		if d > 0 {
			e.SegmentTimeout = d
		}
	}
}

// WithSegmentRateLimit は、セグメント合成のレート制限間隔を設定します（0以下は無視）。
func WithSegmentRateLimit(d time.Duration) Option {
	return func(e *Config) {
		if d > 0 {
			e.SegmentRateLimit = d
		}
	}
}
