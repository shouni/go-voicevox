package engine

import (
	"fmt"
	"strings"
)

// ErrSynthesisBatch は、バッチ 1 回で起きた失敗をまとめて返すエラーです。
//
// 最初の 1 件で止めずに全件を集めるのは、バッチで何が起きたかを一度で見せるためです。
// 1 件目だけ返すと、直して再実行して次の 1 件を知る、を繰り返すことになります。
//
// 中身は文字列ではなくエラーのまま持ちます。以前は `[]string` に潰していたため、
// `errors.Is` / `errors.As` がここで行き止まりになり、呼び出し側は
// 「打ち切られた（context.DeadlineExceeded）」のか「エンジンが落ちている
// （api.ErrAPINetwork）」のかを、メッセージを文字列照合する以外に区別できませんでした。
type ErrSynthesisBatch struct {
	Errors []error
}

func (e *ErrSynthesisBatch) Error() string {
	details := make([]string, len(e.Errors))
	for i, err := range e.Errors {
		details[i] = err.Error()
	}
	return fmt.Sprintf("音声合成バッチ処理中に %d 件のエラーが発生しました:\n- %s",
		len(e.Errors), strings.Join(details, "\n- "))
}

// Unwrap は、まとめた個々のエラーを返します。
//
// 複数エラーを返す Unwrap は errors パッケージが解釈するため、
// これだけで `errors.Is(err, context.DeadlineExceeded)` や
// `errors.As(err, &apiErr)` がバッチ越しに効くようになります。
func (e *ErrSynthesisBatch) Unwrap() []error {
	return e.Errors
}

// newErrSynthesisBatch は、前段と実行時のエラーを 1 つにまとめます。
// 1 件も無ければ nil を返します。
func newErrSynthesisBatch(groups ...[]error) error {
	var all []error
	for _, g := range groups {
		all = append(all, g...)
	}
	if len(all) == 0 {
		return nil
	}
	return &ErrSynthesisBatch{Errors: all}
}
