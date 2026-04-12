package voicevox

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/shouni/go-remote-io/remoteio"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"github.com/shouni/go-voicevox/api"
	"github.com/shouni/go-voicevox/parser"
)

const (
	defaultVoicevoxAPIURL      = "http://localhost:50021"
	DefaultMaxParallelSegments = 8
	DefaultSegmentTimeout      = 300 * time.Second
	DefaultSegmentRateLimit    = 1000 * time.Millisecond
)

// DataFinder は、Engine が Style ID を検索するために SpeakerData に要求するメソッドを定義します。
type DataFinder interface {
	GetStyleID(combinedTag string) (int, bool)
	GetDefaultTag(speakerToolTag string) (string, bool)
}

// AudioQueryClient は Client が満たすべき API 呼び出しインターフェース
type AudioQueryClient interface {
	RunAudioQuery(ctx context.Context, text string, styleID int) ([]byte, error)
	RunSynthesis(ctx context.Context, queryBody []byte, styleID int) ([]byte, error)
}

// Engine は VOICEVOX エンジンを利用した音声合成のメインコントローラーです。
type Engine struct {
	client            AudioQueryClient
	data              DataFinder
	parser            parser.Parser
	limiter           *rate.Limiter
	opts              options // 適用されたオプションを保持
	writer            remoteio.Writer
	styleIDCache      map[string]int
	styleIDCacheMutex sync.RWMutex
}

// engineSegment は内部処理用のセグメント構造体です。
type engineSegment struct {
	parser.Segment
	StyleID int
	Err     error
}

// segmentResult は合成処理の結果を保持します。
type segmentResult struct {
	index   int
	wavData []byte
	err     error
}

// NewEngine は、指定された依存関係と設定を使用して新しい Engine インスタンスを作成します。
func NewEngine(client AudioQueryClient, data DataFinder, p parser.Parser, writer remoteio.Writer, opts ...Option) *Engine {
	// 1. まずデフォルト値を設定
	appliedOpts := options{
		MaxParallelSegments: DefaultMaxParallelSegments,
		SegmentTimeout:      DefaultSegmentTimeout,
		SegmentRateLimit:    DefaultSegmentRateLimit,
	}

	// 2. 渡されたオプションを順番に適用して上書き
	for _, opt := range opts {
		opt(&appliedOpts)
	}

	// 3. エンジンの初期化
	engine := &Engine{
		client:       client,
		data:         data,
		parser:       p,
		writer:       writer,
		opts:         appliedOpts,
		styleIDCache: make(map[string]int),
	}

	// 4. 指定された間隔でリクエストを制限するリミッターを作成
	engine.limiter = rate.NewLimiter(rate.Every(appliedOpts.SegmentRateLimit), 1)

	return engine
}

// Execute は音声合成プロセスを一貫して実行します。
func (e *Engine) Execute(ctx context.Context, scriptContent string, outputWavFile string, opts ...ExecuteOption) error {
	cfg := newExecuteConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	segments, preCalcErrors, err := e.prepareSegments(ctx, scriptContent, cfg)
	if err != nil {
		return err
	}

	orderedAudioDataList, runtimeErrors := e.runSynthesisBatch(ctx, segments)

	return e.finalizeOutput(ctx, orderedAudioDataList, outputWavFile, preCalcErrors, runtimeErrors)
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
func (e *Engine) prepareSegments(ctx context.Context, scriptContent string, cfg *ExecuteConfig) ([]engineSegment, []string, error) {
	parserSegments, err := e.parser.Parse(scriptContent, cfg.FallbackTag)
	if err != nil {
		return nil, nil, fmt.Errorf("スクリプトの解析に失敗しました: %w", err)
	}

	if len(parserSegments) == 0 {
		return nil, nil, fmt.Errorf("スクリプトから有効なセグメントを抽出できませんでした")
	}

	segments := make([]engineSegment, len(parserSegments))
	for i, pSeg := range parserSegments {
		segments[i] = engineSegment{Segment: pSeg}
	}

	var preCalcErrors []string
	for i := range segments {
		seg := &segments[i]
		styleID, err := e.getStyleID(ctx, seg.SpeakerTag, seg.BaseSpeakerTag, i)
		if err != nil {
			seg.Err = err
			preCalcErrors = append(preCalcErrors, err.Error())
		} else {
			seg.StyleID = styleID
		}
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
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(e.opts.MaxParallelSegments)

	results := make([]segmentResult, len(segments))
	var runtimeErrors []string

	slog.Info("音声合成バッチ処理開始", "total_segments", len(segments), "max_parallel", e.opts.MaxParallelSegments)

	for i, seg := range segments {
		if seg.Text == "" || seg.Err != nil {
			continue
		}
		g.Go(func() error {
			// リミッターの待機エラー（キャンセルなど）を拾う
			if err := e.limiter.Wait(gCtx); err != nil {
				return fmt.Errorf("リミッター待機中にエラーが発生しました: %w", err)
			}

			segCtx, cancel := context.WithTimeout(gCtx, e.opts.SegmentTimeout)
			defer cancel()

			results[i] = e.processSegment(segCtx, seg, i)
			return nil
		})
	}

	// バッチ全体の終了を待機し、エラーがあれば集約する
	if err := g.Wait(); err != nil {
		slog.ErrorContext(ctx, "音声合成バッチ処理中にエラーが発生しました", "error", err)
		runtimeErrors = append(runtimeErrors, fmt.Sprintf("バッチ処理エラー: %v", err))
	}

	orderedAudioDataList := make([][]byte, len(segments))
	for _, res := range results {
		if res.err != nil {
			runtimeErrors = append(runtimeErrors, res.err.Error())
		} else if res.wavData != nil {
			orderedAudioDataList[res.index] = res.wavData
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

	if remoteio.IsGCSURI(outputWavFile) {
		slog.InfoContext(ctx, "音声結合完了。GCS へのアップロードを開始します。", "gcs_uri", outputWavFile)
	} else {
		slog.InfoContext(ctx, "音声結合完了。ローカルファイルへの書き込みを開始します。", "output_file", outputWavFile)
	}

	return e.writer.Write(ctx, outputWavFile, reader, "audio/wav")
}
