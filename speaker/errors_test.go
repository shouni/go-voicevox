package speaker

import (
	"encoding/json"
	"errors"
	"testing"
)

// TestNewRegistryReportsDecodeFailure verifies that a broken /speakers response becomes an
// error whose cause is readable.
//
// Decode failures used to be restated as ErrMissingRequiredField (a required field is
// missing), so what actually happened (the JSON is broken) could not be read.
func TestNewRegistryReportsDecodeFailure(t *testing.T) {
	t.Parallel()

	_, err := NewRegistry([]byte(`{"これは配列ではありません":true}`))
	if err == nil {
		t.Fatal("a broken response passed through")
	}

	if _, ok := errors.AsType[*ErrInvalidPayload](err); !ok {
		t.Fatalf("error type = %T, want *ErrInvalidPayload", err)
	}
	// The wrapped decode error must be reachable.
	if _, ok := errors.AsType[*json.UnmarshalTypeError](err); !ok {
		t.Errorf("the decode error is not reachable: %v", err)
	}
}
