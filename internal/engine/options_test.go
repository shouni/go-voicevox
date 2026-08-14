package engine

import (
	"testing"
	"time"
)

func TestNewConfigAppliesDefaultsAndOptionsOnce(t *testing.T) {
	calls := 0
	cfg := NewConfig(func(cfg *Config) {
		calls++
		cfg.MaxParallelSegments = 7
	})

	if calls != 1 {
		t.Fatalf("option calls = %d, want 1", calls)
	}
	if cfg.MaxParallelSegments != 7 {
		t.Fatalf("MaxParallelSegments = %d, want 7", cfg.MaxParallelSegments)
	}
	if cfg.SegmentTimeout != DefaultSegmentTimeout {
		t.Fatalf("SegmentTimeout = %v, want %v", cfg.SegmentTimeout, DefaultSegmentTimeout)
	}
	if cfg.SegmentRateLimit != DefaultSegmentRateLimit {
		t.Fatalf("SegmentRateLimit = %v, want %v", cfg.SegmentRateLimit, DefaultSegmentRateLimit)
	}
}

func TestOptionsApplyOnlyPositiveValues(t *testing.T) {
	cfg := Config{
		MaxParallelSegments: 9,
		SegmentTimeout:      9 * time.Second,
		SegmentRateLimit:    9 * time.Millisecond,
	}

	WithMaxParallelSegments(0)(&cfg)
	WithSegmentTimeout(0)(&cfg)
	WithSegmentRateLimit(0)(&cfg)
	if cfg.MaxParallelSegments != 9 || cfg.SegmentTimeout != 9*time.Second || cfg.SegmentRateLimit != 9*time.Millisecond {
		t.Fatalf("zero-value options should not overwrite config: %+v", cfg)
	}

	WithMaxParallelSegments(2)(&cfg)
	WithSegmentTimeout(3 * time.Second)(&cfg)
	WithSegmentRateLimit(4 * time.Millisecond)(&cfg)
	if cfg.MaxParallelSegments != 2 {
		t.Fatalf("MaxParallelSegments = %d, want 2", cfg.MaxParallelSegments)
	}
	if cfg.SegmentTimeout != 3*time.Second {
		t.Fatalf("SegmentTimeout = %v, want 3s", cfg.SegmentTimeout)
	}
	if cfg.SegmentRateLimit != 4*time.Millisecond {
		t.Fatalf("SegmentRateLimit = %v, want 4ms", cfg.SegmentRateLimit)
	}
}
