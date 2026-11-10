package engine

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"testing"
)

// This file pins the invariant that the library's logs carry the caller's context.
//
// The application (ap-voice) installs go-utils/slogctx's handler as the default logger and
// puts per-request attributes on the context. If slog.Info (the variant without a context) is
// used here, slog hands the handler context.Background(), and that line alone loses the
// correlation attributes. The lines that lose them are the synthesis logs themselves, i.e. the
// ones most wanted when following a job.

// correlationKey is the marker the test handler looks for. Importing slogctx would add a
// dependency edge on go-utils, so the same mechanism is reproduced minimally.
type correlationKey struct{}

// capturingHandler collects the messages of records logged from a context carrying the
// marker. Records without it are dropped, so logs from other tests running in parallel do not
// mix in.
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
	// This test replaces the default logger, so it does not run in parallel.
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
	// With exactly 5 lines the progress log (every 5th and the last) is emitted exactly once.
	if _, err := e.Run(ctx, testLines(5)); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// The last two come from helpers (logSynthesisProgress / logSynthesisSummary), which are
	// the places most prone to regress.
	want := []string{
		"synthesis batch started",
		"synthesis progress",
		"all segments processed",
	}

	mu.Lock()
	got := slices.Clone(messages)
	mu.Unlock()

	for _, message := range want {
		if !slices.Contains(got, message) {
			t.Errorf("%q does not carry the caller's context; use the Context variants instead of slog.Info/Warn. Received %q", message, got)
		}
	}
}
