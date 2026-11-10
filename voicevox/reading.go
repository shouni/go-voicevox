package voicevox

import (
	"fmt"

	"github.com/shouni/audio/phonetic"

	internalengine "github.com/shouni/go-voicevox/internal/engine"
)

// ReadingPreview performs the same reading conversion as synthesis without connecting to the
// engine.
//
// Synthesis splits text into 200-character units before converting it to a reading (a word
// across a boundary is analysed in two parts). A preview that built its own
// phonetic.Converter would keep a second copy of the settings and, skipping the split, would
// differ from synthesis on long lines. This entry point takes the same Options as New and
// shares both the converter and the split with synthesis.
type ReadingPreview struct {
	converter *phonetic.Converter
}

// NewReadingPreview builds only the reading conversion from the same Options as New.
// Engine Options (parallelism, timeouts) are accepted but unused.
// Callers should pass one []Option to both New and this function.
func NewReadingPreview(opts ...Option) (*ReadingPreview, error) {
	o := newOptions(opts...)
	converter, err := phonetic.NewConverter(o.converter...)
	if err != nil {
		return nil, fmt.Errorf("initialize reading converter: %w", err)
	}
	return &ReadingPreview{converter: converter}, nil
}

// Read splits text into the same units as synthesis and returns each converted to a reading
// (katakana). The number of elements matches the number of segments synthesis sends, and each
// element is that segment's reading.
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
