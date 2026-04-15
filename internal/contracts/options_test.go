package contracts

import (
	"testing"
	"time"
)

func TestNewRunConfigUsesDefaultFallback(t *testing.T) {
	cfg := NewRunConfig()
	if cfg.FallbackTag != DefaultFallbackTag {
		t.Fatalf("FallbackTag = %q, want %q", cfg.FallbackTag, DefaultFallbackTag)
	}
}

func TestOptionsApplyOnlyPositiveValues(t *testing.T) {
	cfg := EngineConfig{
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

func TestWithFallbackTagIgnoresEmptyValue(t *testing.T) {
	cfg := NewRunConfig()
	WithFallbackTag("")(cfg)
	if cfg.FallbackTag != DefaultFallbackTag {
		t.Fatalf("FallbackTag = %q, want %q", cfg.FallbackTag, DefaultFallbackTag)
	}

	WithFallbackTag("[めたん][ノーマル]")(cfg)
	if cfg.FallbackTag != "[めたん][ノーマル]" {
		t.Fatalf("FallbackTag = %q", cfg.FallbackTag)
	}
}
