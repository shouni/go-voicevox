package engine

import (
	"errors"
	"strings"
	"testing"

	"github.com/shouni/audio/wav"
)

// buildWav assembles a minimal WAV in the given format.
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
	_, err := combineOutput(nil, []error{errors.New("preparation error")}, []error{errors.New("runtime error")})

	var batch *ErrSynthesisBatch
	if !errors.As(err, &batch) {
		t.Fatalf("did not return ErrSynthesisBatch: %v", err)
	}
	if len(batch.Errors) != 2 {
		t.Errorf("error count = %d, want 2", len(batch.Errors))
	}
}

func TestCombineOutput_NoValidData(t *testing.T) {
	if _, err := combineOutput([][]byte{nil, nil}, nil, nil); err == nil {
		t.Fatal("no error without any valid data")
	}
}

func TestCombineOutput_Success(t *testing.T) {
	list := [][]byte{
		buildWav(t, 24000, []byte{1, 2, 3, 4}),
		nil, // a segment with empty text
		buildWav(t, 24000, []byte{5, 6, 7, 8}),
	}

	got, err := combineOutput(list, nil, nil)
	if err != nil {
		t.Fatalf("combineOutput() error = %v", err)
	}
	// the payloads of both are concatenated
	if want := 44 + 8; len(got) != want {
		t.Errorf("combined result = %d bytes, want %d", len(got), want)
	}
}

// TestCombineOutput_MismatchReportsOriginalSegment checks that a format mismatch is reported
// against the original segment number, not the position after compaction. Segments with empty
// text stay nil, so the two differ.
func TestCombineOutput_MismatchReportsOriginalSegment(t *testing.T) {
	list := [][]byte{
		buildWav(t, 24000, []byte{1, 2}), // segment 0 → still 0 after compaction
		nil,                              // segment 1: empty text
		nil,                              // segment 2: empty text
		buildWav(t, 44100, []byte{3, 4}), // segment 3 → 1 after compaction
	}

	_, err := combineOutput(list, nil, nil)
	if err == nil {
		t.Fatal("no error on a format mismatch")
	}

	// the original type is kept
	var mismatch *wav.ErrMismatchedWAVFormat
	if !errors.As(err, &mismatch) {
		t.Fatalf("ErrMismatchedWAVFormat is not wrapped: %v", err)
	}
	if mismatch.Index != 1 {
		t.Errorf("wav Index = %d, want 1 (the position after compaction)", mismatch.Index)
	}

	// the message shows the original segment number
	if !strings.Contains(err.Error(), "segment 3 audio format") {
		t.Errorf("the error does not point at the original segment number: %v", err)
	}
}

func TestNonNilAudioData(t *testing.T) {
	data, indexes := nonNilAudioData([][]byte{nil, {1}, nil, {2}})

	if len(data) != 2 {
		t.Fatalf("data count = %d, want 2", len(data))
	}
	if want := []int{1, 3}; len(indexes) != 2 || indexes[0] != want[0] || indexes[1] != want[1] {
		t.Errorf("segmentIndexes = %v, want %v", indexes, want)
	}
}

// TestWithSegmentIndex_TranslatesSubFormatMismatch checks that a sub-format mismatch is
// restated as the original segment number, just like an ordinary format mismatch.
func TestWithSegmentIndex_TranslatesSubFormatMismatch(t *testing.T) {
	base := &wav.ErrMismatchedSubFormat{Index: 1}
	got := withSegmentIndex(base, []int{0, 3})

	if !errors.Is(got, base) {
		t.Fatalf("the original error is not wrapped: %v", got)
	}
	if !strings.Contains(got.Error(), "segment 3 audio format") {
		t.Errorf("the error does not point at the original segment number: %v", got)
	}
}

func TestWithSegmentIndex_TranslatesUnsupportedFormat(t *testing.T) {
	base := &wav.ErrUnsupportedWAVFormat{Index: 1}
	got := withSegmentIndex(base, []int{0, 3})

	if !errors.Is(got, base) {
		t.Fatalf("the original error is not wrapped: %v", got)
	}
	if !strings.Contains(got.Error(), "segment 3 audio format cannot be combined") {
		t.Errorf("the error does not point at the original segment number: %v", got)
	}
}

func TestWithSegmentIndex_PassesThroughUnknownError(t *testing.T) {
	base := errors.New("some other error")
	if got := withSegmentIndex(base, []int{0, 1}); !errors.Is(got, base) {
		t.Errorf("an unknown error was not passed through: %v", got)
	}
}
