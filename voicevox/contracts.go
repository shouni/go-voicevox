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
type Segment = contracts.Segment
type ScriptLine = contracts.ScriptLine
type EngineConfig = contracts.EngineConfig
type Option = contracts.Option

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
