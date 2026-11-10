// Package voicevox is the public API for speech synthesis with the VOICEVOX engine.
// It re-exports types from the internal packages so that callers need not deal with internal.
package voicevox

import (
	"context"
	"net/http"

	internalengine "github.com/shouni/go-voicevox/internal/engine"
)

// Requester is the HTTP client New uses to talk to the VOICEVOX engine.
//
// This interface is also declared here, by its consumer. It used to name go-http-kit's
// interface directly, but a public signature that demands a foreign type forces every caller
// to take on that dependency just to satisfy it. Only the two methods below are used, and
// httpkit.Client satisfies them as is (internal/api declares an interface of the same shape
// itself; changing only one of them breaks compilation where it is passed to api.New).
type Requester interface {
	// SendBytes executes a prepared request and returns the response body.
	SendBytes(req *http.Request) ([]byte, error)
	// GetBytes fetches the response body from a URL.
	GetBytes(ctx context.Context, url string) ([]byte, error)
}

// Engine is the synthesis engine that turns script lines into one combined WAV.
//
// This interface is declared here, by its consumer. The implementation is in internal/engine,
// but satisfying it is structural, so that package need not know this type.
type Engine interface {
	// Run takes structured ScriptLines and returns the combined WAV bytes.
	// Writing them anywhere is the caller's responsibility.
	Run(ctx context.Context, lines []ScriptLine) ([]byte, error)
}

// ScriptLine is one line of input: speaker, style and text.
//
// Only what the public API can actually touch is exported. The engine's dependency
// interfaces, such as AudioQueryClient / StyleFinder, are not re-exported: New builds its
// own client, so there is no way to substitute one, and listing them would wrongly suggest
// that implementing them lets you swap them in. The internal segment type is likewise
// unexported; assembling tags is an internal job, and this type is the way in.
//
// The default constants are not exported either. DefaultMaxParallelSegments and friends used
// to be listed, but a caller that wants the default simply passes no option, so
// WithX(default) was only ever a call that did nothing. The actual defaults are in each
// option's doc comment (options.go).
type ScriptLine = internalengine.ScriptLine
