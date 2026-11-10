package engine

import (
	"context"
	"fmt"
	"log/slog"
)

// styleResolver resolves style IDs and lives only for the duration of one Run.
//
// The cache belongs here, not to Engine. Engine used to hold a map and an RWMutex, but what
// the cache wraps (StyleFinder.GetStyleID) is itself a map lookup, so carrying it across Runs
// gained almost nothing. Shared mutable state, on the other hand, turned "can two Runs use the
// same Engine at once?" into a question about the lock being right. Scoped to one Run, Engine
// is immutable after construction and the question goes away.
//
// The fallback warning is also logged once per batch. Once per Engine lifetime meant the
// second and later jobs silently repeated the same miss.
type styleResolver struct {
	styles StyleFinder
	cache  map[string]int
}

func newStyleResolver(styles StyleFinder) *styleResolver {
	return &styleResolver{
		styles: styles,
		cache:  make(map[string]int),
	}
}

// resolve finds the style ID for a speaker tag (with a per-batch cache).
func (r *styleResolver) resolve(ctx context.Context, tag string, baseSpeakerTag string, index int) (int, error) {
	if id, ok := r.cache[tag]; ok {
		return id, nil
	}

	styleID, ok := r.styles.GetStyleID(tag)
	if ok {
		r.cache[tag] = styleID
		return styleID, nil
	}

	if baseSpeakerTag == "" {
		return 0, fmt.Errorf("extract speaker tag %s (segment %d)", tag, index)
	}

	fallbackKey, defaultOk := r.styles.GetDefaultTag(baseSpeakerTag)
	if defaultOk {
		slog.WarnContext(ctx, "tag from AI output is undefined, applying fallback",
			"segment_index", index,
			"original_tag", tag,
			"fallback_key", fallbackKey)

		styleID, styleOk := r.styles.GetStyleID(fallbackKey)
		if styleOk {
			r.cache[tag] = styleID
			return styleID, nil
		}
	}

	return 0, fmt.Errorf("no ID found for speaker/style tag %s (segment %d)", tag, index)
}
