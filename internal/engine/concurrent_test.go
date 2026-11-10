package engine

import (
	"bytes"
	"context"
	"sync"
	"testing"
)

// This file pins the invariant that one Engine can be Run from several goroutines at once.
// Engine is assumed not to change after construction, and the style ID cache belongs to
// styleResolver, which lives for one Run. Moving it back into a shared map on Engine makes
// this test fail under -race.

// wavClient is a stub that returns the same minimal WAV for every segment.
// The test goes all the way through combining, so it returns a real WAV shape rather than
// stubClient's fake bytes.
type wavClient struct {
	wav []byte
}

func (c wavClient) RunAudioQuery(context.Context, string, int) ([]byte, error) {
	return []byte("{}"), nil
}

func (c wavClient) RunSynthesis(context.Context, []byte, int) ([]byte, error) {
	return c.wav, nil
}

func TestRunIsSafeForConcurrentUse(t *testing.T) {
	t.Parallel()

	// An undefined tag is mixed in so the fallback path (a cache write) runs concurrently too.
	e := New(
		wavClient{wav: buildWav(t, 24000, []byte{1, 2, 3, 4})},
		stubFinder{
			styleIDs:    map[string]int{"[話者アルファ][標準]": 1, "[話者アルファ][既定]": 9},
			defaultTags: map[string]string{"[話者アルファ]": "[話者アルファ][既定]"},
		},
		stubConverter{},
		WithMaxParallelSegments(4),
	)

	lines := []ScriptLine{
		{Speaker: "話者アルファ", Style: "標準", Text: "いち"},
		{Speaker: "話者アルファ", Style: "存在しない", Text: "に"},
		{Speaker: "話者アルファ", Style: "標準", Text: "さん"},
	}

	const runs = 8
	var wg sync.WaitGroup
	outputs := make([][]byte, runs)
	errs := make([]error, runs)
	for i := range runs {
		wg.Go(func() {
			outputs[i], errs[i] = e.Run(context.Background(), lines)
		})
	}
	wg.Wait()

	for i := range runs {
		if errs[i] != nil {
			t.Fatalf("Run() #%d error = %v", i, errs[i])
		}
		// The same input yields the same output. A mix-up or a dropped segment breaks here.
		if !bytes.Equal(outputs[i], outputs[0]) {
			t.Fatalf("Run() #%d output differs from #0 (%d bytes, want %d bytes)",
				i, len(outputs[i]), len(outputs[0]))
		}
	}
}
