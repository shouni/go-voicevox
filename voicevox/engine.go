package voicevox

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/shouni/go-remote-io/remoteio"
	"github.com/shouni/go-voicevox/voicevox/audio"
	"github.com/shouni/go-voicevox/voicevox/parser"
	"github.com/shouni/go-voicevox/voicevox/speaker"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

type Engine struct {
	client            AudioQueryClient
	data              DataFinder
	parser            parser.Parser
	limiter           *rate.Limiter
	config            EngineConfig
	writer            remoteio.Writer
	styleIDCache      map[string]int
	styleIDCacheMutex sync.RWMutex
}

type EngineConfig struct {
	MaxParallelSegments int
	SegmentTimeout      time.Duration
	SegmentRateLimit    time.Duration
}

// --- 内部データ構造と定数 ---

// engineSegment は parser.Segment に Engine 処理に必要なフィールドを追加した内部構造体です。
type engineSegment struct {
	parser.Segment
	StyleID int
	Err     error
}

// segmentResult は Goルーチンからの結果を格納するための内部構造体です。
type segmentResult struct {
	index   int
	wavData []byte
	err     error
}

// ----------------------------------------------------------------------
// Executeメソッド用のオプション定義 (Functional Options Pattern)
// ----------------------------------------------------------------------

// ExecuteConfig は Execute メソッドの実行中に適用されるオプション設定を保持する
type ExecuteConfig struct {
	FallbackTag string
}

// ExecuteOption はオプションを適用するための関数シグネチャ
type ExecuteOption func(*ExecuteConfig)

// newExecuteConfig は Execute のデフォルト設定を初期化する
func newExecuteConfig() *ExecuteConfig {
	return &ExecuteConfig{
		FallbackTag: speaker.VvTagNormal,
	}
}

// WithFallbackTag は、ユーザーがカスタムの FallbackTag を指定するためのオプション
func WithFallbackTag(tag string) ExecuteOption {
	return func(cfg *ExecuteConfig) {
		if tag != "" {
			cfg.FallbackTag = tag
		}
	}
}

// NewEngine は新しい Engine インスタンスを作成し、依存関係を注入します。
// writer: Go Remote IO ファクトリから取得された Writer を注入します。
func NewEngine(client AudioQueryClient, data DataFinder, p parser.Parser, config EngineConfig, writer remoteio.Writer) *Engine {

	// NOTE: Default 定数が未定義のため、仮の値を設定
	if config.MaxParallelSegments == 0 {
		config.MaxParallelSegments = DefaultMaxParallelSegments
	}
	if config.SegmentTimeout == 0 {
		config.SegmentTimeout = DefaultSegmentTimeout
	}
	if config.SegmentRateLimit == 0 {
		config.SegmentRateLimit = DefaultSegmentRateLimit
	}

	// rate.Every を使用して、指定された間隔でトークンを生成するリミッターを作成
	limiter := rate.NewLimiter(rate.Every(config.SegmentRateLimit), 1)

	return &Engine{
		client:       client,
		data:         data,
		parser:       p,
		config:       config,
		writer:       writer,
		styleIDCache: make(map[string]int),
		limiter:      limiter,
	}
}

// ----------------------------------------------------------------------
// ヘルパー関数 (省略)
// ----------------------------------------------------------------------

// getStyleID はセグメントの話者タグから対応するStyle IDを検索し、キャッシュを使用/更新します。
func (e *Engine) getStyleID(ctx context.Context, tag string, baseSpeakerTag string, index int) (int, error) {
	// 1. 内部キャッシュのチェック (読み取り操作)
	e.styleIDCacheMutex.RLock()
	if id, ok := e.styleIDCache[tag]; ok {
		e.styleIDCacheMutex.RUnlock()
		return id, nil
	}
	e.styleIDCacheMutex.RUnlock()

	// 2. 完全なタグでの検索 (キャッシュミスの場合)
	styleID, ok := e.data.GetStyleID(tag)
	if ok {
		// キャッシュに保存 (書き込み操作)
		e.styleIDCacheMutex.Lock()
		e.styleIDCache[tag] = styleID
		e.styleIDCacheMutex.Unlock()
		return styleID, nil
	}

	// 3. フォールバック処理: デフォルトスタイルを試す
	if baseSpeakerTag == "" {
		return 0, fmt.Errorf("話者タグ %s の抽出失敗 (セグメント %d)", tag, index)
	}

	fallbackKey, defaultOk := e.data.GetDefaultTag(baseSpeakerTag)

	if defaultOk {
		slog.WarnContext(ctx, "AI出力タグが未定義のためフォールバック",
			"segment_index", index,
			"original_tag", tag,
			"fallback_key", fallbackKey)

		// デフォルトスタイルキーに対応するIDを検索
		styleID, styleOk := e.data.GetStyleID(fallbackKey)
		if styleOk {
			// フォールバック成功の場合もキャッシュに保存 (書き込み操作)
			e.styleIDCacheMutex.Lock()
			e.styleIDCache[tag] = styleID // 元のタグに対してデフォルトのIDを保存
			e.styleIDCacheMutex.Unlock()
			return styleID, nil
		}
	}

	return 0, fmt.Errorf("話者・スタイルタグ %s (およびデフォルトスタイル) に対応するStyle IDが見つかりません (セグメント %d)", tag, index)
}

