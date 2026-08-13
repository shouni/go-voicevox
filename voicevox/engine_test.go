package voicevox

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/shouni/go-voicevox/internal/contracts"
)

func TestNewReturnsNoopEngineWhenDisabled(t *testing.T) {
	engine, err := New(context.Background(), nil, "", false)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if engine == nil {
		t.Fatal("New() returned nil engine")
	}
	if _, err := engine.Run(context.Background(), []ScriptLine{{Speaker: "ずんだもん", Style: "ノーマル", Text: "sample"}}); err != nil {
		t.Fatalf("noop Run() error = %v", err)
	}
}

// stubRequester は httpkit.Requester の最小スタブです。
// New は話者データのロードで /speakers を叩くため、そこだけ応答を差し替えます。
type stubRequester struct {
	speakersJSON string
	fetchErr     error
	lastTarget   string
}

func (s *stubRequester) DoRequest(*http.Request) ([]byte, error) { return nil, nil }

func (s *stubRequester) FetchBytes(_ context.Context, target string) ([]byte, string, error) {
	s.lastTarget = target
	if s.fetchErr != nil {
		return nil, "", s.fetchErr
	}
	return []byte(s.speakersJSON), "application/json", nil
}

func (s *stubRequester) FetchAndDecodeJSON(context.Context, string, any) error { return nil }

func (s *stubRequester) PostJSONAndFetchBytes(context.Context, string, any) ([]byte, error) {
	return nil, nil
}

func (s *stubRequester) PostRawBodyAndFetchBytes(context.Context, string, []byte, string) ([]byte, error) {
	return nil, nil
}

// LoadSpeakers は SupportedSpeakers の全話者にノーマルスタイルがあることを要求します。
const stubSpeakersJSON = `[
  {"name":"四国めたん","styles":[{"name":"ノーマル","id":2}]},
  {"name":"ずんだもん","styles":[{"name":"ノーマル","id":3}]},
  {"name":"春日部つむぎ","styles":[{"name":"ノーマル","id":8}]}
]`

func TestNew_BuildsEngineWhenEnabled(t *testing.T) {
	reqer := &stubRequester{speakersJSON: stubSpeakersJSON}

	engine, err := New(context.Background(), reqer, "http://voicevox.test:50021", true)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if engine == nil {
		t.Fatal("New() が nil を返しました")
	}
	if _, ok := engine.(*noopEngine); ok {
		t.Error("有効時に noopEngine が返っています")
	}
	if !strings.HasPrefix(reqer.lastTarget, "http://voicevox.test:50021") {
		t.Errorf("指定したURLが使われていません: %q", reqer.lastTarget)
	}
}

// TestNew_FallsBackToDefaultURL は、URL未指定時に既定のローカルURLを使うことを確認します。
func TestNew_FallsBackToDefaultURL(t *testing.T) {
	reqer := &stubRequester{speakersJSON: stubSpeakersJSON}

	if _, err := New(context.Background(), reqer, "", true); err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if !strings.HasPrefix(reqer.lastTarget, defaultVoicevoxAPIURL) {
		t.Errorf("既定URLが使われていません: %q (want prefix %q)", reqer.lastTarget, defaultVoicevoxAPIURL)
	}
}

func TestNew_ReturnsErrorWhenSpeakerLoadFails(t *testing.T) {
	reqer := &stubRequester{fetchErr: errors.New("接続できません")}

	_, err := New(context.Background(), reqer, "http://voicevox.test:50021", true)
	if err == nil {
		t.Fatal("話者ロード失敗でエラーになりませんでした")
	}
	if !strings.Contains(err.Error(), "VOICEVOXデータのロードに失敗") {
		t.Errorf("エラーが文脈を持っていません: %v", err)
	}
}

func TestOptionsAreApplied(t *testing.T) {
	cfg := contracts.NewEngineConfig(
		WithMaxParallelSegments(9),
		WithSegmentTimeout(7*time.Second),
		WithSegmentRateLimit(250*time.Millisecond),
	)

	if cfg.MaxParallelSegments != 9 {
		t.Errorf("MaxParallelSegments = %d, want 9", cfg.MaxParallelSegments)
	}
	if cfg.SegmentTimeout != 7*time.Second {
		t.Errorf("SegmentTimeout = %v, want 7s", cfg.SegmentTimeout)
	}
	if cfg.SegmentRateLimit != 250*time.Millisecond {
		t.Errorf("SegmentRateLimit = %v, want 250ms", cfg.SegmentRateLimit)
	}
}

// TestOptionsIgnoreNonPositive は、0以下の指定が既定値を壊さないことを確認します。
func TestOptionsIgnoreNonPositive(t *testing.T) {
	cfg := contracts.NewEngineConfig(
		WithMaxParallelSegments(0),
		WithSegmentTimeout(-time.Second),
		WithSegmentRateLimit(0),
	)

	if cfg.MaxParallelSegments != DefaultMaxParallelSegments {
		t.Errorf("MaxParallelSegments = %d, want %d", cfg.MaxParallelSegments, DefaultMaxParallelSegments)
	}
	if cfg.SegmentTimeout != DefaultSegmentTimeout {
		t.Errorf("SegmentTimeout = %v, want %v", cfg.SegmentTimeout, DefaultSegmentTimeout)
	}
	if cfg.SegmentRateLimit != DefaultSegmentRateLimit {
		t.Errorf("SegmentRateLimit = %v, want %v", cfg.SegmentRateLimit, DefaultSegmentRateLimit)
	}
}
