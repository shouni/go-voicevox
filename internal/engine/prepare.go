package engine

import (
	"context"
	"fmt"

	"github.com/shouni/go-voicevox/internal/contracts"
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
