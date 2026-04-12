package ports

import "github.com/shouni/go-voicevox/speaker"

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
		FallbackTag: speaker.VvTagNormal,
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
