package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shouni/go-voicevox/internal/contracts"
	"github.com/shouni/go-voicevox/parser"
)

type stubParser struct {
	segments []contracts.Segment
	err      error
}

func (s stubParser) Parse(scriptContent string, fallbackTag string) ([]contracts.Segment, error) {
	return s.segments, s.err
}

type stubFinder struct {
	styleIDs    map[string]int
	defaultTags map[string]string
}

func (s stubFinder) GetStyleID(tag string) (int, bool) {
	id, ok := s.styleIDs[tag]
	return id, ok
}

func (s stubFinder) GetDefaultTag(baseSpeakerTag string) (string, bool) {
	tag, ok := s.defaultTags[baseSpeakerTag]
	return tag, ok
}

type stubClient struct{}

func (stubClient) RunAudioQuery(ctx context.Context, text string, styleID int) ([]byte, error) {
	return []byte("query"), nil
}

func (stubClient) RunSynthesis(ctx context.Context, queryBody []byte, styleID int) ([]byte, error) {
	return []byte("wav"), nil
}

func TestNewAppliesOptions(t *testing.T) {
	e := New(
		stubClient{},
		stubFinder{},
		stubParser{},
		nil,
		contracts.WithMaxParallelSegments(2),
		contracts.WithSegmentTimeout(3),
		contracts.WithSegmentRateLimit(4),
		contracts.WithPhoneticPreprocessing(true),
	)

	if e.config.MaxParallelSegments != 2 {
		t.Fatalf("MaxParallelSegments = %d, want 2", e.config.MaxParallelSegments)
	}
	if e.config.SegmentTimeout != 3 {
		t.Fatalf("SegmentTimeout = %v, want 3", e.config.SegmentTimeout)
	}
	if e.config.SegmentRateLimit != 4 {
		t.Fatalf("SegmentRateLimit = %v, want 4", e.config.SegmentRateLimit)
	}
	if !e.config.PhoneticPreprocessing {
		t.Fatal("PhoneticPreprocessing = false, want true")
	}
}

func TestPrepareSegmentsUsesDefaultTagFallback(t *testing.T) {
	e := New(
		stubClient{},
		stubFinder{
			styleIDs: map[string]int{
				"[ずんだもん][ノーマル]": 42,
			},
			defaultTags: map[string]string{
				"[ずんだもん]": "[ずんだもん][ノーマル]",
			},
		},
		stubParser{
			segments: []contracts.Segment{
				{SpeakerTag: "[ずんだもん][未知]", BaseSpeakerTag: "[ずんだもん]", Text: "hello"},
			},
		},
		nil,
	)

	segments, preCalcErrors, err := e.prepareSegments(context.Background(), "ignored", contracts.NewRunConfig())
	if err != nil {
		t.Fatalf("prepareSegments() error = %v", err)
	}
	if len(preCalcErrors) != 0 {
		t.Fatalf("preCalcErrors = %v, want none", preCalcErrors)
	}
	if len(segments) != 1 {
		t.Fatalf("len(segments) = %d, want 1", len(segments))
	}
	if segments[0].StyleID != 42 {
		t.Fatalf("StyleID = %d, want 42", segments[0].StyleID)
	}
	if got := e.styleIDCache["[ずんだもん][未知]"]; got != 42 {
		t.Fatalf("cached StyleID = %d, want 42", got)
	}
}

func TestPrepareSegmentsReturnsBatchErrorWhenAllStyleLookupsFail(t *testing.T) {
	e := New(
		stubClient{},
		stubFinder{},
		stubParser{
			segments: []contracts.Segment{
				{SpeakerTag: "[ずんだもん][未知]", BaseSpeakerTag: "[ずんだもん]", Text: "a"},
				{SpeakerTag: "[めたん][未知]", BaseSpeakerTag: "[めたん]", Text: "b"},
			},
		},
		nil,
	)

	_, _, err := e.prepareSegments(context.Background(), "ignored", contracts.NewRunConfig())
	if err == nil {
		t.Fatal("prepareSegments() error = nil, want batch error")
	}

	var batchErr *ErrSynthesisBatch
	if !errors.As(err, &batchErr) {
		t.Fatalf("error type = %T, want *ErrSynthesisBatch", err)
	}
	if batchErr.TotalErrors != 2 {
		t.Fatalf("TotalErrors = %d, want 2", batchErr.TotalErrors)
	}
	if !strings.Contains(batchErr.Error(), "[ずんだもん][未知]") {
		t.Fatalf("batch error = %q, want missing tag detail", batchErr.Error())
	}
}

