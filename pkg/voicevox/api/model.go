package api

// ----------------------------------------------------------------------
// データモデル (API応答)
// ----------------------------------------------------------------------

// AudioQueryResponse は /audio_query APIの応答構造の一部に対応する型です。
type AudioQueryResponse struct {
	AccentPhrases []map[string]interface{} `json:"accent_phrases"`
	SpeedScale    float64                  `json:"speedScale"`
	// ... 他のフィールドは必要に応じて追加
}
