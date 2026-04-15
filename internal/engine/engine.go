package engine

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/shouni/go-remote-io/remoteio"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"github.com/shouni/go-voicevox/api"
	"github.com/shouni/go-voicevox/internal/contracts"
)

// Engine は VOICEVOX エンジンを利用した音声合成のメインコントローラーです。
type Engine struct {
	client            contracts.AudioQueryClient
	data              contracts.DataFinder
	parser            contracts.Parser
	limiter           *rate.Limiter
	writer            remoteio.Writer
	config            contracts.EngineConfig
	styleIDCache      map[string]int
	styleIDCacheMutex sync.RWMutex
}

// engineSegment は内部処理用のセグメント構造体です。
type engineSegment struct {
	contracts.Segment
	StyleID int
	Err     error
}

// segmentResult は合成処理の結果を保持します。
type segmentResult struct {
	index   int
	wavData []byte
	err     error
}

// New は、指定された依存関係と設定を使用して新しい Engine インスタンスを作成します。
func New(client contracts.AudioQueryClient, data contracts.DataFinder, p contracts.Parser, writer remoteio.Writer, opts ...contracts.Option) *Engine {
	allOpts := []contracts.Option{
		contracts.WithMaxParallelSegments(contracts.DefaultMaxParallelSegments),
		contracts.WithSegmentTimeout(contracts.DefaultSegmentTimeout),
		contracts.WithSegmentRateLimit(contracts.DefaultSegmentRateLimit),
	}
	allOpts = append(allOpts, opts...)

	engine := &Engine{
		client:       client,
		data:         data,
		parser:       p,
		writer:       writer,
		styleIDCache: make(map[string]int),
	}

	for _, opt := range allOpts {
		opt(&engine.config)
	}
	engine.limiter = rate.NewLimiter(rate.Every(engine.config.SegmentRateLimit), 1)

	return engine
}

// Run は、音声合成プロセスを一貫して実行します。
func (e *Engine) Run(ctx context.Context, outputURI, content string, opts ...contracts.RunOption) error {
	cfg := contracts.NewRunConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	segments, preCalcErrors, err := e.prepareSegments(ctx, content, cfg)
	if err != nil {
		return err
	}

	orderedAudioDataList, runtimeErrors := e.runSynthesisBatch(ctx, segments)

	return e.finalizeOutput(ctx, orderedAudioDataList, outputURI, preCalcErrors, runtimeErrors)
}

// getStyleID は話者タグから対応する Style ID を特定します（キャッシュ付き）。
func (e *Engine) getStyleID(ctx context.Context, tag string, baseSpeakerTag string, index int) (int, error) {
	e.styleIDCacheMutex.RLock()
	if id, ok := e.styleIDCache[tag]; ok {
		e.styleIDCacheMutex.RUnlock()
		return id, nil
	}
	e.styleIDCacheMutex.RUnlock()

	styleID, ok := e.data.GetStyleID(tag)
	if ok {
		e.styleIDCacheMutex.Lock()
		e.styleIDCache[tag] = styleID
		e.styleIDCacheMutex.Unlock()
		return styleID, nil
	}

	if baseSpeakerTag == "" {
		return 0, fmt.Errorf("話者タグ %s の抽出失敗 (セグメント %d)", tag, index)
	}

	fallbackKey, defaultOk := e.data.GetDefaultTag(baseSpeakerTag)
	if defaultOk {
		slog.WarnContext(ctx, "AI出力タグが未定義のためフォールバックを適用します",
			"segment_index", index,
			"original_tag", tag,
			"fallback_key", fallbackKey)

		styleID, styleOk := e.data.GetStyleID(fallbackKey)
		if styleOk {
			e.styleIDCacheMutex.Lock()
			e.styleIDCache[tag] = styleID
			e.styleIDCacheMutex.Unlock()
			return styleID, nil
		}
	}

	return 0, fmt.Errorf("話者・スタイルタグ %s に対応する ID が見つかりません (セグメント %d)", tag, index)
}

// processSegment は API に対してクエリと合成を実行します。
func (e *Engine) processSegment(ctx context.Context, seg engineSegment, index int) segmentResult {
	if seg.Err != nil {
		return segmentResult{index: index, err: seg.Err}
	}
	styleID := seg.StyleID

	queryBody, err := e.client.RunAudioQuery(ctx, seg.Text, styleID)
	if err != nil {
		return segmentResult{index: index, err: fmt.Errorf("セグメント %d のオーディオクエリ失敗: %w", index, err)}
	}

	wavData, err := e.client.RunSynthesis(ctx, queryBody, styleID)
	if err != nil {
		return segmentResult{index: index, err: fmt.Errorf("セグメント %d の音声合成失敗: %w", index, err)}
	}

	return segmentResult{index: index, wavData: wavData}
}

