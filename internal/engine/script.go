package engine

// ScriptLine is one utterance, which the caller builds directly from structured data (such
// as JSON). It is passed to Engine.Run as is.
type ScriptLine struct {
	// Speaker is the speaker name (e.g. "ずんだもん"), without square brackets.
	Speaker string `json:"speaker"`
	// Style is the VOICEVOX style name (e.g. "ノーマル"), without square brackets.
	Style string `json:"style"`
	// Text is the text to synthesize.
	Text string `json:"text"`
}
