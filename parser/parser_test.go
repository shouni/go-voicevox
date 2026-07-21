package parser

import (
	"strings"
	"testing"
)

func TestParseAppendsUntaggedLineToCurrentSegment(t *testing.T) {
	p := NewParser()

	segments, err := p.Parse("[ずんだもん][ノーマル] こんにちは\n追記です", "[めたん][ノーマル]")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(segments) != 1 {
		t.Fatalf("len(segments) = %d, want 1", len(segments))
	}
	if segments[0].SpeakerTag != "[ずんだもん][ノーマル]" {
		t.Fatalf("SpeakerTag = %q", segments[0].SpeakerTag)
	}
	if segments[0].BaseSpeakerTag != "[ずんだもん]" {
		t.Fatalf("BaseSpeakerTag = %q", segments[0].BaseSpeakerTag)
	}
	if segments[0].Text != "こんにちは 追記です" {
		t.Fatalf("Text = %q, want %q", segments[0].Text, "こんにちは 追記です")
	}
}

func TestParseKeepsTextUnchangedByDefault(t *testing.T) {
	p := NewParser()

	segments, err := p.Parse("[ずんだもん][ノーマル] 私は閃光", "[めたん][ノーマル]")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(segments) != 1 {
		t.Fatalf("len(segments) = %d, want 1", len(segments))
	}
	if segments[0].Text != "私は閃光" {
		t.Fatalf("Text = %q, want %q", segments[0].Text, "私は閃光")
	}
}

func TestParseAppliesTextPreprocessor(t *testing.T) {
	p := NewParser(WithTextPreprocessor(func(text string) string {
		return "processed:" + text
	}))

	segments, err := p.Parse("[ずんだもん][ノーマル] こんにちは", "[めたん][ノーマル]")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(segments) != 1 {
		t.Fatalf("len(segments) = %d, want 1", len(segments))
	}
	if segments[0].Text != "processed:こんにちは" {
		t.Fatalf("Text = %q, want %q", segments[0].Text, "processed:こんにちは")
	}
}

func TestParseAppliesPhoneticPreprocessor(t *testing.T) {
	p, err := NewPhoneticParser()
	if err != nil {
		t.Fatalf("NewPhoneticParser() error = %v", err)
	}

	segments, err := p.Parse("[ずんだもん][ノーマル] 私は閃光", "[めたん][ノーマル]")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(segments) != 1 {
		t.Fatalf("len(segments) = %d, want 1", len(segments))
	}
	if segments[0].Text != "ワタシワヒカリ" {
		t.Fatalf("Text = %q, want %q", segments[0].Text, "ワタシワヒカリ")
	}
}

func TestParseUsesFallbackForUntaggedScript(t *testing.T) {
	p := NewParser()

	segments, err := p.Parse("タグなし本文だけです", "[めたん][ノーマル]")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(segments) != 1 {
		t.Fatalf("len(segments) = %d, want 1", len(segments))
	}
	if segments[0].SpeakerTag != "[めたん][ノーマル]" {
		t.Fatalf("SpeakerTag = %q", segments[0].SpeakerTag)
	}
	if segments[0].BaseSpeakerTag != "[めたん]" {
		t.Fatalf("BaseSpeakerTag = %q", segments[0].BaseSpeakerTag)
	}
}

func TestParseSplitsLongTextWithinLimit(t *testing.T) {
	p := NewParser()
	longText := strings.Repeat("あ", 210)

	segments, err := p.Parse("[ずんだもん][ノーマル] "+longText, "[めたん][ノーマル]")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(segments) != 2 {
		t.Fatalf("len(segments) = %d, want 2", len(segments))
	}
	if got := len([]rune(segments[0].Text)); got != MaxSegmentCharLength {
		t.Fatalf("first segment rune len = %d, want %d", got, MaxSegmentCharLength)
	}
	if got := len([]rune(segments[1].Text)); got != 10 {
		t.Fatalf("second segment rune len = %d, want 10", got)
	}
}

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
