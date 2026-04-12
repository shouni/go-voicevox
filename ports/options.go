package ports

import (
	"time"
)

const (
	DefaultMaxParallelSegments = 5
	DefaultSegmentTimeout      = 180 * time.Second
	DefaultSegmentRateLimit    = 1000 * time.Millisecond
	vvTagNormal                = "[ノーマル]"
)

type EngineConfig struct {
	MaxParallelSegments int
	SegmentTimeout      time.Duration
	SegmentRateLimit    time.Duration
}

// EngineOption は Engine の初期化時に設定を適用するための関数型です。
type EngineOption func(*EngineConfig)

// WithMaxParallelSegments は、同時に実行する音声合成リクエストの最大数を設定します。
func WithMaxParallelSegments(n int) EngineOption {
	return func(e *EngineConfig) {
		if n > 0 {
			e.MaxParallelSegments = n
		}
	}
}

// WithSegmentTimeout は、単一セグメントの合成に許容される最大時間を設定します。
func WithSegmentTimeout(d time.Duration) EngineOption {
	return func(e *EngineConfig) {
		if d > 0 {
			e.SegmentTimeout = d
		}
	}
}

// WithSegmentRateLimit は、セグメントごとのリクエスト間隔（レートリミット）を設定します。
func WithSegmentRateLimit(d time.Duration) EngineOption {
	return func(e *EngineConfig) {
		if d > 0 {
			e.SegmentRateLimit = d
		}
	}
}

// RunConfig は Run メソッドの実行中に適用される設定を保持します。
type RunConfig struct {
	// FallbackTag は、Style ID が見つからない場合に使用される代替タグです。
	FallbackTag string
}

// RunOption は、RunConfig に対して設定を適用するための関数型です。
type RunOption func(*RunConfig)

// NewRunConfig は、Run のデフォルト設定を生成します。
func NewRunConfig() *RunConfig {
	return &RunConfig{
		FallbackTag: vvTagNormal,
	}
}

// WithFallbackTag は、Style ID 解決に失敗した際に使用するフォールバック用のタグを指定します。
func WithFallbackTag(tag string) RunOption {
	return func(cfg *RunConfig) {
		if tag != "" {
			cfg.FallbackTag = tag
		}
	}
}
