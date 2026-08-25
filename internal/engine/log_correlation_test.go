package engine

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"testing"
)

// 本ファイルは「ライブラリのログが呼び出し元の context を運ぶ」という不変条件を固定します。
//
// アプリ側（ap-voice）は go-utils/slogctx のハンドラーを既定ロガーに入れ、リクエスト
// 単位の属性を context に積んでいます。ここで slog.Info（context を取らない版）を使うと、
// slog はハンドラーへ context.Background() を渡すため、その行だけ相関属性が付きません。
// 落ちるのは合成本体のログ、つまりジョブを追うときに一番見たい行です。

// correlationKey は、テスト用ハンドラーが拾う目印です。slogctx を import すると
// go-utils への依存辺が増えるため、同じ仕組みを最小限で再現します。
type correlationKey struct{}

// capturingHandler は、目印を持つ context から出たレコードのメッセージだけを集めます。
// 目印を持たないレコードは捨てるので、並列に走る他のテストのログとは混ざりません。
type capturingHandler struct {
	mu       *sync.Mutex
	messages *[]string
}

func (h capturingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h capturingHandler) Handle(ctx context.Context, record slog.Record) error {
	if ctx.Value(correlationKey{}) == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	*h.messages = append(*h.messages, record.Message)
	return nil
}

func (h capturingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h capturingHandler) WithGroup(string) slog.Handler      { return h }

func TestSynthesisLogsCarryCallerContext(t *testing.T) {
	// 既定ロガーを差し替えるため、このテストは並列化しません。
	var mu sync.Mutex
	var messages []string

	previous := slog.Default()
	slog.SetDefault(slog.New(capturingHandler{mu: &mu, messages: &messages}))
	t.Cleanup(func() { slog.SetDefault(previous) })

	e := New(
		instantClient{},
		stubFinder{styleIDs: map[string]int{"[話者アルファ][標準]": 1}},
		stubConverter{},
		WithMaxParallelSegments(1),
	)

	ctx := context.WithValue(context.Background(), correlationKey{}, "job-1")
	// 5 件ちょうどにすると、進捗ログ（5 件ごと・最終件）が必ず 1 回出ます。
	if _, err := e.Run(ctx, testLines(5)); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// 後ろ 2 つは ctx を引数に持たないヘルパー（logSynthesisProgress /
	// logSynthesisSummary）から出るため、退行しやすい箇所です。
	want := []string{
		"音声合成バッチ処理開始",
		"音声合成進捗",
		"全セグメントの処理が終了しました",
	}

	mu.Lock()
	got := slices.Clone(messages)
	mu.Unlock()

	for _, message := range want {
		if !slices.Contains(got, message) {
			t.Errorf("%q が呼び出し元の context を運んでいません。slog.Info/Warn ではなく Context 版を使ってください。届いたのは %q", message, got)
		}
	}
}
