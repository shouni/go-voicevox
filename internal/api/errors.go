package api

import (
	"errors"
	"fmt"
)

// ErrAPINetwork is a custom error type for a transport failure in an API call, or for the
// final failure after retries.
//
// 4xx / 5xx responses are included. Status-code handling and retries are go-http-kit's job,
// and this layer only receives the final outcome. There was once a dedicated status error
// type, but nothing ever created one, so it was removed.
type ErrAPINetwork struct {
	Endpoint   string
	WrappedErr error
}

func (e *ErrAPINetwork) Error() string {
	return fmt.Sprintf("API network error (%s): %v", e.Endpoint, e.WrappedErr)
}

// Unwrap returns the wrapped error.
//
// Without it errors.Is stops here. When a request fails during a cutoff,
// context.DeadlineExceeded is wrapped in this type, so the caller could not tell
// "timed out" apart from "the engine is down".
func (e *ErrAPINetwork) Unwrap() error { return e.WrappedErr }

// ErrInvalidJSON reports that an API response or data was not in the expected JSON form.
type ErrInvalidJSON struct {
	Details    string
	WrappedErr error
}

func (e *ErrInvalidJSON) Error() string {
	return fmt.Sprintf("invalid JSON data: %s (details: %v)", e.Details, e.WrappedErr)
}

// Unwrap returns the wrapped error.
func (e *ErrInvalidJSON) Unwrap() error { return e.WrappedErr }

// errNotJSON is the cause error for a response that could not be read as JSON.
// json.Valid gives no reason, so this is what gets wrapped.
var errNotJSON = errors.New("not valid JSON")
