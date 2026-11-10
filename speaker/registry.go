// Package speaker maps VOICEVOX speakers and styles to the tags used inside this library.
package speaker

import (
	"encoding/json"
	"fmt"
	"slices"
)

// styleTypeTalk is the type of a style usable for speech.
// /audio_query and /synthesis are for speech, so singing styles are excluded.
// The empty string is also accepted, treating older responses without a type as talk.
const styleTypeTalk = "talk"

// speakerEntry is one speaker with only its speech styles kept.
//
// The filtering happens once, in NewRegistry. It used to strip singing styles with
// talkStyles() on every query, rebuilding the same result over and over. The list does not
// change after construction, so keeping the filtered result is enough.
type speakerEntry struct {
	name string
	// styles are the speech styles in declaration order. NewRegistry guarantees it is not
	// empty.
	styles []vvStyle
}

// Registry is the list of speakers and styles to use.
//
// This library ships no list. Which speakers to use is application policy, not a concern of
// the synthesis engine, so the caller saves a /speakers response and passes it in. Baking a
// list into the library would require a library release to add a single speaker, and would
// stop each app from choosing its own cast.
//
// It holds only the vocabulary (who has which styles) and does not use style IDs. IDs can
// change between engine builds, so LoadStyles asks the live engine at startup.
type Registry struct {
	// speakers is in declaration order; enumeration follows the order of the response the
	// caller passed.
	speakers []speakerEntry
	// indexByName maps a name to its position in speakers. LoadStyles repeats the per-speaker
	// lookup once per engine speaker, so it does not stack linear scans.
	indexByName map[string]int
}

// NewRegistry builds a Registry from a VOICEVOX /speakers response.
//
// The response can be passed as is. Updating means saving it again, and unlike a hand-copied
// list it never misses styles the engine has added.
//
//	curl -s "$VOICEVOX_API_URL/speakers" -o speakers.json
func NewRegistry(raw []byte) (*Registry, error) {
	var speakers []vvSpeaker
	if err := json.Unmarshal(raw, &speakers); err != nil {
		return nil, &ErrInvalidPayload{
			Context:    "saved /speakers response",
			WrappedErr: err,
		}
	}
	if len(speakers) == 0 {
		return nil, &ErrMissingRequiredField{Field: "speaker list", Context: "it contains no speakers"}
	}

	r := &Registry{
		speakers:    make([]speakerEntry, 0, len(speakers)),
		indexByName: make(map[string]int, len(speakers)),
	}
	for _, s := range speakers {
		if s.Name == "" {
			return nil, &ErrMissingRequiredField{Field: "speaker name", Context: "a speaker has no name"}
		}
		// A speaker with no speech style has nothing to fall back to.
		talk := s.talkStyles()
		if len(talk) == 0 {
			return nil, &ErrMissingRequiredField{
				Field:   "speech style",
				Context: fmt.Sprintf("speaker %q has no speech style", s.Name),
			}
		}

		// When a name appears twice, the first occurrence wins (as the old linear scan did).
		if _, seen := r.indexByName[s.Name]; !seen {
			r.indexByName[s.Name] = len(r.speakers)
		}
		r.speakers = append(r.speakers, speakerEntry{name: s.Name, styles: talk})
	}

	return r, nil
}

// SpeakerNames returns the speaker names in this list (e.g. "四国めたん"), spelled exactly as
// VOICEVOX spells them.
//
// It is meant for callers that enumerate the allowed vocabulary, such as when building an AI
// prompt or response schema.
func (r *Registry) SpeakerNames() []string {
	names := make([]string, len(r.speakers))
	for i, s := range r.speakers {
		names[i] = s.name
	}
	return names
}

// StyleNames returns, in declaration order, the style names that any speaker has.
//
// Not every speaker has all of them. When per-speaker combinations are needed, use StylesFor.
// Offering the union as is lets the AI pick combinations that do not exist, and those fall
// back to the default style, silently ignoring the instruction.
func (r *Registry) StyleNames() []string {
	var names []string
	for _, s := range r.speakers {
		for _, style := range s.styles {
			if !slices.Contains(names, style.Name) {
				names = append(names, style.Name)
			}
		}
	}
	return names
}

// StylesFor returns the speech style names of the given speaker in declaration order.
// It returns false for a speaker not in the list.
func (r *Registry) StylesFor(name string) ([]string, bool) {
	s, ok := r.find(name)
	if !ok {
		return nil, false
	}

	names := make([]string, len(s.styles))
	for i, style := range s.styles {
		names[i] = style.Name
	}
	return names, true
}

// DefaultStyleFor returns the default style name of the given speaker.
//
// The first style is the default, because some speakers have no "ノーマル" at all
// (白上虎太郎 has ふつう, 後鬼 has 人間ver., 里石ユカ has つぼみ). The engine's list puts each
// speaker's canonical style first.
func (r *Registry) DefaultStyleFor(name string) (string, bool) {
	s, ok := r.find(name)
	if !ok {
		return "", false
	}

	// NewRegistry guarantees it is not empty.
	return s.styles[0].Name, true
}

func (r *Registry) find(name string) (speakerEntry, bool) {
	if r == nil {
		return speakerEntry{}, false
	}
	i, ok := r.indexByName[name]
	if !ok {
		return speakerEntry{}, false
	}
	return r.speakers[i], true
}

// speakerTag builds a tag from a speaker name (e.g. "四国めたん" → "[四国めたん]").
func speakerTag(name string) string {
	return "[" + name + "]"
}

// styleTag builds a full tag from a speaker name and a style name.
func styleTag(speakerName, styleName string) string {
	return speakerTag(speakerName) + "[" + styleName + "]"
}
