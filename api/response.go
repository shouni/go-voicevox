package api

import (
	"fmt"
)

// AudioQueryResponse は、/audio_query API の応答構造の一部に対応する型です。
type AudioQueryResponse struct {
	// AccentPhrases はアクセント句情報のリストです。
	AccentPhrases []map[string]interface{} `json:"accent_phrases"`
	// SpeedScale は話速のスケール設定です。
	SpeedScale float64 `json:"speedScale"`
}

// ErrAPINetwork はAPI呼び出しにおける通信エラーやリトライ後の最終失敗を示すカスタムエラー型です。
type ErrAPINetwork struct {
	Endpoint   string
	WrappedErr error
}

func (e *ErrAPINetwork) Error() string {
	return fmt.Sprintf("API通信エラー (%s): %v", e.Endpoint, e.WrappedErr)
}

// ErrAPIResponse はAPIが 4xx や 5xx などの異常なステータスコードを返したことを示します。
type ErrAPIResponse struct {
	Endpoint   string
	StatusCode int
	Body       string
}

func (e *ErrAPIResponse) Error() string {
	// 応答ボディが長すぎる場合は切り詰める
	bodyDisplay := e.Body
	if len(bodyDisplay) > 100 {
		bodyDisplay = bodyDisplay[:100] + "..."
	}
	return fmt.Sprintf("API応答エラー (%s)。ステータスコード %d: %s", e.Endpoint, e.StatusCode, bodyDisplay)
}

// ErrInvalidJSON はAPI応答やデータが期待されるJSON形式でなかったことを示します。
type ErrInvalidJSON struct {
	Details    string
	WrappedErr error
}

func (e *ErrInvalidJSON) Error() string {
	return fmt.Sprintf("不正なJSONデータ: %s (詳細: %v)", e.Details, e.WrappedErr)
}
