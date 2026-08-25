package engine

import (
	"bytes"
	"context"
	"sync"
	"testing"
)

// 本ファイルは「1つの Engine を複数のゴルーチンから同時に Run できる」という不変条件を
// 固定します。Engine は構築後に書き換わらない前提で、スタイル ID のキャッシュは
// 1回の Run に閉じた styleResolver が持ちます。ここを Engine 側の共有 map へ戻すと、
// -race 付きのこのテストが落ちます。

// wavClient は、どのセグメントにも同じ最小WAVを返すスタブです。
// 結合まで通したいので、stubClient の偽バイト列ではなく本物の形を返します。
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

	// 未定義タグを混ぜ、フォールバック経路（キャッシュへの書き込み）も同時に踏ませます。
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
		wg.Add(1)
		go func() {
			defer wg.Done()
			outputs[i], errs[i] = e.Run(context.Background(), lines)
		}()
	}
	wg.Wait()

	for i := range runs {
		if errs[i] != nil {
			t.Fatalf("Run() %d 回目 error = %v", i, errs[i])
		}
		// 同じ入力からは同じ出力が出ます。取り違えや取りこぼしがあればここで割れます。
		if !bytes.Equal(outputs[i], outputs[0]) {
			t.Fatalf("Run() %d 回目の出力が 0 回目と異なります (%d バイト, want %d バイト)",
				i, len(outputs[i]), len(outputs[0]))
		}
	}
}
