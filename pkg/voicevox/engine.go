package voicevox

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/shouni/go-voicevox/pkg/voicevox/audio"
	"github.com/shouni/go-voicevox/pkg/voicevox/parser"
	"github.com/shouni/go-voicevox/pkg/voicevox/speaker"
	"golang.org/x/time/rate"
)

type Engine struct {
	client  AudioQueryClient
	data    DataFinder
	parser  parser.Parser
	limiter *rate.Limiter
	config  EngineConfig

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
func NewEngine(client AudioQueryClient, data DataFinder, p parser.Parser, config EngineConfig) *Engine {

	// NOTE: Default 定数が未定義のため、仮の値を設定
	if config.MaxParallelSegments == 0 {
		config.MaxParallelSegments = 4
	}
	if config.SegmentTimeout == 0 {
		config.SegmentTimeout = 30 * time.Second
	}
	if config.SegmentRateLimit == 0 {
		config.SegmentRateLimit = 100 * time.Millisecond
	}

	// rate.Every を使用して、指定された間隔でトークンを生成するリミッターを作成
	limiter := rate.NewLimiter(rate.Every(config.SegmentRateLimit), 1)

	return &Engine{
		client:       client,
		data:         data,
		parser:       p,
		config:       config,
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

	var queryBody []byte
	var currentErr error

	// 1. RunAudioQuery (インターフェースのメソッド名に合わせる)
	queryBody, currentErr = e.client.RunAudioQuery(seg.Text, styleID, ctx)
	if currentErr != nil {
		return segmentResult{index: index, err: fmt.Errorf("セグメント %d のオーディオクエリ失敗: %w", index, currentErr)}
	}

	// 2. RunSynthesis (インターフェースのメソッド名に合わせる)
	wavData, currentErr := e.client.RunSynthesis(queryBody, styleID, ctx)
	if currentErr != nil {
		return segmentResult{index: index, err: fmt.Errorf("セグメント %d の音声合成失敗: %w", index, currentErr)}
	}

	// 3. 成功
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

	// 3. 音声合成バッチ処理の実行 (ステップ 5 & 6 を抽出)
	orderedAudioDataList, runtimeErrors := e.runSynthesisBatch(ctx, segments)

	// 4. 結果の集約とファイルへの書き込み (ステップ 7, 8, 9, 10 を抽出)
	return e.finalizeOutput(ctx, segments, orderedAudioDataList, preCalcErrors, runtimeErrors, outputWavFile)
}

// prepareSegments はスクリプトを解析し、Style IDを決定するなど、並列処理の前のすべての準備を行います。
func (e *Engine) prepareSegments(ctx context.Context, scriptContent string, cfg *ExecuteConfig) ([]engineSegment, []string, error) {
	// 3. スクリプト解析
	parserSegments, err := e.parser.Parse(scriptContent, cfg.FallbackTag)
	if err != nil {
		return nil, nil, fmt.Errorf("スクリプトの解析に失敗しました: %w", err)
	}

	if len(parserSegments) == 0 {
		return nil, nil, fmt.Errorf("スクリプトから有効なセグメントを抽出できませんでした。AIの出力形式を確認してください")
	}

	// 4. Engine内部構造体への変換と事前計算
	segments := make([]engineSegment, len(parserSegments))
	for i, pSeg := range parserSegments {
		segments[i] = engineSegment{Segment: pSeg}
	}

	var preCalcErrors []string
	for i := range segments {
		seg := &segments[i] // ポインターでアクセス

		// 4-2. Style IDの決定
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

// runSynthesisBatch はセグメントの並列処理（レートリミットとセマフォ制御）を実行します。
// 結果をインデックス順に格納するためのリストと、ランタイムエラーのリストを返します。
func (e *Engine) runSynthesisBatch(ctx context.Context, segments []engineSegment) ([][]byte, []string) {
	// 5. 並列処理の準備
	semaphore := make(chan struct{}, e.config.MaxParallelSegments)
	wg := sync.WaitGroup{}
	resultsChan := make(chan segmentResult, len(segments))

	// ループを中断するためのフラグ
	shouldBreak := false

	slog.Info("音声合成バッチ処理開始", "total_segments", len(segments), "max_parallel", e.config.MaxParallelSegments)

	// 6. セグメントごとの並列処理開始
	for i, seg := range segments {
		if seg.Text == "" || seg.Err != nil {
			continue
		}

		// レートリミット待機
		if err := e.limiter.Wait(ctx); err != nil {
			slog.InfoContext(ctx, "バッチ処理ループが外部コンテキストキャンセルにより終了しました。(レートリミット待機中)", "error", err)
			shouldBreak = true
		}

		if shouldBreak {
			break
		}

		// セマフォの確保。コンテキストキャンセルをチェック
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "バッチ処理ループが外部コンテキストキャンセルにより終了しました。(セマフォ確保前)")
			shouldBreak = true
		case semaphore <- struct{}{}:
			// セマフォ確保成功
		}

		if shouldBreak {
			break
		}

		wg.Add(1)

		go func(i int, seg engineSegment) {
			defer wg.Done()
			defer func() { <-semaphore }()

			segCtx, cancel := context.WithTimeout(ctx, e.config.SegmentTimeout)
			defer cancel()

			result := e.processSegment(segCtx, seg, i)
			resultsChan <- result

		}(i, seg)
	}

	// 7. 並列処理終了後の集約準備
	wg.Wait()
	close(resultsChan)

	orderedAudioDataList := make([][]byte, len(segments))
	var runtimeErrors []string

	for res := range resultsChan {
		if res.err != nil {
			runtimeErrors = append(runtimeErrors, res.err.Error())
		} else if res.wavData != nil {
			orderedAudioDataList[res.index] = res.wavData
		}
	}

	return orderedAudioDataList, runtimeErrors
}

// finalizeOutput はバッチ結果を集約し、WAVデータを結合し、ファイルに書き出します。
func (e *Engine) finalizeOutput(ctx context.Context, segments []engineSegment, orderedAudioDataList [][]byte, preCalcErrors []string, runtimeErrors []string, outputWavFile string) error {
	// 8. 最終エラー処理
	allErrors := append([]string{}, preCalcErrors...)
	allErrors = append(allErrors, runtimeErrors...)

	if len(allErrors) > 0 {
		return &ErrSynthesisBatch{
			TotalErrors: len(allErrors),
			Details:     allErrors,
		}
	}

	// 9. WAVデータの結合
	finalAudioDataList := make([][]byte, 0, len(orderedAudioDataList))
	for _, data := range orderedAudioDataList {
		if data != nil {
			finalAudioDataList = append(finalAudioDataList, data)
		}
	}

	if len(finalAudioDataList) == 0 {
		// このエラーは、allErrorsのチェックで既に除外されているはずだが、念のため。
		return fmt.Errorf("すべてのセグメントの合成に失敗したか、有効なセグメントがありませんでした")
	}

	combinedWavBytes, err := audio.CombineWavData(finalAudioDataList)
	if err != nil {
		return fmt.Errorf("WAVデータの結合に失敗しました: %w", err)
	}

	// 10. ファイルへの書き込み
	slog.InfoContext(ctx, "全てのセグメントの合成と結合が完了しました。ファイル書き込みを行います。", "output_file", outputWavFile)

	dir := filepath.Dir(outputWavFile)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("出力ディレクトリの作成に失敗しました (%s): %w", dir, err)
		}
	}

	return os.WriteFile(outputWavFile, combinedWavBytes, 0644)
}
