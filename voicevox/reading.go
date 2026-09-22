package voicevox

import (
	"fmt"

	"github.com/shouni/audio/phonetic"

	internalengine "github.com/shouni/go-voicevox/internal/engine"
)

// ReadingPreview は、エンジンに接続せずに、合成と同じ読み変換を行います。
//
// 合成本体は本文を 200 文字の単位に分けてから読みに変換します（境界をまたぐ語は
// 別々に解析されます）。プレビューが自前で phonetic.Converter を組むと、設定の
// 写しが 2 か所になるうえ、分割を通さないので長い行で合成と結果がずれます。
// New と同じ Option を渡せる口をここに置き、変換器と分割の両方を合成と共有します。
type ReadingPreview struct {
	converter *phonetic.Converter
}

// NewReadingPreview は、New と同じ Option から読み変換だけを組み立てます。
// エンジンに関する Option（並列数やタイムアウト）は受け取りますが使いません。
// 呼び出し側は 1 つの []Option を New とここの両方へ渡してください。
func NewReadingPreview(opts ...Option) (*ReadingPreview, error) {
	o := newOptions(opts...)
	converter, err := phonetic.NewConverter(o.converter...)
	if err != nil {
		return nil, fmt.Errorf("読み変換コンバータの初期化に失敗しました: %w", err)
	}
	return &ReadingPreview{converter: converter}, nil
}

// Read は、text を合成と同じ単位に分け、それぞれを読み（カタカナ）に変換して返します。
// 返り値の要素数は合成が送るセグメント数と一致し、各要素はそのセグメントの読みです。
func (p *ReadingPreview) Read(text string) []string {
	chunks := internalengine.SplitForSynthesis(text)
	readings := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk == "" {
			continue
		}
		readings = append(readings, p.converter.ConvertToReading(chunk))
	}
	return readings
}