// prepareSegments は並列処理の前の事前準備を行います。
func (e *Engine) prepareSegments(ctx context.Context, scriptContent string, cfg *contracts.RunConfig) ([]engineSegment, []string, error) {
	parserSegments, err := e.parser.Parse(scriptContent, cfg.FallbackTag)
	if err != nil {
		return nil, nil, fmt.Errorf("スクリプトの解析に失敗しました: %w", err)
	}

	if len(parserSegments) == 0 {
		return nil, nil, fmt.Errorf("スクリプトから有効なセグメントを抽出できませんでした")
	}

	segments := make([]engineSegment, len(parserSegments))
	var preCalcErrors []string

	for i, pSeg := range parserSegments {
		seg := engineSegment{Segment: pSeg}
		styleID, err := e.getStyleID(ctx, seg.SpeakerTag, seg.BaseSpeakerTag, i)
		if err != nil {
			seg.Err = err
			preCalcErrors = append(preCalcErrors, err.Error())
		} else {
			seg.StyleID = styleID
		}
		segments[i] = seg
	}

	if len(preCalcErrors) == len(segments) {
		return nil, nil, &ErrSynthesisBatch{
			TotalErrors: len(preCalcErrors),
			Details:     preCalcErrors,
		}
	}

	return segments, preCalcErrors, nil
}

// runSynthesisBatch は音声合成タスクを並列処理します。
func (e *Engine) runSynthesisBatch(ctx context.Context, segments []engineSegment) ([][]byte, []string) {
	var runtimeErrors []string
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

		g.Go(func() error {
			if err := e.limiter.Wait(gCtx); err != nil {
				return fmt.Errorf("リミッター待機中にエラーが発生しました: %w", err)
			}

			segCtx, cancel := context.WithTimeout(gCtx, e.config.SegmentTimeout)
			defer cancel()
			results[i] = new(e.processSegment(segCtx, seg, i))

			done := atomic.AddInt32(&completed, 1)
			percentage := float64(done) / float64(total) * 100

			if done%5 == 0 || done == int32(total) {
				slog.Info("音声合成進捗",
					"progress", fmt.Sprintf("%.1f%% (%d/%d)", percentage, done, total),
					"current_segment", map[string]any{
						"index":    i,
						"style_id": seg.StyleID,
						"text":     truncateString(seg.Text, 20),
						"length":   len([]rune(seg.Text)),
					},
				)
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		slog.ErrorContext(ctx, "音声合成バッチ処理中にエラーが発生しました", "error", err)
	}

	slog.Info("全セグメントの処理が終了しました", "total", total)

	orderedAudioDataList := make([][]byte, 0, total)
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

// finalizeOutput は結果を結合して書き出します。
func (e *Engine) finalizeOutput(ctx context.Context, orderedAudioDataList [][]byte, outputWavFile string, preCalcErrors []string, runtimeErrors []string) error {
	allErrors := append([]string{}, preCalcErrors...)
	allErrors = append(allErrors, runtimeErrors...)

	if len(allErrors) > 0 {
		return &ErrSynthesisBatch{
			TotalErrors: len(allErrors),
			Details:     allErrors,
		}
	}

	finalAudioDataList := make([][]byte, 0, len(orderedAudioDataList))
	for _, data := range orderedAudioDataList {
		if data != nil {
			finalAudioDataList = append(finalAudioDataList, data)
		}
	}

	if len(finalAudioDataList) == 0 {
		return fmt.Errorf("有効な合成データが生成されませんでした")
	}

	combinedWavBytes, err := api.CombineWavData(finalAudioDataList)
	if err != nil {
		return fmt.Errorf("WAVデータの結合に失敗しました: %w", err)
	}

	reader := bytes.NewReader(combinedWavBytes)

	slog.InfoContext(ctx, "音声結合完了。出力先への書き込みを開始します。", "output_uri", outputWavFile)

	return e.writer.Write(ctx, outputWavFile, reader, "audio/wav")
}

func truncateString(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen]) + "..."
}
