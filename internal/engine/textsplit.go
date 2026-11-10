package engine

import "unicode/utf8"

// maxSegmentCharLength is a guideline for the most characters VOICEVOX handles safely.
//
// The value itself does not leave this package. The caller does not choose the limit; the way
// to use the whole splitting rule is SplitForSynthesis.
const maxSegmentCharLength = 200

// SplitForSynthesis splits text by the same rule synthesis uses to divide a line into the
// units sent to VOICEVOX. It lets the reading preview convert at the same breaks as
// synthesis, and keeps the splitting rule in this one place (with different breaks, a word
// across a boundary would be read differently from synthesis).
func SplitForSynthesis(text string) []string {
	return splitByCharLimit(text, maxSegmentCharLength)
}

// splitPoints are the characters a split may follow. The comma (、) is included because some
// long text does not fit under the limit with full stops (。) alone.
var splitPoints = map[rune]bool{'。': true, '、': true, '！': true, '？': true}

// splitByCharLimit splits text into chunks of at most limit characters, preferring
// punctuation (。、！？). When no punctuation is found it cuts mechanically at limit.
func splitByCharLimit(text string, limit int) []string {
	if limit <= 0 || utf8.RuneCountInString(text) <= limit {
		return []string{text}
	}

	var chunks []string
	for remaining := text; remaining != ""; {
		head, rest := cutHead(remaining, limit)
		chunks = append(chunks, head)
		remaining = rest
	}
	return chunks
}

// cutHead returns the head that fits within limit characters, and the rest.
//
// It cuts at the last punctuation mark within the limit. Cutting at the first one would leave
// a run of needlessly short fragments and unnatural pauses in the synthesis. Only when there
// is no punctuation at all does it cut mechanically at the limit.
//
// head always has at least one character, because an empty head would stop the caller from
// making progress.
func cutHead(text string, limit int) (head, rest string) {
	runes := []rune(text)
	if len(runes) <= limit {
		return text, ""
	}

	for i := limit - 1; i >= 0; i-- {
		if splitPoints[runes[i]] {
			return string(runes[:i+1]), string(runes[i+1:])
		}
	}

	return string(runes[:limit]), string(runes[limit:])
}
