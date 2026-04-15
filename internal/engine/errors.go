package engine

import (
	"fmt"
	"strings"
)

// ErrSynthesisBatch は音声合成処理のバッチ全体で発生した複数のエラーをラップするカスタムエラー型です。
type ErrSynthesisBatch struct {
	TotalErrors int
	Details     []string
}

func (e *ErrSynthesisBatch) Error() string {
	return fmt.Sprintf("音声合成バッチ処理中に %d 件のエラーが発生しました:\n- %s",
		e.TotalErrors, strings.Join(e.Details, "\n- "))
}
