package api

import (
	"context"
	"errors"
	"testing"
)

// TestErrorsUnwrapToCause verifies that API errors can be followed down to their cause.
//
// This is what tells a cutoff apart: when ctx expires mid-synthesis, context.DeadlineExceeded
// is wrapped in ErrAPINetwork, so without Unwrap the caller cannot distinguish a timeout from
// an engine failure.
func TestErrorsUnwrapToCause(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{name: "通信エラー", err: &ErrAPINetwork{Endpoint: "/synthesis", WrappedErr: context.DeadlineExceeded}},
		{name: "JSONエラー", err: &ErrInvalidJSON{Details: "response", WrappedErr: context.DeadlineExceeded}},
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
