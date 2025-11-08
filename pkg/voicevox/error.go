package voicevox

import (
	"fmt"
	"strings"
)

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
