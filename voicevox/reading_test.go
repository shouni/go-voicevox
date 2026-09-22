package voicevox

import (
	"strings"
	"testing"
)

// エンジンに接続せずに、New と同じ Option で読みが得られること。
func TestReadingPreviewUsesTheSameOptionsAsNew(t *testing.T) {
	t.Parallel()

	plain, err := NewReadingPreview()
	if err != nil {
		t.Fatalf("NewReadingPreview() error = %v", err)
	}
	numbered, err := NewReadingPreview(WithNumberReading(), WithMaxParallelSegments(3))
	if err != nil {
		t.Fatalf("NewReadingPreview(WithNumberReading) error = %v", err)
	}

	if got := plain.Read("3本"); len(got) != 1 || strings.Contains(got[0], "サンボン") {
		t.Errorf("without WithNumberReading: Read() = %v, want the dictionary reading", got)
	}
	if got := numbered.Read("3本"); len(got) != 1 || got[0] != "サンボン" {
		t.Errorf("with WithNumberReading: Read() = %v, want [サンボン]", got)
	}
}

// 長い行は合成と同じ切れ目で分けてから変換されること。分割を通さないと、
// 境界をまたぐ語の読みが合成とずれる。
func TestReadingPreviewSplitsLikeSynthesis(t *testing.T) {
	t.Parallel()

	p, err := NewReadingPreview()
	if err != nil {
		t.Fatalf("NewReadingPreview() error = %v", err)
	}
	long := strings.Repeat("これは長い文です。", 40) // 200 文字を超える

	got := p.Read(long)
	if len(got) < 2 {
		t.Fatalf("Read() returned %d segment(s), want the line split like synthesis", len(got))
	}
	for i, r := range got {
		if r == "" {
			t.Errorf("segment %d is empty", i)
		}
	}
}
