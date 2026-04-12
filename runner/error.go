package runner

import (
	"fmt"
	"strings"
)

// ErrSynthesisBatch は音声合成処理のバッチ全体で発生した複数のエラーをラップするカスタムエラー型です。
//
// スクリプトの解析時に発生した事前計算エラー（Style ID の欠如など）や、
// API 呼び出し中に発生した実行時エラーをすべて集約して保持します。
type ErrSynthesisBatch struct {
	// TotalErrors はバッチ処理中に発生したエラーの総数です。
	TotalErrors int
	// Details は発生した各エラーの詳細なメッセージリストです。
	Details []string
}

// Error は error インターフェースの実装です。
// 発生したエラーの件数と、それぞれの詳細な内容を整形した文字列として返します。
func (e *ErrSynthesisBatch) Error() string {
	return fmt.Sprintf("音声合成バッチ処理中に %d 件のエラーが発生しました:\n- %s",
		e.TotalErrors, strings.Join(e.Details, "\n- "))
}
