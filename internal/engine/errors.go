package engine

import (
	"fmt"
	"strings"
)

// ErrSynthesisBatch collects every failure from one batch into a single error.
//
// It gathers all of them instead of stopping at the first so that one look shows what
// happened in the batch. Returning only the first means fixing, rerunning and learning about
// the next one, over and over.
//
// The contents are kept as errors, not strings. They used to be flattened to `[]string`, so
// `errors.Is` / `errors.As` dead-ended here, and the caller had no way other than matching
// message text to tell "cut off (context.DeadlineExceeded)" from "the engine is down
// (api.ErrAPINetwork)".
type ErrSynthesisBatch struct {
	Errors []error
}

func (e *ErrSynthesisBatch) Error() string {
	details := make([]string, len(e.Errors))
	for i, err := range e.Errors {
		details[i] = err.Error()
	}
	return fmt.Sprintf("%d errors in the synthesis batch:\n- %s",
		len(e.Errors), strings.Join(details, "\n- "))
}

// Unwrap returns the individual errors collected.
//
// The errors package understands an Unwrap that returns several errors, so this alone makes
// `errors.Is(err, context.DeadlineExceeded)` and `errors.As(err, &apiErr)` work through the
// batch.
func (e *ErrSynthesisBatch) Unwrap() []error {
	return e.Errors
}

// newErrSynthesisBatch combines the preparation-time and runtime errors into one.
// It returns nil when there are none.
func newErrSynthesisBatch(groups ...[]error) error {
	var all []error
	for _, g := range groups {
		all = append(all, g...)
	}
	if len(all) == 0 {
		return nil
	}
	return &ErrSynthesisBatch{Errors: all}
}
