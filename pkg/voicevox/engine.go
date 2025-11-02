package voicevox

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"sync"
	"time"
)

// NOTE: この正規表現は、BaseSpeakerTagの抽出にEngineのロジックとして残す
var reSpeaker = regexp.MustCompile(`^(\[.+?\])`)

// ----------------------------------------------------------------------
// エンジン構造体とコンストラクタ
// ----------------------------------------------------------------------

type Engine struct {
	client AudioQueryClient
	data   DataFinder
	parser Parser
	config EngineConfig

	// 内部キャッシュ状態
	styleIDCache      map[string]int
	styleIDCacheMutex sync.RWMutex
}

type EngineConfig struct {
	MaxParallelSegments int
	SegmentTimeout      time.Duration
}

// NewEngine は新しい Engine インスタンスを作成し、依存関係を注入します。
// 修正: NewEngine が EngineConfig を引数で受け取るように変更
func NewEngine(client AudioQueryClient, data DataFinder, parser Parser, config EngineConfig) *Engine {

	// MaxParallelSegments のデフォルト値設定
	if config.MaxParallelSegments == 0 {
		// const.go に定義されたデフォルト値を参照
		config.MaxParallelSegments = DefaultMaxParallelSegments
	}

	// SegmentTimeout のデフォルト値設定
	if config.SegmentTimeout == 0 {
		// const.go に定義されたデフォルト値を参照
		config.SegmentTimeout = DefaultSegmentTimeout
	}

	return &Engine{
		client:       client,
		data:         data,
		parser:       parser,
		config:       config,
		styleIDCache: make(map[string]int),
	}
}

// ----------------------------------------------------------------------
// ヘルパー関数 (変更なし)
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

		// デフォルトスタイルキーに対応するIDを検索 (DataFinder.GetStyleID は bool を返すように修正)
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
func (e *Engine) processSegment(ctx context.Context, seg scriptSegment, index int) segmentResult {
	// seg.Err は事前計算で処理されるため、ここでは主にネットワーク処理
	if seg.Err != nil {
		return segmentResult{index: index, err: seg.Err}
	}
	styleID := seg.StyleID

	var queryBody []byte
	var currentErr error

	// 1. runAudioQuery
	queryBody, currentErr = e.client.runAudioQuery(seg.Text, styleID, ctx)
	if currentErr != nil {
		return segmentResult{index: index, err: fmt.Errorf("セグメント %d のオーディオクエリ失敗: %w", index, currentErr)}
	}

	// 2. runSynthesis
	wavData, currentErr := e.client.runSynthesis(queryBody, styleID, ctx)
	if currentErr != nil {
		return segmentResult{index: index, err: fmt.Errorf("セグメント %d の音声合成失敗: %w", index, currentErr)}
	}

	// 3. 成功
	return segmentResult{index: index, wavData: wavData}
}

// ----------------------------------------------------------------------
// メイン処理 (Execute メソッド化)
// ----------------------------------------------------------------------

func (e *Engine) Execute(ctx context.Context, scriptContent string, outputWavFile string, fallbackTag string) error {

	// 1. スクリプト解析
	// 修正: 注入されたパーサーの Parse メソッドを呼び出す
	segments, err := e.parser.Parse(scriptContent, fallbackTag)
	if err != nil {
		return fmt.Errorf("スクリプトの解析に失敗しました: %w", err)
	}

	if len(segments) == 0 {
		return fmt.Errorf("スクリプトから有効なセグメントを抽出できませんでした。AIの出力形式を確認してください")
	}

	// 2. 速度改善ステップ: 並列処理前に全セグメントの Style ID を事前計算
	var preCalcErrors []string
	for i := range segments {
		seg := &segments[i] // ポインターでアクセス

		// 2-1. 正規表現による話者タグの抽出 (BaseSpeakerTagを設定)
		speakerMatch := reSpeaker.FindStringSubmatch(seg.SpeakerTag)
		if len(speakerMatch) >= 2 {
			seg.BaseSpeakerTag = speakerMatch[1] // 例: [ずんだもん]
		}

		// 2-2. Style IDの決定 (Engine メソッドを利用)
		styleID, err := e.getStyleID(ctx, seg.SpeakerTag, seg.BaseSpeakerTag, i)
		if err != nil {
			seg.Err = err
			preCalcErrors = append(preCalcErrors, err.Error())
		} else {
			seg.StyleID = styleID
		}
	}

	if len(preCalcErrors) == len(segments) {
		// ErrSynthesisBatch を利用
		return &ErrSynthesisBatch{
			TotalErrors: len(preCalcErrors),
			Details:     preCalcErrors,
		}
	}

	// 3. 並列処理の準備
	semaphore := make(chan struct{}, e.config.MaxParallelSegments)
	wg := sync.WaitGroup{}
	resultsChan := make(chan segmentResult, len(segments))

	slog.Info("音声合成バッチ処理開始", "total_segments", len(segments), "max_parallel", e.config.MaxParallelSegments) // ログも修正

	// 4. セグメントごとの並列処理開始
	for i, seg := range segments {
		if seg.Text == "" || seg.Err != nil {
			continue // 事前計算で失敗したセグメントはスキップ
		}

		semaphore <- struct{}{}
		wg.Add(1)

		go func(i int, seg scriptSegment) {
			defer wg.Done()
			defer func() { <-semaphore }()

			segCtx, cancel := context.WithTimeout(ctx, e.config.SegmentTimeout)
			defer cancel()

			result := e.processSegment(segCtx, seg, i)
			resultsChan <- result

		}(i, seg)
	}

	// 5. 並列処理終了後の集約
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

	// 6. 最終エラー処理
	allErrors := append([]string{}, preCalcErrors...)
	allErrors = append(allErrors, runtimeErrors...)

	if len(allErrors) > 0 {
		return &ErrSynthesisBatch{ // ⬅️ ErrSynthesisBatch を利用
			TotalErrors: len(allErrors),
			Details:     allErrors,
		}
	}

	// 7. WAVデータの結合
	finalAudioDataList := make([][]byte, 0, len(orderedAudioDataList))
	for _, data := range orderedAudioDataList {
		if data != nil {
			finalAudioDataList = append(finalAudioDataList, data)
		}
	}

	if len(finalAudioDataList) == 0 {
		return fmt.Errorf("すべてのセグメントの合成に失敗したか、有効なセグメントがありませんでした")
	}

	combinedWavBytes, err := combineWavData(finalAudioDataList) // audio.go のロジック
	if err != nil {
		return fmt.Errorf("WAVデータの結合に失敗しました: %w", err)
	}

	// 8. ファイルへの書き込み
	slog.InfoContext(ctx, "全てのセグメントの合成と結合が完了しました。ファイル書き込みを行います。", "output_file", outputWavFile)

	return os.WriteFile(outputWavFile, combinedWavBytes, 0644)
}
