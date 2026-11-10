package engine

import "time"

// The engine's defaults. Callers do not redefine them; this is their single source.
const (
	// DefaultMaxParallelSegments is the default parallelism for segment synthesis.
	DefaultMaxParallelSegments = 5
	// DefaultSegmentTimeout is the default timeout per segment.
	DefaultSegmentTimeout = 180 * time.Second
	// DefaultSegmentRateLimit is the default rate-limit interval for segment synthesis.
	//
	// It is not a throughput knob. errgroup bounds concurrency with MaxParallelSegments, so
	// this interval's only job is to smooth out the burst of connections at startup. The
	// interval can only bound throughput when "time per segment < parallelism × interval",
	// and against the measured 25-33 seconds per segment it is orders of magnitude off and
	// never takes effect.
	//
	// It used to be 1 second. With a burst of 1 it never limited anything, yet it put a wait
	// of (parallelism - 1) × interval at the head of every batch (4 seconds at the default 5),
	// and since a segment whose ctx expires while waiting counts as failed, it widened the
	// window in which a cutoff takes the tail of the batch with it.
	DefaultSegmentRateLimit = 100 * time.Millisecond
)

// Config holds the synthesis engine's settings.
type Config struct {
	MaxParallelSegments int
	SegmentTimeout      time.Duration
	SegmentRateLimit    time.Duration
}

// Option is a function that modifies a Config.
type Option func(*Config)

// NewConfig returns the defaults with the options applied.
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

// WithMaxParallelSegments sets the parallelism for segment synthesis (0 or less is ignored).
func WithMaxParallelSegments(n int) Option {
	return func(e *Config) {
		if n > 0 {
			e.MaxParallelSegments = n
		}
	}
}

// WithSegmentTimeout sets the timeout per segment (0 or less is ignored).
func WithSegmentTimeout(d time.Duration) Option {
	return func(e *Config) {
		if d > 0 {
			e.SegmentTimeout = d
		}
	}
}

// WithSegmentRateLimit sets the rate-limit interval for segment synthesis (0 or less is
// ignored).
func WithSegmentRateLimit(d time.Duration) Option {
	return func(e *Config) {
		if d > 0 {
			e.SegmentRateLimit = d
		}
	}
}
