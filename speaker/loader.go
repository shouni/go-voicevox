package speaker

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
)

// Client is a client that fetches the speaker list.
//
// It is declared in the package that uses it. LoadStyles is a public method, so if its
// argument type lived in an internal package, a caller could pass one but not write its name.
// It is a single method, so supplying your own client is easy too.
type Client interface {
	GetSpeakers(ctx context.Context) ([]byte, error)
}

// Styles is the "tag → style ID" mapping resolved by asking the engine.
// It satisfies the style ID resolution interface internal/engine requires.
//
// Its contents are read-only. The maps used to be exported fields, but only LoadStyles
// assembles them, so there is no reason to leave a way to rewrite them mid-synthesis.
type Styles struct {
	// byTag maps a full tag to its style ID (e.g. "[四国めたん][ノーマル]" -> 2).
	byTag map[string]int
	// defaultByTag maps a speaker tag to its default style tag
	// (e.g. "[四国めたん]" -> "[四国めたん][ノーマル]").
	defaultByTag map[string]string
}

// GetStyleID looks up the style ID for the given tag.
func (s *Styles) GetStyleID(tag string) (styleID int, ok bool) {
	id, found := s.byTag[tag]
	return id, found
}

// GetDefaultTag looks up, from a speaker's base tag, the style tag to fall back to.
func (s *Styles) GetDefaultTag(baseSpeakerTag string) (fallbackTag string, ok bool) {
	tag, found := s.defaultByTag[baseSpeakerTag]
	return tag, found
}

// LoadStyles calls the /speakers endpoint and resolves the style IDs of the speakers and
// styles in this list.
//
// Style IDs always come from the live engine. They can change between engine builds, so
// using saved values as is turns a missed update into "speaking in another character's
// voice".
//
// A nil receiver applies no filter and accepts everything the engine returned. Either way,
// combinations the engine does not have are never built.
//
// It is a method on Registry because the list itself is the filter. It used to be a function,
// LoadSpeakers(ctx, client, allowed), which put the subject, the list, in the third argument,
// and nothing at the call site explained what passing nil meant.
func (r *Registry) LoadStyles(ctx context.Context, client Client) (*Styles, error) {
	bodyBytes, err := client.GetSpeakers(ctx)
	if err != nil {
		return nil, err
	}

	var engineSpeakers []vvSpeaker
	if err := json.Unmarshal(bodyBytes, &engineSpeakers); err != nil {
		return nil, &ErrInvalidPayload{Context: "/speakers response", WrappedErr: err}
	}

	styles := &Styles{
		byTag:        make(map[string]int),
		defaultByTag: make(map[string]string),
	}

	for _, spk := range engineSpeakers {
		wanted, restricted := r.allowedStyles(spk.Name)
		if restricted && wanted == nil {
			slog.DebugContext(ctx, "skipping speaker not in the list", "speaker", spk.Name)
			continue
		}

		// Singing styles are filtered out once here, and the same slice is reused afterwards.
		talk := spk.talkStyles()
		for _, style := range talk {
			if restricted && !slices.Contains(wanted, style.Name) {
				slog.DebugContext(ctx, "skipping style not in the list", "speaker", spk.Name, "style", style.Name)
				continue
			}
			styles.byTag[styleTag(spk.Name, style.Name)] = style.ID
		}

		if tag, ok := r.resolveDefaultTag(styles.byTag, spk.Name, talk); ok {
			styles.defaultByTag[speakerTag(spk.Name)] = tag
		}
	}

	// If no speaker could be built, every later segment would fail. Stop before synthesis
	// starts.
	if len(styles.defaultByTag) == 0 {
		return nil, &ErrMissingRequiredField{
			Field:   "available speakers",
			Context: "no usable speaker could be built from the engine's /speakers response",
		}
	}

	slog.InfoContext(ctx, "VOICEVOX style data loaded",
		"speakers_count", len(styles.defaultByTag), "styles_count", len(styles.byTag))

	return styles, nil
}

// allowedStyles returns the style names allowed for this speaker and whether a filter is in
// effect. A nil list means no filter (restricted=false).
func (r *Registry) allowedStyles(name string) (styles []string, restricted bool) {
	if r == nil {
		return nil, false
	}
	names, ok := r.StylesFor(name)
	if !ok {
		return nil, true
	}
	return names, true
}

// resolveDefaultTag decides a speaker's fallback tag.
//
// The first choice is the list's default (its first style), but if the engine did not return
// it, the first style that was actually built is used. A saved list being newer than the
// engine is normal, so there is no reason to lose the whole speaker over it.
func (r *Registry) resolveDefaultTag(styleIDs map[string]int, name string, talkStyles []vvStyle) (string, bool) {
	if preferred, ok := r.DefaultStyleFor(name); ok {
		if tag := styleTag(name, preferred); hasTag(styleIDs, tag) {
			return tag, true
		}
	}

	for _, style := range talkStyles {
		if tag := styleTag(name, style.Name); hasTag(styleIDs, tag) {
			return tag, true
		}
	}
	return "", false
}

// hasTag checks only whether the key exists, so that a style ID of 0 (a real value, such as
// あまあま's ID) is not mistaken for a miss.
func hasTag(m map[string]int, key string) bool {
	_, ok := m[key]
	return ok
}
