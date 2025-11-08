package api

import (
	"fmt"
)

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

// ErrInvalidWAVHeader はWAVデータが短すぎる、またはヘッダーの記載とデータ長が一致しないなど、
// ヘッダーに問題があることを示します。
type ErrInvalidWAVHeader struct {
	Index   int // エラーが発生したWAVセグメントのインデックス
	Details string
}

func (e *ErrInvalidWAVHeader) Error() string {
	if e.Index >= 0 {
		return fmt.Sprintf("WAVデータ #%d のヘッダーが無効です: %s", e.Index, e.Details)
	}
	return fmt.Sprintf("WAVデータ結合時のエラー: %s", e.Details)
}
