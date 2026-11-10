package engine

import (
	"strings"
	"testing"
)

func TestSplitByCharLimitReturnsWholeTextWhenWithinLimit(t *testing.T) {
	chunks := splitByCharLimit("短い文章です", 200)
	if len(chunks) != 1 || chunks[0] != "短い文章です" {
		t.Fatalf("chunks = %v, want single unchanged chunk", chunks)
	}
}

func TestSplitByCharLimitPrefersPunctuation(t *testing.T) {
	text := strings.Repeat("あ", 5) + "。" + strings.Repeat("い", 5)
	chunks := splitByCharLimit(text, 6)
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
	chunks := splitByCharLimit(text, maxSegmentCharLength)

	if len(chunks) != 2 {
		t.Fatalf("len(chunks) = %d, want 2: %v", len(chunks), chunks)
	}
	if got := len([]rune(chunks[0])); got != maxSegmentCharLength {
		t.Fatalf("chunks[0] rune len = %d, want %d", got, maxSegmentCharLength)
	}
	if got := len([]rune(chunks[1])); got != 10 {
		t.Fatalf("chunks[1] rune len = %d, want 10", got)
	}
}

func TestSplitByCharLimitReturnsUnchangedWhenLimitIsZero(t *testing.T) {
	chunks := splitByCharLimit("何か文章", 0)
	if len(chunks) != 1 || chunks[0] != "何か文章" {
		t.Fatalf("chunks = %v, want single unchanged chunk", chunks)
	}
}

// TestSplitByCharLimitAlwaysMakesProgress verifies that it terminates for every input.
//
// The old implementation guarded against an infinite loop with "if nothing could be cut,
// append the whole remainder and stop". cutHead always returns at least one character when
// over the limit, so that safety net is no longer needed. This test pins that it really is so.
func TestSplitByCharLimitAlwaysMakesProgress(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"。。。。。。",     // punctuation only
		"、",          // punctuation as long as the limit
		"あああああああああ",  // no punctuation
		"あ。あ。あ。あ。あ。", // punctuation everywhere
	}

	for _, limit := range []int{1, 2, 3} {
		for _, in := range inputs {
			chunks := splitByCharLimit(in, limit)

			var joined string
			for _, c := range chunks {
				if c == "" {
					t.Errorf("splitByCharLimit(%q, %d) returned an empty chunk", in, limit)
				}
				if n := len([]rune(c)); n > limit {
					t.Errorf("splitByCharLimit(%q, %d) chunk %q exceeds the limit (%d characters)", in, limit, c, n)
				}
				joined += c
			}
			// Splitting never drops characters; a dropped one would be a word missing from the audio.
			if joined != in {
				t.Errorf("splitByCharLimit(%q, %d) joined back to %q", in, limit, joined)
			}
		}
	}
}

// TestSplitByCharLimitCutsAtLastPunctuation verifies that it cuts at the last punctuation mark
// within the limit. Cutting at the first leaves a run of short fragments and unnatural pauses
// in the synthesis.
func TestSplitByCharLimitCutsAtLastPunctuation(t *testing.T) {
	t.Parallel()

	// The limit is 8 characters. The comma after 「あ、い、う」 (the 6th character) is the last
	// punctuation mark within it.
	chunks := splitByCharLimit("あ、い、う、えお かきく", 8)
	if len(chunks) == 0 || chunks[0] != "あ、い、う、" {
		t.Fatalf("first chunk = %q, want %q", chunks, "あ、い、う、")
	}
}
