package voicevox

import (
	"time"

	"github.com/shouni/audio/phonetic"

	internalengine "github.com/shouni/go-voicevox/internal/engine"
)

// Option changes the settings of the engine New builds.
//
// It is not an alias of internal/engine's Option, because the settings have more than one
// destination. Parallelism, timeout and rate limit are read by the synthesis engine
// (internal/engine), while reading overrides are read by the reading converter New builds
// (audio/phonetic). Keeping the alias would mean putting a field on the engine's Config that
// the engine never reads: "settable but read by no one", the shape this repository has
// repeatedly removed.
type Option func(*options)

// options groups the settings New received by destination.
type options struct {
	engine    []internalengine.Option
	converter []phonetic.Option
}

// newOptions sorts the given Options by destination.
func newOptions(opts ...Option) options {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// WithMaxParallelSegments sets the parallelism for segment synthesis (default 5; 0 or less is
// ignored).
func WithMaxParallelSegments(n int) Option {
	return func(o *options) {
		o.engine = append(o.engine, internalengine.WithMaxParallelSegments(n))
	}
}

// WithSegmentTimeout sets the timeout per segment (default 180 seconds; 0 or less is ignored).
func WithSegmentTimeout(dur time.Duration) Option {
	return func(o *options) {
		o.engine = append(o.engine, internalengine.WithSegmentTimeout(dur))
	}
}

// WithSegmentRateLimit sets the rate-limit interval for segment synthesis (default 100ms; 0 or
// less is ignored).
//
// It cannot be used to raise throughput. Concurrency is bounded by WithMaxParallelSegments;
// this interval only smooths out the burst of connections at startup.
func WithSegmentRateLimit(dur time.Duration) Option {
	return func(o *options) {
		o.engine = append(o.engine, internalengine.WithSegmentRateLimit(dur))
	}
}

// WithNumberReading converts Arabic numerals to their Japanese readings.
//
// The morphological analyser's dictionary has no readings for Arabic numerals, so by default
// "2026年8月" becomes "2026ネン8ツキ". With this option it becomes
// "ニセンニジュウロクネンハチガツ", getting both the number readings and the sound changes of
// counters (一回→イッカイ, 三本→サンボン).
//
// In a script with dates, head counts or ages, VOICEVOX otherwise reads the digits literally
// (8日→ハチニチ, 1人→イチニン, 20歳→ニジュッサイ). They can also be supplied word by word
// with WithReadingOverrides, but numbers are endless, so a rule comes first.
//
// It is not enabled by default. A change in reading is an observable change in behaviour, so
// whether to enable it is the caller's choice.
func WithNumberReading() Option {
	return func(o *options) {
		o.converter = append(o.converter, phonetic.WithNumberReading())
	}
}

// WithReadingOverrides adds readings per surface form for the reading conversion.
// The key is the spelling as it appears in the text, the value its reading (katakana).
//
//	voicevox.WithReadingOverrides(map[string]string{"8日": "ヨウカ", "1人": "ヒトリ"})
//
// It exists for proper nouns and for words the rules get wrong. Numbers and counters are read
// by rule with WithNumberReading, which is enough for them. What goes here is each work's
// personal and place names, and readings no rule produces (special readings such as
// 8日→ヨウカ). Having the prompt "write it in katakana" distorts the script itself, and since
// a check of the converted text still shows the original spelling, the misreading goes
// unnoticed until synthesis.
//
// It is layered on top of the bundled default dictionary, and wins for the same spelling.
// Empty keys or values are ignored. When given more than once, the later one takes precedence.
//
// Which words get which reading is application vocabulary. For the same reason as the speaker
// list, the library holds none and only accepts it: proper nouns differ per work, and needing
// a library release for every added word would never keep up.
func WithReadingOverrides(overrides map[string]string) Option {
	return func(o *options) {
		if len(overrides) == 0 {
			return
		}
		o.converter = append(o.converter, phonetic.WithReadingOverrides(overrides))
	}
}
