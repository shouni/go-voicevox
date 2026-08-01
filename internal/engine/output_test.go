package engine

import (
	"errors"
	"strings"
	"testing"

	"github.com/shouni/audio/wav"
)

// buildWav は指定フォーマットの最小WAVを組み立てます。
func buildWav(t *testing.T, sampleRate uint32, payload []byte) []byte {
	t.Helper()

	const fmtChunkSize = 16
	dataSize := uint32(len(payload))
	riffSize := 4 + (8 + fmtChunkSize) + (8 + dataSize)

	put32 := func(b []byte, v uint32) { b[0], b[1], b[2], b[3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24) }
	put16 := func(b []byte, v uint16) { b[0], b[1] = byte(v), byte(v>>8) }

	out := make([]byte, 0, 44+len(payload))
	header := make([]byte, 44)
	copy(header[0:], "RIFF")
	put32(header[4:], riffSize)
	copy(header[8:], "WAVE")
	copy(header[12:], "fmt ")
	put32(header[16:], fmtChunkSize)
	put16(header[20:], 1) // PCM
	put16(header[22:], 1) // mono
	put32(header[24:], sampleRate)
	put32(header[28:], sampleRate*2) // byte rate
	put16(header[32:], 2)            // block align
	put16(header[34:], 16)           // bits per sample
	copy(header[36:], "data")
	put32(header[40:], dataSize)

	out = append(out, header...)
	out = append(out, payload...)
	return out
}

func TestCombineOutput_ReportsBatchErrors(t *testing.T) {
	_, err := combineOutput(nil, []string{"事前エラー"}, []string{"実行時エラー"})

	var batch *ErrSynthesisBatch
	if !errors.As(err, &batch) {
		t.Fatalf("ErrSynthesisBatch が返りませんでした: %v", err)
	}
	if batch.TotalErrors != 2 {
		t.Errorf("TotalErrors = %d, want 2", batch.TotalErrors)
	}
}

func TestCombineOutput_NoValidData(t *testing.T) {
	if _, err := combineOutput([][]byte{nil, nil}, nil, nil); err == nil {
		t.Fatal("有効データなしでエラーになりませんでした")
	}
}

func TestCombineOutput_Success(t *testing.T) {
	list := [][]byte{
		buildWav(t, 24000, []byte{1, 2, 3, 4}),
		nil, // 空テキストのセグメント
		buildWav(t, 24000, []byte{5, 6, 7, 8}),
	}

	got, err := combineOutput(list, nil, nil)
	if err != nil {
		t.Fatalf("combineOutput() error = %v", err)
	}
	// 2本分のペイロードが連結される
	if want := 44 + 8; len(got) != want {
		t.Errorf("結合結果 = %d bytes, want %d", len(got), want)
	}
}

// TestCombineOutput_MismatchReportsOriginalSegment は、形式不一致の報告が
// 詰めた後の位置ではなく元のセグメント番号を指すことを確認します。
// 空テキストのセグメントが nil で残るため、両者は一致しません。
func TestCombineOutput_MismatchReportsOriginalSegment(t *testing.T) {
	list := [][]byte{
		buildWav(t, 24000, []byte{1, 2}), // セグメント0 → 詰めた後も 0
		nil,                              // セグメント1: 空テキスト
		nil,                              // セグメント2: 空テキスト
		buildWav(t, 44100, []byte{3, 4}), // セグメント3 → 詰めた後は 1
	}

	_, err := combineOutput(list, nil, nil)
	if err == nil {
		t.Fatal("形式不一致でエラーになりませんでした")
	}

	// 元の型は保たれる
	var mismatch *wav.ErrMismatchedWAVFormat
	if !errors.As(err, &mismatch) {
		t.Fatalf("ErrMismatchedWAVFormat が包まれていません: %v", err)
	}
	if mismatch.Index != 1 {
		t.Errorf("wav 側の Index = %d, want 1（詰めた後の位置）", mismatch.Index)
	}

	// 表示は元のセグメント番号
	if !strings.Contains(err.Error(), "セグメント 3 の音声形式") {
		t.Errorf("エラーが元のセグメント番号を指していません: %v", err)
	}
}

func TestNonNilAudioData(t *testing.T) {
	data, indexes := nonNilAudioData([][]byte{nil, {1}, nil, {2}})

	if len(data) != 2 {
		t.Fatalf("データ数 = %d, want 2", len(data))
	}
	if want := []int{1, 3}; len(indexes) != 2 || indexes[0] != want[0] || indexes[1] != want[1] {
		t.Errorf("segmentIndexes = %v, want %v", indexes, want)
	}
}

func TestWithSegmentIndex_PassesThroughUnknownError(t *testing.T) {
	base := errors.New("なにか別のエラー")
	if got := withSegmentIndex(base, []int{0, 1}); !errors.Is(got, base) {
		t.Errorf("未知のエラーがそのまま返っていません: %v", got)
	}
}
