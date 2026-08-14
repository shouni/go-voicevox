package api

import "fmt"

// ErrAPINetwork はAPI呼び出しにおける通信エラーやリトライ後の最終失敗を示すカスタムエラー型です。
//
// 4xx / 5xx もここに含まれます。ステータスコードの判定とリトライは
// go-http-kit が担っており、**この層はその最終結果だけを受け取ります。**
// 独自のステータス用エラー型を持っていた時期がありますが、誰も生成しないまま
// 残っていたため削除しました。
type ErrAPINetwork struct {
	Endpoint   string
	WrappedErr error
}

func (e *ErrAPINetwork) Error() string {
	return fmt.Sprintf("API通信エラー (%s): %v", e.Endpoint, e.WrappedErr)
}

// ErrInvalidJSON はAPI応答やデータが期待されるJSON形式でなかったことを示します。
type ErrInvalidJSON struct {
	Details    string
	WrappedErr error
}

func (e *ErrInvalidJSON) Error() string {
	return fmt.Sprintf("不正なJSONデータ: %s (詳細: %v)", e.Details, e.WrappedErr)
}
