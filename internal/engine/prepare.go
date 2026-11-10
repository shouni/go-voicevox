package engine

import (
	"context"
	"errors"
)

// errNoSegments reports that there is nothing at all to synthesize.
var errNoSegments = errors.New("no valid segments could be extracted from the script")

// segment is the unit of one synthesis call.
//
// It is internal only. prepareSegments assembles the bracketed tags and resolves the style ID
// on the spot; a caller never builds one. The way in is ScriptLine.
//
// It used to be split into a Segment holding only the tag and text, and an engineSegment that
// added StyleID / Err. The former existed only to be converted into the latter and was never
// handed anywhere on its own.
type segment struct {
	SpeakerTag     string
	BaseSpeakerTag string
	Text           string
	StyleID        int
	// Err is why resolving the style ID failed. A segment with Err set is not sent to the
	// API and is counted as a failure.
	Err error
}

// prepareSegments expands structured ScriptLines into synthesis units.
// Lines over the length limit are split, and each segment's style ID is resolved on the spot.
//
// The returned preCalcErrors are the ones that failed to resolve. Only when every one fails
// does it return an error, stopping before anything goes out to the network.
func (e *Engine) prepareSegments(ctx context.Context, lines []ScriptLine) ([]segment, []error, error) {
	if len(lines) == 0 {
		return nil, nil, errNoSegments
	}

	var (
		segments      []segment
		preCalcErrors []error
	)
	resolver := newStyleResolver(e.styles)
	for _, line := range lines {
		if line.Text == "" {
			continue
		}
		baseTag := "[" + line.Speaker + "]"
		speakerTag := baseTag + "[" + line.Style + "]"

		for _, chunk := range splitByCharLimit(line.Text, maxSegmentCharLength) {
			if chunk == "" {
				continue
			}
			seg := segment{
				SpeakerTag:     speakerTag,
				BaseSpeakerTag: baseTag,
				Text:           e.converter.ConvertToReading(chunk),
			}

			styleID, err := resolver.resolve(ctx, seg.SpeakerTag, seg.BaseSpeakerTag, len(segments))
			if err != nil {
				seg.Err = err
				preCalcErrors = append(preCalcErrors, err)
			} else {
				seg.StyleID = styleID
			}

			segments = append(segments, seg)
		}
	}

	if len(segments) == 0 {
		return nil, nil, errNoSegments
	}
	if len(preCalcErrors) == len(segments) {
		return nil, nil, newErrSynthesisBatch(preCalcErrors)
	}

	return segments, preCalcErrors, nil
}
