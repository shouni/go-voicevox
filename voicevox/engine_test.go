package voicevox

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/shouni/audio/phonetic"

	internalengine "github.com/shouni/go-voicevox/internal/engine"
)

// stubRequester is a minimal Requester stub.
// New hits /speakers to load speaker data, so only that response is replaced.
type stubRequester struct {
	speakersJSON string
	fetchErr     error
	lastTarget   string
}

func (s *stubRequester) SendBytes(*http.Request) ([]byte, error) { return nil, nil }

func (s *stubRequester) GetBytes(_ context.Context, target string) ([]byte, error) {
	s.lastTarget = target
	if s.fetchErr != nil {
		return nil, s.fetchErr
	}
	return []byte(s.speakersJSON), nil
}

// LoadStyles requires at least one speaker with a speech style.
const stubSpeakersJSON = `[
  {"name":"話者アルファ","styles":[{"name":"標準","id":2}]},
  {"name":"話者ベータ","styles":[{"name":"標準","id":3}]},
  {"name":"話者ガンマ","styles":[{"name":"標準","id":8}]}
]`

func TestNew_BuildsEngineWhenEnabled(t *testing.T) {
	reqer := &stubRequester{speakersJSON: stubSpeakersJSON}

	engine, err := New(context.Background(), reqer, "http://voicevox.test:50021", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if engine == nil {
		t.Fatal("New() returned nil")
	}
	if !strings.HasPrefix(reqer.lastTarget, "http://voicevox.test:50021") {
		t.Errorf("the given URL was not used: %q", reqer.lastTarget)
	}
}

// TestNew_FallsBackToDefaultURL checks that the default local URL is used when no URL is given.
func TestNew_FallsBackToDefaultURL(t *testing.T) {
	reqer := &stubRequester{speakersJSON: stubSpeakersJSON}

	if _, err := New(context.Background(), reqer, "", nil); err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if !strings.HasPrefix(reqer.lastTarget, defaultVoicevoxAPIURL) {
		t.Errorf("the default URL was not used: %q (want prefix %q)", reqer.lastTarget, defaultVoicevoxAPIURL)
	}
}

func TestNew_ReturnsErrorWhenSpeakerLoadFails(t *testing.T) {
	reqer := &stubRequester{fetchErr: errors.New("cannot connect")}

	_, err := New(context.Background(), reqer, "http://voicevox.test:50021", nil)
	if err == nil {
		t.Fatal("no error when loading speakers failed")
	}
	if !strings.Contains(err.Error(), "load VOICEVOX data") {
		t.Errorf("the error carries no context: %v", err)
	}
}

func TestOptionsAreApplied(t *testing.T) {
	o := newOptions(
		WithMaxParallelSegments(9),
		WithSegmentTimeout(7*time.Second),
		WithSegmentRateLimit(250*time.Millisecond),
	)
	cfg := internalengine.NewConfig(o.engine...)

	if cfg.MaxParallelSegments != 9 {
		t.Errorf("MaxParallelSegments = %d, want 9", cfg.MaxParallelSegments)
	}
	if cfg.SegmentTimeout != 7*time.Second {
		t.Errorf("SegmentTimeout = %v, want 7s", cfg.SegmentTimeout)
	}
	if cfg.SegmentRateLimit != 250*time.Millisecond {
		t.Errorf("SegmentRateLimit = %v, want 250ms", cfg.SegmentRateLimit)
	}
}

// TestOptionsIgnoreNonPositive checks that values of 0 or less do not break the defaults.
func TestOptionsIgnoreNonPositive(t *testing.T) {
	o := newOptions(
		WithMaxParallelSegments(0),
		WithSegmentTimeout(-time.Second),
		WithSegmentRateLimit(0),
	)
	cfg := internalengine.NewConfig(o.engine...)

	if cfg.MaxParallelSegments != internalengine.DefaultMaxParallelSegments {
		t.Errorf("MaxParallelSegments = %d, want %d", cfg.MaxParallelSegments, internalengine.DefaultMaxParallelSegments)
	}
	if cfg.SegmentTimeout != internalengine.DefaultSegmentTimeout {
		t.Errorf("SegmentTimeout = %v, want %v", cfg.SegmentTimeout, internalengine.DefaultSegmentTimeout)
	}
	if cfg.SegmentRateLimit != internalengine.DefaultSegmentRateLimit {
		t.Errorf("SegmentRateLimit = %v, want %v", cfg.SegmentRateLimit, internalengine.DefaultSegmentRateLimit)
	}
}

// TestWithReadingOverridesReachesConverter checks that the given readings actually take effect
// in the conversion. It is the one place that checks the option reaches the converter rather
// than the engine.
func TestWithReadingOverridesReachesConverter(t *testing.T) {
	o := newOptions(WithReadingOverrides(map[string]string{"8日": "ヨウカ"}))

	converter, err := phonetic.NewConverter(o.converter...)
	if err != nil {
		t.Fatalf("NewConverter() error = %v", err)
	}

	if got := converter.ConvertToReading("8日"); !strings.Contains(got, "ヨウカ") {
		t.Errorf("ConvertToReading(\"8日\") = %q, does not contain ヨウカ", got)
	}
}

// TestWithNumberReadingReachesConverter checks that the number-reading setting actually takes
// effect in the conversion. By default Arabic numerals stay as written, so it checks that
// difference.
func TestWithNumberReadingReachesConverter(t *testing.T) {
	base, err := phonetic.NewConverter()
	if err != nil {
		t.Fatalf("NewConverter() error = %v", err)
	}
	if got := base.ConvertToReading("8日"); !strings.Contains(got, "8") {
		t.Fatalf("got %q by default; the premise that digits are not read has changed", got)
	}

	o := newOptions(WithNumberReading())
	converter, err := phonetic.NewConverter(o.converter...)
	if err != nil {
		t.Fatalf("NewConverter() error = %v", err)
	}

	got := converter.ConvertToReading("8日")
	if strings.Contains(got, "8") {
		t.Errorf("ConvertToReading(\"8日\") = %q, Arabic numerals remain", got)
	}
}

// TestWithReadingOverridesIgnoresEmpty checks that an empty value adds no converter setting,
// so a call passing nil does not make the converter rebuild its default dictionary for
// nothing.
func TestWithReadingOverridesIgnoresEmpty(t *testing.T) {
	if o := newOptions(WithReadingOverrides(nil)); len(o.converter) != 0 {
		t.Errorf("%d converter options were added, want 0", len(o.converter))
	}
	if o := newOptions(WithReadingOverrides(map[string]string{})); len(o.converter) != 0 {
		t.Errorf("%d converter options were added, want 0", len(o.converter))
	}
}
