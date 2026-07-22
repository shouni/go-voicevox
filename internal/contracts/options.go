package contracts

import "time"

const (
	DefaultMaxParallelSegments = 5
	DefaultSegmentTimeout      = 180 * time.Second
	DefaultSegmentRateLimit    = 1000 * time.Millisecond
)

type EngineConfig struct {
	MaxParallelSegments int
	SegmentTimeout      time.Duration
	SegmentRateLimit    time.Duration
}

type Option func(*EngineConfig)

func NewEngineConfig(opts ...Option) EngineConfig {
	cfg := EngineConfig{
		MaxParallelSegments: DefaultMaxParallelSegments,
		SegmentTimeout:      DefaultSegmentTimeout,
		SegmentRateLimit:    DefaultSegmentRateLimit,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

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
