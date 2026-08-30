package speaker

import (
	"encoding/json"
	"errors"
	"testing"
)

// TestNewRegistryReportsDecodeFailure は、壊れた /speakers 応答が
// 原因の読めるエラーになることを検証します。
//
// 以前はデコードの失敗を ErrMissingRequiredField（必須フィールドが無い）と
// 言い換えていたため、実際に起きたこと（JSON が壊れている）が読めませんでした。
func TestNewRegistryReportsDecodeFailure(t *testing.T) {
	t.Parallel()

	_, err := NewRegistry([]byte(`{"これは配列ではありません":true}`))
	if err == nil {
		t.Fatal("壊れた応答が素通りしました")
	}

	if _, ok := errors.AsType[*ErrInvalidPayload](err); !ok {
		t.Fatalf("error type = %T, want *ErrInvalidPayload", err)
	}
	// 包んだデコードエラーまで辿れること。
	if _, ok := errors.AsType[*json.UnmarshalTypeError](err); !ok {
		t.Errorf("デコードエラーまで辿れません: %v", err)
	}
}
