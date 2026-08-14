package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

// segmentResult は合成処理の結果を保持します。
type segmentResult struct {
	wavData []byte
	err     error
	// duration は 1 セグメントの所要時間です。並列数が妥当かを判断する材料になります。
	// 進捗ログの時刻から逆算すると、進捗が出る間隔（5件ごと）より細かくは分かりません。
	duration time.Duration
}

// isSynthesizable は、そのセグメントを API へ投げるかどうかを返します。
//
// 空テキストは合成するものがなく、スタイル ID の解決に失敗したものは
// 事前エラーとして既に数えられているため、どちらも投げません。
func isSynthesizable(seg engineSegment) bool {
	return seg.Text != "" && seg.Err == nil
}

// processSegment は API に対してクエリと合成を実行します。
func (e *Engine) processSegment(ctx context.Context, seg engineSegment, index int) segmentResult {
	queryBody, err := e.client.RunAudioQuery(ctx, seg.Text, seg.StyleID)
	if err != nil {
		return segmentResult{err: fmt.Errorf("セグメント %d のオーディオクエリ失敗: %w", index, err)}
	}

	wavData, err := e.client.RunSynthesis(ctx, queryBody, seg.StyleID)
	if err != nil {
		return segmentResult{err: fmt.Errorf("セグメント %d の音声合成失敗: %w", index, err)}
	}

	return segmentResult{wavData: wavData}
}

// runSynthesisBatch は音声合成タスクを並列処理します。
func (e *Engine) runSynthesisBatch(ctx context.Context, segments []engineSegment) ([][]byte, []error) {
	// errgroup.WithContext は使いません。**どのゴルーチンもエラーを返さない**ためです。
	// 失敗は results に記録して集計側へ渡します。1 件の失敗で残りを打ち切ると、
	// バッチ全体で何が起きたかを 1 つのエラーにまとめる ErrSynthesisBatch の意図に反します。
	var g errgroup.Group
	g.SetLimit(e.config.MaxParallelSegments)

	total := len(segments)
	var completed int32
	results := make([]*segmentResult, total)

	slog.Info("音声合成バッチ処理開始", "total_segments", total, "max_parallel", e.config.MaxParallelSegments)

	for i, seg := range segments {
		if !isSynthesizable(seg) {
			atomic.AddInt32(&completed, 1)
			continue
		}

		g.Go(func() error {
			// **待機の失敗も結果として残します。** ここで results[i] を nil のまま抜けると、
			// 集計側はそのセグメントを「無かったもの」として扱い、途中までの音声が
			// エラー無しで返ります。打ち切り（ctx のキャンセル）でまさにこれが起きていました。
			if err := e.limiter.Wait(ctx); err != nil {
				// rate.Limiter は期限超過を**予測した**時点で独自のエラーを返し、
				// ctx.Err() を包みません。既に打ち切られている場合はそちらを原因に
				// 据え直して、呼び出し側が errors.Is で打ち切りだと判別できるようにします。
				if ctxErr := ctx.Err(); ctxErr != nil {
					err = ctxErr
				}
				results[i] = &segmentResult{err: fmt.Errorf("セグメント %d は開始前に中断されました: %w", i, err)}
				return nil
			}

			segCtx, cancel := context.WithTimeout(ctx, e.config.SegmentTimeout)
			defer cancel()

			startedAt := time.Now()
			res := e.processSegment(segCtx, seg, i)
			res.duration = time.Since(startedAt)
			results[i] = &res

			done := atomic.AddInt32(&completed, 1)
			logSynthesisProgress(done, total, i, seg, res.duration)
			return nil
		})
	}

	// 各ゴルーチンは常に nil を返すため、ここで受け取るエラーはありません。
	_ = g.Wait()

	logSynthesisSummary(total, results)

	return collectSynthesisResults(segments, results)
}

// collectSynthesisResults は、各セグメントの結果を元の順序のまま取り出します。
//
// 返す音声のスライスは**セグメントと同じ長さ**で、合成しなかった位置は nil のままです。
// ここで詰めてしまうと、結合時のエラーが指す位置を元のセグメント番号へ戻せません。
func collectSynthesisResults(segments []engineSegment, results []*segmentResult) ([][]byte, []error) {
	orderedAudioDataList := make([][]byte, len(results))
	runtimeErrors := make([]error, 0)

	for i, res := range results {
		if !isSynthesizable(segments[i]) {
			// 投げていないセグメントです。スタイル ID の解決に失敗した分は
			// preCalcErrors が既に持っているため、ここでは数えません。
			continue
		}
		if res == nil {
			// **投げるはずのセグメントに結果がありません。** 黙って捨てると、
			// 欠けたまま結合された音声が成功として返ります。
			runtimeErrors = append(runtimeErrors, fmt.Errorf("セグメント %d の結果が記録されませんでした", i))
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

// logSynthesisSummary は、バッチ全体の所要時間を 1 行にまとめます。
//
// **並列数を決めるのに要る数字です。** 1 セグメントの所要時間が分かって初めて、
// レート制限と並列数のどちらがスループットを縛っているかを判断できます
// （スループット = min(1/レート制限, 並列数 ÷ 1セグメントの所要時間)）。
func logSynthesisSummary(total int, results []*segmentResult) {
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
		slog.Info("全セグメントの処理が終了しました", "total", total, "succeeded", 0)
		return
	}

	slog.Info("全セグメントの処理が終了しました",
		"total", total,
		"succeeded", count,
		"segment_duration_avg", (sum / time.Duration(count)).Round(time.Millisecond).String(),
		"segment_duration_min", fastest.Round(time.Millisecond).String(),
		"segment_duration_max", slowest.Round(time.Millisecond).String(),
	)
}

func logSynthesisProgress(done int32, total int, index int, seg engineSegment, duration time.Duration) {
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
			"duration": duration.Round(time.Millisecond).String(),
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
