package speaker

import "fmt"

// ErrInvalidPayload reports that a /speakers response is not in the expected shape.
//
// This type lives here because speaker is a public package. It used to return the internal
// JSON error as is, and since the caller cannot name that type, errors.As could not identify
// it. It also restated decode failures as "a required field is missing", which only hid the
// cause.
type ErrInvalidPayload struct {
	// Context is the input that was being read.
	Context    string
	WrappedErr error
}

func (e *ErrInvalidPayload) Error() string {
	return fmt.Sprintf("parse %s: %v", e.Context, e.WrappedErr)
}

// Unwrap returns the wrapped decode error.
func (e *ErrInvalidPayload) Unwrap() error { return e.WrappedErr }

// ErrMissingRequiredField reports that a field required in an external API response (here,
// a style) is missing.
type ErrMissingRequiredField struct {
	Field   string
	Context string // e.g. "no speaker in the list"
}

func (e *ErrMissingRequiredField) Error() string {
	return fmt.Sprintf("required field '%s' missing: %s", e.Field, e.Context)
}
