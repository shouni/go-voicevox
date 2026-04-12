package runner

import (
	"time"
)

// Option は Engine の初期化時に設定を適用するための関数型です。
type Option func(*Engine)

// WithMaxParallelSegments は、同時に実行する音声合成リクエストの最大数を設定します。
func WithMaxParallelSegments(n int) Option {
	return func(e *Engine) {
		if n > 0 {
			e.MaxParallelSegments = n
		}
	}
}

// WithSegmentTimeout は、単一セグメントの合成に許容される最大時間を設定します。
func WithSegmentTimeout(d time.Duration) Option {
	return func(e *Engine) {
		if d > 0 {
			e.SegmentTimeout = d
		}
	}
}

// WithSegmentRateLimit は、セグメントごとのリクエスト間隔（レートリミット）を設定します。
func WithSegmentRateLimit(d time.Duration) Option {
	return func(e *Engine) {
		if d > 0 {
			e.SegmentRateLimit = d
		}
	}
}
