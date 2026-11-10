package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// requester is a minimal implementation of voicevox.Requester using net/http alone.
//
// The library calls only these two methods, so this is enough for a demo talking to a
// localhost engine. A production caller can pass its own client with retries and error
// handling as is; the library knows nothing beyond this interface.
type requester struct {
	client *http.Client
}

func newRequester(timeout time.Duration) *requester {
	return &requester{client: &http.Client{Timeout: timeout}}
}

// SendBytes executes a prepared request and returns the response body.
func (r *requester) SendBytes(req *http.Request) ([]byte, error) {
	return r.do(req)
}

// GetBytes fetches the response body from a URL.
func (r *requester) GetBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	return r.do(req)
}

// do is the part the two methods share. Status codes are checked here.
func (r *requester) do(req *http.Request) ([]byte, error) {
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%s returned %s: %s", req.URL.Path, resp.Status, body)
	}

	return body, nil
}
