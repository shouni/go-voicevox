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
type Segment = contracts.Segment
type EngineConfig = contracts.EngineConfig
type Option = contracts.Option
type RunConfig = contracts.RunConfig
type RunOption = contracts.RunOption

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

func NewRunConfig() *RunConfig {
	return contracts.NewRunConfig()
}

func WithFallbackTag(tag string) RunOption {
	return contracts.WithFallbackTag(tag)
}
