package voicevox

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/shouni/audio/phonetic"

	internalengine "github.com/shouni/go-voicevox/internal/engine"
)

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

// LoadStyles は、少なくとも 1 人の話者に読み上げスタイルがあることを要求します。
const stubSpeakersJSON = `[
  {"name":"話者アルファ","styles":[{"name":"標準","id":2}]},
  {"name":"話者ベータ","styles":[{"name":"標準","id":3}]},
  {"name":"話者ガンマ","styles":[{"name":"標準","id":8}]}
]`

func TestNew_BuildsEngineWhenEnabled(t *testing.T) {
	reqer := &stubRequester{speakersJSON: stubSpeakersJSON}

	engine, err := New(context.Background(), reqer, "http://voicevox.test:50021", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if engine == nil {
		t.Fatal("New() が nil を返しました")
	}
	if !strings.HasPrefix(reqer.lastTarget, "http://voicevox.test:50021") {
		t.Errorf("指定したURLが使われていません: %q", reqer.lastTarget)
	}
}

// TestNew_FallsBackToDefaultURL は、URL未指定時に既定のローカルURLを使うことを確認します。
func TestNew_FallsBackToDefaultURL(t *testing.T) {
	reqer := &stubRequester{speakersJSON: stubSpeakersJSON}

	if _, err := New(context.Background(), reqer, "", nil); err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if !strings.HasPrefix(reqer.lastTarget, defaultVoicevoxAPIURL) {
		t.Errorf("既定URLが使われていません: %q (want prefix %q)", reqer.lastTarget, defaultVoicevoxAPIURL)
	}
}

func TestNew_ReturnsErrorWhenSpeakerLoadFails(t *testing.T) {
	reqer := &stubRequester{fetchErr: errors.New("接続できません")}

	_, err := New(context.Background(), reqer, "http://voicevox.test:50021", nil)
	if err == nil {
		t.Fatal("話者ロード失敗でエラーになりませんでした")
	}
	if !strings.Contains(err.Error(), "VOICEVOXデータのロードに失敗") {
		t.Errorf("エラーが文脈を持っていません: %v", err)
	}
}

func TestOptionsAreApplied(t *testing.T) {
	o := newOptions(
		WithMaxParallelSegments(9),
		WithSegmentTimeout(7*time.Second),
		WithSegmentRateLimit(250*time.Millisecond),
	)
	cfg := internalengine.NewConfig(o.engine...)

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
	o := newOptions(
		WithMaxParallelSegments(0),
		WithSegmentTimeout(-time.Second),
		WithSegmentRateLimit(0),
	)
	cfg := internalengine.NewConfig(o.engine...)

	if cfg.MaxParallelSegments != internalengine.DefaultMaxParallelSegments {
		t.Errorf("MaxParallelSegments = %d, want %d", cfg.MaxParallelSegments, internalengine.DefaultMaxParallelSegments)
	}
	if cfg.SegmentTimeout != internalengine.DefaultSegmentTimeout {
		t.Errorf("SegmentTimeout = %v, want %v", cfg.SegmentTimeout, internalengine.DefaultSegmentTimeout)
	}
	if cfg.SegmentRateLimit != internalengine.DefaultSegmentRateLimit {
		t.Errorf("SegmentRateLimit = %v, want %v", cfg.SegmentRateLimit, internalengine.DefaultSegmentRateLimit)
	}
}

// TestWithReadingOverridesReachesConverter は、渡した読みが実際に変換へ効くことを
// 確認します。オプションが engine ではなく変換器へ届いているかを見る唯一の場所です。
func TestWithReadingOverridesReachesConverter(t *testing.T) {
	o := newOptions(WithReadingOverrides(map[string]string{"8日": "ヨウカ"}))

	converter, err := phonetic.NewConverter(o.converter...)
	if err != nil {
		t.Fatalf("NewConverter() error = %v", err)
	}

	if got := converter.ConvertToReading("8日"); !strings.Contains(got, "ヨウカ") {
		t.Errorf("ConvertToReading(\"8日\") = %q, ヨウカ を含みません", got)
	}
}

// TestWithReadingOverridesIgnoresEmpty は、空の指定が変換器の設定を増やさないことを
// 確認します。nil を渡した呼び出しが既定の辞書を組み直すだけの空回りにならないようにします。
func TestWithReadingOverridesIgnoresEmpty(t *testing.T) {
	if o := newOptions(WithReadingOverrides(nil)); len(o.converter) != 0 {
		t.Errorf("converter オプションが %d 件付きました、want 0", len(o.converter))
	}
	if o := newOptions(WithReadingOverrides(map[string]string{})); len(o.converter) != 0 {
		t.Errorf("converter オプションが %d 件付きました、want 0", len(o.converter))
	}
}
