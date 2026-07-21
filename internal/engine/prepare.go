package engine

import (
	"context"
	"fmt"

	"github.com/shouni/go-voicevox/internal/contracts"
	"github.com/shouni/go-voicevox/parser"
)

// prepareSegments は並列処理の前の事前準備を行います。
func (e *Engine) prepareSegments(ctx context.Context, scriptContent string, cfg *contracts.RunConfig) ([]engineSegment, []string, error) {
	parserSegments, err := e.parser.Parse(scriptContent, cfg.FallbackTag)
	if err != nil {
		return nil, nil, fmt.Errorf("スクリプトの解析に失敗しました: %w", err)
	}

	if len(parserSegments) == 0 {
		return nil, nil, fmt.Errorf("スクリプトから有効なセグメントを抽出できませんでした")
	}

	return e.resolveStyleIDs(ctx, parserSegments)
}

// prepareScriptSegments は、構造化された ScriptLine を Segment に変換し、
// 事前準備(文字数超過時の強制分割・スタイルID解決)を行います。
func (e *Engine) prepareScriptSegments(ctx context.Context, lines []contracts.ScriptLine) ([]engineSegment, []string, error) {
	if len(lines) == 0 {
		return nil, nil, fmt.Errorf("スクリプトから有効なセグメントを抽出できませんでした")
	}

	var segments []contracts.Segment
	for _, line := range lines {
		if line.Text == "" {
			continue
		}
		baseTag := "[" + line.Speaker + "]"
		speakerTag := baseTag + "[" + line.Style + "]"

		for _, chunk := range parser.SplitByCharLimit(line.Text, parser.MaxSegmentCharLength) {
			if chunk == "" {
				continue
			}
			segments = append(segments, contracts.Segment{
				SpeakerTag:     speakerTag,
				BaseSpeakerTag: baseTag,
				Text:           chunk,
			})
		}
	}

	if len(segments) == 0 {
		return nil, nil, fmt.Errorf("スクリプトから有効なセグメントを抽出できませんでした")
	}

	return e.resolveStyleIDs(ctx, segments)
}

// resolveStyleIDs は、各 Segment に対応するスタイルIDを解決し、engineSegment に変換します。
// 全セグメントの解決に失敗した場合はバッチエラーを返します。
func (e *Engine) resolveStyleIDs(ctx context.Context, parserSegments []contracts.Segment) ([]engineSegment, []string, error) {
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
