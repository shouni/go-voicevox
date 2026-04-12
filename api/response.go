package api

// AudioQueryResponse は、/audio_query API の応答構造の一部に対応する型です。
type AudioQueryResponse struct {
	// AccentPhrases はアクセント句情報のリストです。
	AccentPhrases []map[string]interface{} `json:"accent_phrases"`
	// SpeedScale は話速のスケール設定です。
	SpeedScale float64 `json:"speedScale"`
}
