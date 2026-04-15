package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

// segmentResult は合成処理の結果を保持します。
type segmentResult struct {
	index   int
	wavData []byte
	err     error
}

// processSegment は API に対してクエリと合成を実行します。
func (e *Engine) processSegment(ctx context.Context, seg engineSegment, index int) segmentResult {
	if seg.Err != nil {
		return segmentResult{index: index, err: seg.Err}
	}

	queryBody, err := e.client.RunAudioQuery(ctx, seg.Text, seg.StyleID)
	if err != nil {
		return segmentResult{index: index, err: fmt.Errorf("セグメント %d のオーディオクエリ失敗: %w", index, err)}
	}

	wavData, err := e.client.RunSynthesis(ctx, queryBody, seg.StyleID)
	if err != nil {
		return segmentResult{index: index, err: fmt.Errorf("セグメント %d の音声合成失敗: %w", index, err)}
	}

	return segmentResult{index: index, wavData: wavData}
}

// runSynthesisBatch は音声合成タスクを並列処理します。
func (e *Engine) runSynthesisBatch(ctx context.Context, segments []engineSegment) ([][]byte, []string) {
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(e.config.MaxParallelSegments)

	total := len(segments)
	var completed int32
	results := make([]*segmentResult, total)

	slog.Info("音声合成バッチ処理開始", "total_segments", total, "max_parallel", e.config.MaxParallelSegments)

	for i, seg := range segments {
		if seg.Text == "" || seg.Err != nil {
			atomic.AddInt32(&completed, 1)
			continue
		}

		index := i
		segment := seg
		g.Go(func() error {
			if err := e.limiter.Wait(gCtx); err != nil {
				return fmt.Errorf("リミッター待機中にエラーが発生しました: %w", err)
			}

			segCtx, cancel := context.WithTimeout(gCtx, e.config.SegmentTimeout)
			defer cancel()
			result := e.processSegment(segCtx, segment, index)
			results[index] = &result

			done := atomic.AddInt32(&completed, 1)
			logSynthesisProgress(done, total, index, segment)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		slog.ErrorContext(ctx, "音声合成バッチ処理中にエラーが発生しました", "error", err)
	}

	slog.Info("全セグメントの処理が終了しました", "total", total)
	return collectSynthesisResults(results, total)
}

func collectSynthesisResults(results []*segmentResult, total int) ([][]byte, []string) {
	orderedAudioDataList := make([][]byte, 0, total)
	runtimeErrors := make([]string, 0)

	for _, res := range results {
		if res == nil {
			continue
		}
		if res.err != nil {
			runtimeErrors = append(runtimeErrors, res.err.Error())
			continue
		}
		if len(res.wavData) > 0 {
			orderedAudioDataList = append(orderedAudioDataList, res.wavData)
		}
	}

	return orderedAudioDataList, runtimeErrors
}

func logSynthesisProgress(done int32, total int, index int, seg engineSegment) {
	if done%5 != 0 && done != int32(total) {
		return
	}

	percentage := float64(done) / float64(total) * 100
	slog.Info("音声合成進捗",
		"progress", fmt.Sprintf("%.1f%% (%d/%d)", percentage, done, total),
		"current_segment", map[string]any{
			"index":    index,
			"style_id": seg.StyleID,
			"text":     truncateString(seg.Text, 20),
			"length":   len([]rune(seg.Text)),
		},
	)
}

func truncateString(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen]) + "..."
}
