package engine

import (
	"errors"
	"fmt"

	"github.com/shouni/audio/wav"
)

// errNoAudioData reports that there is no audio at all to combine.
//
// It has no value to embed, so it is a variable rather than an fmt.Errorf. Like
// errNoSegments, it stays in a form the caller can identify with errors.Is.
var errNoAudioData = errors.New("no valid synthesized audio was produced")

// combineOutput combines each segment's synthesized audio and returns the WAV bytes.
// Saving them anywhere is the caller's responsibility.
func combineOutput(orderedAudioDataList [][]byte, preCalcErrors []error, runtimeErrors []error) ([]byte, error) {
	if err := newErrSynthesisBatch(preCalcErrors, runtimeErrors); err != nil {
		return nil, err
	}

	finalAudioDataList, segmentIndexes := nonNilAudioData(orderedAudioDataList)
	if len(finalAudioDataList) == 0 {
		return nil, errNoAudioData
	}

	combinedWavBytes, err := wav.CombineWavData(finalAudioDataList)
	if err != nil {
		return nil, fmt.Errorf("combine WAV data: %w", withSegmentIndex(err, segmentIndexes))
	}

	return combinedWavBytes, nil
}

// nonNilAudioData extracts only the synthesized data and also returns which original segment
// each item came from.
//
// A segment with empty text is not synthesized and stays nil, so positions after compaction
// do not match segment numbers. The mapping is needed to report errors against the original
// numbers.
func nonNilAudioData(audioDataList [][]byte) (data [][]byte, segmentIndexes []int) {
	data = make([][]byte, 0, len(audioDataList))
	segmentIndexes = make([]int, 0, len(audioDataList))

	for i, d := range audioDataList {
		if d == nil {
			continue
		}
		data = append(data, d)
		segmentIndexes = append(segmentIndexes, i)
	}

	return data, segmentIndexes
}

// withSegmentIndex restates the position a combine error points at as the original segment
// number.
//
// The wav package only knows positions in the compacted list, so reporting those as is would
// give the wrong number whenever a segment had empty text. The original error stays wrapped,
// so errors.As still identifies it.
func withSegmentIndex(err error, segmentIndexes []int) error {
	segmentOf := func(i int) int {
		if i < 0 || i >= len(segmentIndexes) {
			return i
		}
		return segmentIndexes[i]
	}

	if mismatch, ok := errors.AsType[*wav.ErrMismatchedWAVFormat](err); ok {
		return fmt.Errorf("segment %d audio format does not match the first segment: %w",
			segmentOf(mismatch.Index), err)
	}

	// A WAVE_FORMAT_EXTENSIBLE sub-format mismatch comes back as a different type, but it means
	// the same as a format mismatch.
	if mismatch, ok := errors.AsType[*wav.ErrMismatchedSubFormat](err); ok {
		return fmt.Errorf("segment %d audio format does not match the first segment: %w",
			segmentOf(mismatch.Index), err)
	}

	// Formats such as ADPCM that cannot be joined without decoding (since audio v1.5.0).
	if unsupported, ok := errors.AsType[*wav.ErrUnsupportedWAVFormat](err); ok {
		return fmt.Errorf("segment %d audio format cannot be combined: %w", segmentOf(unsupported.Index), err)
	}

	if header, ok := errors.AsType[*wav.ErrInvalidWAVHeader](err); ok {
		return fmt.Errorf("segment %d audio data is invalid: %w", segmentOf(header.Index), err)
	}

	return err
}
