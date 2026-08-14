package speaker

import "fmt"

// ErrInvalidPayload は /speakers 応答が期待する形になっていないことを示します。
//
// **この型がここにあるのは、speaker が公開パッケージだからです。** 以前は
// internal 側の JSON エラーをそのまま返しており、呼び出し側は型を名指しできないため
// errors.As で判別できませんでした。デコードの失敗を「必須フィールドが無い」と
// 言い換えてもいましたが、原因が読めなくなるだけでした。
type ErrInvalidPayload struct {
	// Context は、どの入力を読もうとしたかです。
	Context    string
	WrappedErr error
}

func (e *ErrInvalidPayload) Error() string {
	return fmt.Sprintf("%sの解釈に失敗しました: %v", e.Context, e.WrappedErr)
}

// Unwrap は、包んだデコードエラーを返します。
func (e *ErrInvalidPayload) Unwrap() error { return e.WrappedErr }

// ErrMissingRequiredField は外部API応答に必要なフィールド（この場合はスタイル）が見つからないことを示します。
type ErrMissingRequiredField struct {
	Field   string
	Context string // 例: "話者データロード時"
}

func (e *ErrMissingRequiredField) Error() string {
	return fmt.Sprintf("%sで必須フィールド '%s' が見つかりません", e.Context, e.Field)
}
