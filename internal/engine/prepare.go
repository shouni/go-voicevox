package engine

import (
	"context"
	"fmt"
)

// prepareSegments は、構造化された ScriptLine を Segment に変換し、
// 事前準備(文字数超過時の強制分割・スタイルID解決)を行います。
func (e *Engine) prepareSegments(ctx context.Context, lines []ScriptLine) ([]engineSegment, []error, error) {
	if len(lines) == 0 {
		return nil, nil, fmt.Errorf("スクリプトから有効なセグメントを抽出できませんでした")
	}

	var segments []Segment
	for _, line := range lines {
		if line.Text == "" {
			continue
		}
		baseTag := "[" + line.Speaker + "]"
		speakerTag := baseTag + "[" + line.Style + "]"

		for _, chunk := range SplitByCharLimit(line.Text, MaxSegmentCharLength) {
			if chunk == "" {
				continue
			}
			segments = append(segments, Segment{
				SpeakerTag:     speakerTag,
				BaseSpeakerTag: baseTag,
				Text:           e.converter.ConvertToReading(chunk),
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
func (e *Engine) resolveStyleIDs(ctx context.Context, parserSegments []Segment) ([]engineSegment, []error, error) {
	segments := make([]engineSegment, len(parserSegments))
	var preCalcErrors []error

	for i, pSeg := range parserSegments {
		seg := engineSegment{Segment: pSeg}
		styleID, err := e.getStyleID(ctx, seg.SpeakerTag, seg.BaseSpeakerTag, i)
		if err != nil {
			seg.Err = err
			preCalcErrors = append(preCalcErrors, err)
		} else {
			seg.StyleID = styleID
		}
		segments[i] = seg
	}

	if len(preCalcErrors) == len(segments) {
		return nil, nil, newErrSynthesisBatch(preCalcErrors)
	}

	return segments, preCalcErrors, nil
}
