package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"golang.org/x/sync/errgroup"
)

// segmentResult holds the outcome of synthesizing one segment.
type segmentResult struct {
	wavData []byte
	err     error
	// duration is the time one segment took, which is what tells whether the parallelism is
	// right. Working it back from progress log timestamps gives nothing finer than the
	// interval at which progress is logged (every 5 segments).
	duration time.Duration
}

// isSynthesizable reports whether the segment is sent to the API.
//
// Neither kind is sent: empty text has nothing to synthesize, and a segment whose style ID
// failed to resolve has already been counted as a preparation-time error.
func isSynthesizable(seg segment) bool {
	return seg.Text != "" && seg.Err == nil
}

// processSegment runs the query and the synthesis against the API.
func (e *Engine) processSegment(ctx context.Context, seg segment, index int) segmentResult {
	queryBody, err := e.client.RunAudioQuery(ctx, seg.Text, seg.StyleID)
	if err != nil {
		return segmentResult{err: fmt.Errorf("segment %d audio query: %w", index, err)}
	}

	wavData, err := e.client.RunSynthesis(ctx, queryBody, seg.StyleID)
	if err != nil {
		return segmentResult{err: fmt.Errorf("segment %d synthesis: %w", index, err)}
	}

	return segmentResult{wavData: wavData}
}

// runSynthesisBatch synthesizes the segments in parallel.
func (e *Engine) runSynthesisBatch(ctx context.Context, segments []segment) ([][]byte, []error) {
	// errgroup.WithContext is not used, because no goroutine returns an error. Failures are
	// recorded in results and handed to the collector. Aborting the rest on one failure would
	// defeat ErrSynthesisBatch, whose point is to report everything that happened in the
	// batch as one error.
	var g errgroup.Group
	g.SetLimit(e.config.MaxParallelSegments)

	total := len(segments)
	var completed atomic.Int32
	results := make([]*segmentResult, total)

	slog.InfoContext(ctx, "synthesis batch started", "total_segments", total, "max_parallel", e.config.MaxParallelSegments)

	for i, seg := range segments {
		if !isSynthesizable(seg) {
			completed.Add(1)
			continue
		}

		g.Go(func() error {
			// A failed wait is recorded as a result too. Leaving results[i] nil here makes the
			// collector treat the segment as if it never existed, and the partial audio comes
			// back with no error. That is exactly what happened on a cutoff (ctx cancellation).
			if err := e.limiter.Wait(ctx); err != nil {
				// rate.Limiter returns its own error as soon as it predicts a deadline overrun,
				// without wrapping ctx.Err(). When the context is already done, that is put
				// back as the cause, so the caller can recognise the cutoff with errors.Is.
				if ctxErr := ctx.Err(); ctxErr != nil {
					err = ctxErr
				}
				results[i] = &segmentResult{err: fmt.Errorf("segment %d was interrupted before it started: %w", i, err)}
				return nil
			}

			segCtx, cancel := context.WithTimeout(ctx, e.config.SegmentTimeout)
			defer cancel()

			startedAt := time.Now()
			res := e.processSegment(segCtx, seg, i)
			res.duration = time.Since(startedAt)
			results[i] = &res

			done := completed.Add(1)
			logSynthesisProgress(ctx, done, total, i, seg, res.duration)
			return nil
		})
	}

	// Every goroutine returns nil, so there is no error to receive here.
	_ = g.Wait()

	logSynthesisSummary(ctx, total, results)

	return collectSynthesisResults(segments, results)
}

// collectSynthesisResults takes each segment's result out in the original order.
//
// The returned audio slice has the same length as segments, with nil left at every position
// that was not synthesized. Compacting it here would make it impossible to map a combine
// error's position back to the original segment number.
func collectSynthesisResults(segments []segment, results []*segmentResult) ([][]byte, []error) {
	orderedAudioDataList := make([][]byte, len(results))
	runtimeErrors := make([]error, 0)

	for i, res := range results {
		if !isSynthesizable(segments[i]) {
			// A segment that was not sent. Those that failed to resolve a style ID are already
			// in preCalcErrors, so they are not counted here.
			continue
		}
		if res == nil {
			// A segment that should have been sent has no result. Dropping it silently would
			// return audio combined with a gap in it as a success.
			runtimeErrors = append(runtimeErrors, fmt.Errorf("segment %d left no result", i))
			continue
		}
		if res.err != nil {
			runtimeErrors = append(runtimeErrors, res.err)
			continue
		}
		if len(res.wavData) > 0 {
			orderedAudioDataList[i] = res.wavData
		}
	}

	return orderedAudioDataList, runtimeErrors
}

// logSynthesisSummary sums up the batch's timings in one line.
//
// These are the numbers needed to choose the parallelism. Only once the time per segment is
// known can you tell whether the rate limit or the parallelism bounds throughput
// (throughput = min(1/rate limit, parallelism ÷ time per segment)).
func logSynthesisSummary(ctx context.Context, total int, results []*segmentResult) {
	var (
		count                 int
		sum, fastest, slowest time.Duration
	)
	for _, res := range results {
		if res == nil || res.err != nil || res.duration <= 0 {
			continue
		}
		count++
		sum += res.duration
		if fastest == 0 || res.duration < fastest {
			fastest = res.duration
		}
		if res.duration > slowest {
			slowest = res.duration
		}
	}

	if count == 0 {
		slog.InfoContext(ctx, "all segments processed", "total", total, "succeeded", 0)
		return
	}

	slog.InfoContext(ctx, "all segments processed",
		"total", total,
		"succeeded", count,
		"segment_duration_avg", (sum / time.Duration(count)).Round(time.Millisecond).String(),
		"segment_duration_min", fastest.Round(time.Millisecond).String(),
		"segment_duration_max", slowest.Round(time.Millisecond).String(),
	)
}

func logSynthesisProgress(ctx context.Context, done int32, total int, index int, seg segment, duration time.Duration) {
	if done%5 != 0 && done != int32(total) {
		return
	}

	percentage := float64(done) / float64(total) * 100
	// The attributes go through slog.Group, not a map. To a handler a map[string]any is a
	// single opaque value, so TextHandler printed it as a Go struct literal and JSON lost the
	// per-key types. As a Group it expands to keys such as current_segment.index, which the
	// logging backend can filter on.
	slog.InfoContext(ctx, "synthesis progress",
		"progress", fmt.Sprintf("%.1f%% (%d/%d)", percentage, done, total),
		slog.Group("current_segment",
			"index", index,
			"style_id", seg.StyleID,
			"text", truncateString(seg.Text, 20),
			"length", utf8.RuneCountInString(seg.Text),
			"duration", duration.Round(time.Millisecond).String(),
		),
	)
}

// truncateString shortens s to at most maxLen characters for logging.
func truncateString(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	return string([]rune(s)[:maxLen]) + "..."
}
