package api

import (
	"context"
	"errors"
	"testing"
)

// TestErrorsUnwrapToCause は、API エラーが原因まで辿れることを検証します。
//
// **打ち切りの判別に効きます。** 合成中に ctx が切れると context.DeadlineExceeded が
// ErrAPINetwork に包まれるため、Unwrap が無いと呼び出し側は時間切れと
// エンジン障害を区別できません。
func TestErrorsUnwrapToCause(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{name: "通信エラー", err: &ErrAPINetwork{Endpoint: "/synthesis", WrappedErr: context.DeadlineExceeded}},
		{name: "JSONエラー", err: &ErrInvalidJSON{Details: "応答", WrappedErr: context.DeadlineExceeded}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if !errors.Is(tt.err, context.DeadlineExceeded) {
				t.Errorf("errors.Is(%v, context.DeadlineExceeded) = false", tt.err)
			}
		})
	}
}
