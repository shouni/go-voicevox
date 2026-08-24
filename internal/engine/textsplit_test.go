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

// TestSplitByCharLimitAlwaysMakesProgress は、どの入力でも必ず終わることを検証します。
//
// 旧実装は「切り出せなかったら残り全部を足して抜ける」という無限ループ避けを
// 持っていました。cutHead は上限超過時に必ず 1 文字以上を返すため、
// その保険は不要になっています。**それが本当かを押さえるテストです。**
func TestSplitByCharLimitAlwaysMakesProgress(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"。。。。。。",     // 句読点だけ
		"、",          // 上限と同じ長さの句読点
		"あああああああああ",  // 句読点なし
		"あ。あ。あ。あ。あ。", // 句読点だらけ
	}

	for _, limit := range []int{1, 2, 3} {
		for _, in := range inputs {
			chunks := splitByCharLimit(in, limit)

			var joined string
			for _, c := range chunks {
				if c == "" {
					t.Errorf("splitByCharLimit(%q, %d) が空のチャンクを返しました", in, limit)
				}
				if n := len([]rune(c)); n > limit {
					t.Errorf("splitByCharLimit(%q, %d) のチャンク %q が上限を超えています (%d 文字)", in, limit, c, n)
				}
				joined += c
			}
			// **分割しても文字は落ちません。** 落ちると音声から言葉が消えます。
			if joined != in {
				t.Errorf("splitByCharLimit(%q, %d) を連結すると %q になりました", in, limit, joined)
			}
		}
	}
}

// TestSplitByCharLimitCutsAtLastPunctuation は、上限内の**最後の**句読点で切ることを検証します。
// 最初の句読点で切ると短い断片が並び、合成の間が不自然になります。
func TestSplitByCharLimitCutsAtLastPunctuation(t *testing.T) {
	t.Parallel()

	// 上限 8 文字。「あ、い、う」の後ろの読点（6 文字目）が上限内の最後の句読点です。
	chunks := splitByCharLimit("あ、い、う、えお かきく", 8)
	if len(chunks) == 0 || chunks[0] != "あ、い、う、" {
		t.Fatalf("先頭チャンク = %q, want %q", chunks, "あ、い、う、")
	}
}
