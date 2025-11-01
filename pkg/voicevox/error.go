package voicevox

import (
	"fmt"
	"strings"
)

// ----------------------------------------------------------------------
// API 通信・応答エラー (client.go で利用)
// ----------------------------------------------------------------------

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

// ----------------------------------------------------------------------
// データ処理エラー (audio.go, speaker_data.go で利用)
// ----------------------------------------------------------------------

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

// ErrMissingRequiredField は外部API応答に必要なフィールドが見つからないことを示します。
type ErrMissingRequiredField struct {
	Field   string
	Context string // 例: "話者データロード時"
}

func (e *ErrMissingRequiredField) Error() string {
	return fmt.Sprintf("%sで必須フィールド '%s' が見つかりません", e.Context, e.Field)
}

// ErrNoAudioData は結合すべきWAVデータがないか、セグメントから抽出されたデータがゼロサイズであることを示します。
type ErrNoAudioData struct{}

func (e *ErrNoAudioData) Error() string {
	return "処理対象となる有効なオーディオデータ（WAVファイル、または抽出されたデータ）がありません"
}

// ----------------------------------------------------------------------
// バッチ処理エラー (engine.go で利用)
// ----------------------------------------------------------------------

// ErrSynthesisBatch は音声合成処理のバッチ全体で発生した複数のエラーをラップするカスタムエラー型です。
// 事前計算エラーと実行時エラーの両方をまとめて呼び出し元に返します。
type ErrSynthesisBatch struct {
	TotalErrors int
	Details     []string
}

func (e *ErrSynthesisBatch) Error() string {
	return fmt.Sprintf("音声合成バッチ処理中に %d 件のエラーが発生しました:\n- %s",
		e.TotalErrors, strings.Join(e.Details, "\n- "))
}
