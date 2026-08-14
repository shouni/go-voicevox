package speaker

import (
	"encoding/json"
	"errors"
	"testing"
)

// TestNewRegistryReportsDecodeFailure は、壊れた /speakers 応答が
// **原因の読めるエラー**になることを検証します。
//
// 以前はデコードの失敗を ErrMissingRequiredField（必須フィールドが無い）と
// 言い換えていたため、実際に起きたこと（JSON が壊れている）が読めませんでした。
func TestNewRegistryReportsDecodeFailure(t *testing.T) {
	t.Parallel()

	_, err := NewRegistry([]byte(`{"これは配列ではありません":true}`))
	if err == nil {
		t.Fatal("壊れた応答が素通りしました")
	}

	var invalid *ErrInvalidPayload
	if !errors.As(err, &invalid) {
		t.Fatalf("error type = %T, want *ErrInvalidPayload", err)
	}
	// 包んだデコードエラーまで辿れること。
	var syntax *json.UnmarshalTypeError
	if !errors.As(err, &syntax) {
		t.Errorf("デコードエラーまで辿れません: %v", err)
	}
}