// processSegment は単一のセグメントに対してAPI呼び出しを実行します。
func (e *Engine) processSegment(ctx context.Context, seg engineSegment, index int) segmentResult {
	// seg.Err は事前計算で処理されるため、ここでは主にネットワーク処理
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

// ----------------------------------------------------------------------
// メイン処理 (Execute メソッド)
// ----------------------------------------------------------------------

func (e *Engine) Execute(ctx context.Context, scriptContent string, outputWavFile string, opts ...ExecuteOption) error {
	// 1. 設定初期化と適用
	cfg := newExecuteConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// 2. スクリプト解析とセグメントの事前準備
	segments, preCalcErrors, err := e.prepareSegments(ctx, scriptContent, cfg)
	if err != nil {
		// fatal error (e.g., parsing failed, or all segments failed pre-calc)
		return err
	}

	// 3. 音声合成バッチ処理の実行
	orderedAudioDataList, runtimeErrors := e.runSynthesisBatch(ctx, segments)

	// 4. 結果の集約とファイルへの書き込み
	return e.finalizeOutput(ctx, orderedAudioDataList, outputWavFile, preCalcErrors, runtimeErrors)
}

// prepareSegments はスクリプトを解析し、Style IDを決定するなど、並列処理の前のすべての準備を行います。
func (e *Engine) prepareSegments(ctx context.Context, scriptContent string, cfg *ExecuteConfig) ([]engineSegment, []string, error) {
	// スクリプト解析
	parserSegments, err := e.parser.Parse(scriptContent, cfg.FallbackTag)
	if err != nil {
		return nil, nil, fmt.Errorf("スクリプトの解析に失敗しました: %w", err)
	}

	if len(parserSegments) == 0 {
		return nil, nil, fmt.Errorf("スクリプトから有効なセグメントを抽出できませんでした。AIの出力形式を確認してください")
	}

	// Engine内部構造体への変換と事前計算
	segments := make([]engineSegment, len(parserSegments))
	for i, pSeg := range parserSegments {
		segments[i] = engineSegment{Segment: pSeg}
	}

	var preCalcErrors []string
	for i := range segments {
		seg := &segments[i] // ポインターでアクセス

		// Style IDの決定
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

// runSynthesisBatch は、テキスト音声合成タスクのバッチを並列処理します。
// 合成された音声データと、処理中に発生したランタイムエラーを返します。
func (e *Engine) runSynthesisBatch(ctx context.Context, segments []engineSegment) ([][]byte, []string) {
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(e.config.MaxParallelSegments)

	results := make([]segmentResult, len(segments))

	slog.Info("音声合成バッチ処理開始", "total_segments", len(segments), "max_parallel", e.config.MaxParallelSegments)

	for i, seg := range segments {
		if seg.Text == "" || seg.Err != nil {
			continue
		}

		g.Go(func() error {
			if err := e.limiter.Wait(gCtx); err != nil {
				return err
			}

			segCtx, cancel := context.WithTimeout(gCtx, e.config.SegmentTimeout)
			defer cancel()

			results[i] = e.processSegment(segCtx, seg, i)
			return nil
		})
	}

	err := g.Wait()
	if err != nil {
		slog.Error("バッチ処理中にエラーが発生しました", "error", err)
	}

	orderedAudioDataList := make([][]byte, len(segments))
	var runtimeErrors []string

	for _, res := range results {
		if res.err != nil {
			runtimeErrors = append(runtimeErrors, res.err.Error())
		} else if res.wavData != nil {
			orderedAudioDataList[res.index] = res.wavData
		}
	}

	return orderedAudioDataList, runtimeErrors
}

// finalizeOutput はバッチ結果を集約し、WAVデータを結合し、ファイルに書き出します。
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
		return fmt.Errorf("すべてのセグメントの合成に失敗したか、有効なセグメントがありませんでした")
	}

	combinedWavBytes, err := audio.CombineWavData(finalAudioDataList)
	if err != nil {
		return fmt.Errorf("WAVデータの結合に失敗しました: %w", err)
	}

	// combinedWavBytes ([]byte) を io.Reader に変換
	reader := bytes.NewReader(combinedWavBytes)

	// 10. ファイルへの書き込み

	// 書き込み先の種類に応じてログメッセージを出力
	if remoteio.IsGCSURI(outputWavFile) {
		slog.InfoContext(ctx, "全てのセグメントの合成と結合が完了しました。GCSオブジェクトへのアップロードを行います。", "gcs_uri", outputWavFile)
	} else {
		slog.InfoContext(ctx, "全てのセグメントの合成と結合が完了しました。ローカルファイルへの書き込みを行います。", "output_file", outputWavFile)
	}

	return e.writer.Write(ctx, outputWavFile, reader, "audio/wav")
}
