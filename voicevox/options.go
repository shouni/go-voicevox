package voicevox

import (
	"time"

	"github.com/shouni/go-voicevox/speaker"
)

// options は Engine の動作を制御するための内部設定情報です。
type options struct {
	MaxParallelSegments int
	SegmentTimeout      time.Duration
	SegmentRateLimit    time.Duration
}

// Option は Engine の初期化時に設定を適用するための関数型です。
type Option func(*options)

// --- Engine 初期化用オプション (NewEngine で利用) ---

// WithMaxParallelSegments は、同時に実行する音声合成リクエストの最大数を設定します。
func WithMaxParallelSegments(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.MaxParallelSegments = n
		}
	}
}

// WithSegmentTimeout は、単一セグメントの合成に許容される最大時間を設定します。
func WithSegmentTimeout(d time.Duration) Option {
	return func(o *options) {
		if d > 0 {
			o.SegmentTimeout = d
		}
	}
}

// WithSegmentRateLimit は、セグメントごとのリクエスト間隔（レートリミット）を設定します。
func WithSegmentRateLimit(d time.Duration) Option {
	return func(o *options) {
		if d > 0 {
			o.SegmentRateLimit = d
		}
	}
}

// --- Execute メソッド用オプション ---

// ExecuteConfig は Execute メソッドの実行中に適用される設定を保持します。
type ExecuteConfig struct {
	// FallbackTag は、Style ID が見つからない場合に使用される代替タグです。
	FallbackTag string
}

// ExecuteOption は ExecuteConfig に対して設定を適用するための関数型です。
type ExecuteOption func(*ExecuteConfig)

// newExecuteConfig は Execute のデフォルト設定を生成します。
func newExecuteConfig() *ExecuteConfig {
	return &ExecuteConfig{
		FallbackTag: speaker.VvTagNormal,
	}
}

// WithFallbackTag は、Style ID 解決に失敗した際に使用するフォールバック用のタグを指定します。
func WithFallbackTag(tag string) ExecuteOption {
	return func(cfg *ExecuteConfig) {
		if tag != "" {
			cfg.FallbackTag = tag
		}
	}
}
