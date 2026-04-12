package speaker

import "fmt"

// ErrMissingRequiredField は外部API応答に必要なフィールド（この場合はスタイル）が見つからないことを示します。
type ErrMissingRequiredField struct {
	Field   string
	Context string // 例: "話者データロード時"
}

func (e *ErrMissingRequiredField) Error() string {
	return fmt.Sprintf("%sで必須フィールド '%s' が見つかりません", e.Context, e.Field)
}