func TestPrepareScriptSegmentsBuildsCombinedTags(t *testing.T) {
	e := New(
		stubClient{},
		stubFinder{
			styleIDs: map[string]int{
				"[ずんだもん][ノーマル]": 3,
			},
		},
		stubParser{},
		nil,
	)

	segments, preCalcErrors, err := e.prepareScriptSegments(context.Background(), []contracts.ScriptLine{
		{Speaker: "ずんだもん", Style: "ノーマル", Direction: "呼びかけ", Text: "こんにちは"},
	})
	if err != nil {
		t.Fatalf("prepareScriptSegments() error = %v", err)
	}
	if len(preCalcErrors) != 0 {
		t.Fatalf("preCalcErrors = %v, want none", preCalcErrors)
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
	if segments[0].Text != "こんにちは" {
		t.Fatalf("Text = %q", segments[0].Text)
	}
	if segments[0].StyleID != 3 {
		t.Fatalf("StyleID = %d, want 3", segments[0].StyleID)
	}
}

func TestPrepareScriptSegmentsForceSplitsLongText(t *testing.T) {
	e := New(
		stubClient{},
		stubFinder{
			styleIDs: map[string]int{
				"[ずんだもん][ノーマル]": 3,
			},
		},
		stubParser{},
		nil,
	)

	longText := strings.Repeat("あ", 210)
	segments, _, err := e.prepareScriptSegments(context.Background(), []contracts.ScriptLine{
		{Speaker: "ずんだもん", Style: "ノーマル", Text: longText},
	})
	if err != nil {
		t.Fatalf("prepareScriptSegments() error = %v", err)
	}
	if len(segments) != 2 {
		t.Fatalf("len(segments) = %d, want 2", len(segments))
	}
	if got := len([]rune(segments[0].Text)); got != parser.MaxSegmentCharLength {
		t.Fatalf("segments[0] rune len = %d, want %d", got, parser.MaxSegmentCharLength)
	}
	if got := len([]rune(segments[1].Text)); got != 10 {
		t.Fatalf("segments[1] rune len = %d, want 10", got)
	}
	for _, seg := range segments {
		if seg.SpeakerTag != "[ずんだもん][ノーマル]" {
			t.Fatalf("SpeakerTag = %q, want same tag on every split chunk", seg.SpeakerTag)
		}
	}
}

func TestPrepareScriptSegmentsReturnsBatchErrorWhenAllStyleLookupsFail(t *testing.T) {
	e := New(
		stubClient{},
		stubFinder{},
		stubParser{},
		nil,
	)

	_, _, err := e.prepareScriptSegments(context.Background(), []contracts.ScriptLine{
		{Speaker: "ずんだもん", Style: "未知", Text: "a"},
	})
	if err == nil {
		t.Fatal("prepareScriptSegments() error = nil, want batch error")
	}

	var batchErr *ErrSynthesisBatch
	if !errors.As(err, &batchErr) {
		t.Fatalf("error type = %T, want *ErrSynthesisBatch", err)
	}
}

func TestPrepareScriptSegmentsErrorsOnEmptyInput(t *testing.T) {
	e := New(stubClient{}, stubFinder{}, stubParser{}, nil)

	_, _, err := e.prepareScriptSegments(context.Background(), nil)
	if err == nil {
		t.Fatal("prepareScriptSegments() error = nil, want error for empty input")
	}
}
