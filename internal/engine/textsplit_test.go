package engine

import (
	"strings"
	"testing"
)

func TestSplitByCharLimitReturnsWholeTextWhenWithinLimit(t *testing.T) {
	chunks := SplitByCharLimit("短い文章です", 200)
	if len(chunks) != 1 || chunks[0] != "短い文章です" {
		t.Fatalf("chunks = %v, want single unchanged chunk", chunks)
	}
}

func TestSplitByCharLimitPrefersPunctuation(t *testing.T) {
	text := strings.Repeat("あ", 5) + "。" + strings.Repeat("い", 5)
	chunks := SplitByCharLimit(text, 6)
	if len(chunks) != 2 {
		t.Fatalf("len(chunks) = %d, want 2: %v", len(chunks), chunks)
	}
	if chunks[0] != strings.Repeat("あ", 5)+"。" {
		t.Fatalf("chunks[0] = %q, want split at punctuation", chunks[0])
	}
	if chunks[1] != strings.Repeat("い", 5) {
		t.Fatalf("chunks[1] = %q", chunks[1])
	}
}

func TestSplitByCharLimitForceSplitsWithoutPunctuation(t *testing.T) {
	text := strings.Repeat("あ", 210)
	chunks := SplitByCharLimit(text, MaxSegmentCharLength)

	if len(chunks) != 2 {
		t.Fatalf("len(chunks) = %d, want 2: %v", len(chunks), chunks)
	}
	if got := len([]rune(chunks[0])); got != MaxSegmentCharLength {
		t.Fatalf("chunks[0] rune len = %d, want %d", got, MaxSegmentCharLength)
	}
	if got := len([]rune(chunks[1])); got != 10 {
		t.Fatalf("chunks[1] rune len = %d, want 10", got)
	}
}

func TestSplitByCharLimitReturnsUnchangedWhenLimitIsZero(t *testing.T) {
	chunks := SplitByCharLimit("何か文章", 0)
	if len(chunks) != 1 || chunks[0] != "何か文章" {
		t.Fatalf("chunks = %v, want single unchanged chunk", chunks)
	}
}
