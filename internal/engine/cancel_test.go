package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// minimalWav returns the smallest WAV bytes that can be combined.
func minimalWav() []byte {
	b := make([]byte, 44)
	copy(b[0:], "RIFF")
	copy(b[8:], "WAVEfmt ")
	b[16], b[20], b[22] = 16, 1, 1
	b[24], b[25] = 0x40, 0x1f // 8000Hz
	b[32], b[34] = 2, 16
	copy(b[36:], "data")
	return b
}

// instantClient returns synthesis without waiting. The cutoff is caused by the rate limiter.
type instantClient struct{}

func (instantClient) RunAudioQuery(context.Context, string, int) ([]byte, error) {
	return []byte("query"), nil
}

func (instantClient) RunSynthesis(context.Context, []byte, int) ([]byte, error) {
	return minimalWav(), nil
}

// testLines returns n lines with the same speaker and style.
func testLines(n int) []ScriptLine {
	lines := make([]ScriptLine, n)
	for i := range lines {
		lines[i] = ScriptLine{Speaker: "話者アルファ", Style: "標準", Text: "テストの本文です。"}
	}
	return lines
}

// TestRunReportsSegmentsCancelledBeforeStart verifies that segments cut off before they
// started are reported as errors.
//
// This is a gap that was actually hit. When ctx expired while waiting on the rate limiter, the
// goroutine exited without writing a result. The collector discarded it as if it never
// existed, so audio with only 1 of 10 segments synthesized came back with no error.
// ap-voice's PIPELINE_TIMEOUT cuts ctx in exactly this way, so the partial WAV was saved and a
// completion notice went out.
func TestRunReportsSegmentsCancelledBeforeStart(t *testing.T) {
	t.Parallel()

	e := New(
		instantClient{},
		stubFinder{styleIDs: map[string]int{"[話者アルファ][標準]": 1}},
		stubConverter{},
		WithMaxParallelSegments(8),
		// The second and later segments wait on the rate limiter, and ctx expires before they
		// reach synthesis.
		WithSegmentRateLimit(10*time.Second),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	got, err := e.Run(ctx, testLines(10))
	if err == nil {
		t.Fatalf("no error despite the cutoff (returned %d bytes)", len(got))
	}
	if got != nil {
		t.Errorf("returned audio along with the error: %d bytes", len(got))
	}

	var batch *ErrSynthesisBatch
	if !errors.As(err, &batch) {
		t.Fatalf("not an ErrSynthesisBatch: %v", err)
	}
	// Only the first segment got through, so the other 9 should be reported.
	if len(batch.Errors) != 9 {
		t.Errorf("%d errors reported; some were dropped (want 9)", len(batch.Errors))
	}
	if !strings.Contains(err.Error(), "interrupted before it started") {
		t.Errorf("the reason for the interruption is not readable: %v", err)
	}
}

// TestCollectSynthesisResultsKeepsSegmentPositions verifies that positions not synthesized
// stay nil.
//
// Compacting them would make it impossible to map a combine error's position back to the
// original segment number (output.go's nonNilAudioData / withSegmentIndex build that mapping).
func TestCollectSynthesisResultsKeepsSegmentPositions(t *testing.T) {
	t.Parallel()

	segments := []segment{
		{Text: "あ"},
		{Text: ""}, // empty text is not sent
		{Text: "う"},
	}
	results := []*segmentResult{
		{wavData: []byte("wav-0")},
		nil,
		{wavData: []byte("wav-2")},
	}

	audio, errs := collectSynthesisResults(segments, results)
	if len(errs) != 0 {
		t.Fatalf("empty text was counted as an error: %v", errs)
	}
	if len(audio) != len(segments) {
		t.Fatalf("length does not match the segment count: %d", len(audio))
	}
	if audio[1] != nil {
		t.Error("the empty-text position was compacted away")
	}
	if string(audio[0]) != "wav-0" || string(audio[2]) != "wav-2" {
		t.Errorf("positions shifted: %q", audio)
	}
}

// TestCollectSynthesisResultsReportsMissingResult verifies that a segment that should have
// been sent but has no result is counted as an error. This is the collector's last line of
// defence.
func TestCollectSynthesisResultsReportsMissingResult(t *testing.T) {
	t.Parallel()

	segments := []segment{{Text: "あ"}}

	_, errs := collectSynthesisResults(segments, []*segmentResult{nil})
	if len(errs) != 1 {
		t.Fatalf("%d errors (want 1)", len(errs))
	}
	if !strings.Contains(errs[0].Error(), "left no result") {
		t.Errorf("the reason is not readable: %v", errs[0])
	}
}

// errClient is a client whose synthesis always returns the given error.
type errClient struct{ err error }

func (c errClient) RunAudioQuery(context.Context, string, int) ([]byte, error) {
	return nil, fmt.Errorf("call audio_query: %w", c.err)
}

func (c errClient) RunSynthesis(context.Context, []byte, int) ([]byte, error) {
	return nil, c.err
}

// TestErrSynthesisBatchKeepsErrorTypes verifies that the collected errors keep their types
// and causes.
//
// They used to be flattened to []string, so the caller could tell a cutoff from a downed
// engine only by matching message text. With Unwrap() []error, errors.Is / errors.As reach
// through the batch.
func TestErrSynthesisBatchKeepsErrorTypes(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("cannot connect to the engine")

	e := New(
		errClient{err: sentinel},
		stubFinder{styleIDs: map[string]int{"[話者アルファ][標準]": 1}},
		stubConverter{},
		WithMaxParallelSegments(4),
		WithSegmentRateLimit(time.Millisecond),
	)

	_, err := e.Run(context.Background(), testLines(3))
	if err == nil {
		t.Fatal("no error although every segment failed")
	}

	var batch *ErrSynthesisBatch
	if !errors.As(err, &batch) {
		t.Fatalf("not an ErrSynthesisBatch: %v", err)
	}
	if len(batch.Errors) != 3 {
		t.Errorf("error count = %d, want 3", len(batch.Errors))
	}
	// This is the point: the cause is reachable through both levels, batch and segment.
	if !errors.Is(err, sentinel) {
		t.Errorf("errors.Is(err, sentinel) = false: %v", err)
	}
}

// TestRunSurfacesDeadlineFromInFlightSegments verifies that the cutoff is recognisable from
// segments cut off mid-synthesis too.
//
// Those that failed while waiting wrap ctx.Err() directly and were always traceable, but
// those that fail mid-request are wrapped in api.ErrAPINetwork. Without Unwrap on that type,
// the cutoff becomes unrecognisable when there are no more segments than the parallelism (all
// of them in flight).
func TestRunSurfacesDeadlineFromInFlightSegments(t *testing.T) {
	t.Parallel()

	e := New(
		errClient{err: context.DeadlineExceeded},
		stubFinder{styleIDs: map[string]int{"[話者アルファ][標準]": 1}},
		stubConverter{},
		// The parallelism is at least the segment count, so none fail while waiting.
		WithMaxParallelSegments(8),
		WithSegmentRateLimit(time.Millisecond),
	)

	_, err := e.Run(context.Background(), testLines(3))
	if err == nil {
		t.Fatal("no error although every segment failed")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(err, context.DeadlineExceeded) = false: %v", err)
	}
}
