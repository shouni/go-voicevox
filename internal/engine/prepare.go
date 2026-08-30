package engine

import (
	"context"
	"errors"
)

// errNoSegments は、合成するものが 1 つも無いことを示します。
var errNoSegments = errors.New("スクリプトから有効なセグメントを抽出できませんでした")

// segment は、合成 1 回分の単位です。
//
// 内部専用です。タグは prepareSegments が角括弧付きで組み立て、スタイル ID も
// その場で解決します。呼び出し側がこれを作る場面はありません。入口は ScriptLine です。
//
// 以前はタグと本文だけを持つ Segment と、そこへ StyleID / Err を足した
// engineSegment に分かれていました。前者は後者へ変換されるためだけに存在し、
// 単独で渡る先がありませんでした。
type segment struct {
	SpeakerTag     string
	BaseSpeakerTag string
	Text           string
	StyleID        int
	// Err は、スタイル ID の解決に失敗した理由です。埋まっているセグメントは
	// API へ投げず、失敗として集計されます。
	Err error
}

// prepareSegments は、構造化された ScriptLine を合成単位へ展開します。
// 文字数超過の行は分割し、各セグメントのスタイルIDをその場で解決します。
//
// 返す preCalcErrors は解決に失敗した分です。全件失敗した場合だけエラーを返し、
// ネットワークへ出る前に止めます。
func (e *Engine) prepareSegments(ctx context.Context, lines []ScriptLine) ([]segment, []error, error) {
	if len(lines) == 0 {
		return nil, nil, errNoSegments
	}

	var (
		segments      []segment
		preCalcErrors []error
	)
	resolver := newStyleResolver(e.styles)
	for _, line := range lines {
		if line.Text == "" {
			continue
		}
		baseTag := "[" + line.Speaker + "]"
		speakerTag := baseTag + "[" + line.Style + "]"

		for _, chunk := range splitByCharLimit(line.Text, maxSegmentCharLength) {
			if chunk == "" {
				continue
			}
			seg := segment{
				SpeakerTag:     speakerTag,
				BaseSpeakerTag: baseTag,
				Text:           e.converter.ConvertToReading(chunk),
			}

			styleID, err := resolver.resolve(ctx, seg.SpeakerTag, seg.BaseSpeakerTag, len(segments))
			if err != nil {
				seg.Err = err
				preCalcErrors = append(preCalcErrors, err)
			} else {
				seg.StyleID = styleID
			}

			segments = append(segments, seg)
		}
	}

	if len(segments) == 0 {
		return nil, nil, errNoSegments
	}
	if len(preCalcErrors) == len(segments) {
		return nil, nil, newErrSynthesisBatch(preCalcErrors)
	}

	return segments, preCalcErrors, nil
}
