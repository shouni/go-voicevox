package speaker

// vvStyle is one style in a /speakers response.
type vvStyle struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
	// Type distinguishes "talk" from the singing types. Older responses lack it, so empty is
	// treated as talk.
	Type string `json:"type"`
}

// vvSpeaker is one speaker in a /speakers response.
//
// Both the embedded speakers.json and the response fetched from the engine at startup are
// read with this type. They are the same shape from the same endpoint, so there is no reason
// for separate types.
type vvSpeaker struct {
	Name   string    `json:"name"`
	Styles []vvStyle `json:"styles"`
}

// talkStyles returns only the styles usable for speech, in declaration order.
func (s vvSpeaker) talkStyles() []vvStyle {
	styles := make([]vvStyle, 0, len(s.Styles))
	for _, style := range s.Styles {
		if style.Type == "" || style.Type == styleTypeTalk {
			styles = append(styles, style)
		}
	}
	return styles
}
