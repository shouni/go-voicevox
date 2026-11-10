// Package api is the HTTP API client for the VOICEVOX engine.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/shouni/audio/wav"
)

// Requester is the interface Client uses for HTTP.
//
// It is declared here, by its consumer. The implementation is go-http-kit's httpkit.Client,
// with retries and error handling, but this layer does not import it. That package exposes
// interfaces split by use, while only two methods are called here.
type Requester interface {
	// SendBytes executes a prepared request and returns the response body.
	SendBytes(req *http.Request) ([]byte, error)
	// GetBytes fetches the response body from a URL.
	GetBytes(ctx context.Context, url string) ([]byte, error)
}

// Client handles API requests to the VOICEVOX engine.
// It communicates through a Requester, which provides retries and error handling.
type Client struct {
	client Requester
	apiURL string
}

// New returns a Client that uses the given HTTP requester and API base URL.
func New(client Requester, apiURL string) *Client {
	return &Client{
		client: client,
		apiURL: apiURL,
	}
}

// buildURL joins the base URL and the endpoint and returns the parsed URL.
func (c *Client) buildURL(endpoint string) (*url.URL, error) {
	u, err := url.Parse(c.apiURL)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: fmt.Errorf("parse API URL: %w", err)}
	}

	u.Path, err = url.JoinPath(u.Path, endpoint)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: fmt.Errorf("join endpoint: %w", err)}
	}

	return u, nil
}

// RunAudioQuery calls the /audio_query API and returns the query JSON needed for synthesis.
// text is the string to synthesize and styleID is the speaker's style ID.
func (c *Client) RunAudioQuery(ctx context.Context, text string, styleID int) ([]byte, error) {
	const endpoint = "/audio_query"

	u, err := c.buildURL(endpoint)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("text", text)
	q.Set("speaker", strconv.Itoa(styleID))
	u.RawQuery = q.Encode()
	finalURL := u.String()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, finalURL, nil)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: fmt.Errorf("build request: %w", err)}
	}

	bodyBytes, err := c.client.SendBytes(req)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: err}
	}

	// The content is not read; only that it parses as JSON is checked. These bytes go
	// straight to /synthesis, so this side has no need to know their structure. It used to
	// decode into a type mirroring part of the response, but nothing read those values, and
	// the type only suggested, wrongly, that the response was being interpreted here.
	if !json.Valid(bodyBytes) {
		return nil, &ErrInvalidJSON{Details: endpoint + " response", WrappedErr: errNotJSON}
	}

	return bodyBytes, nil
}

// RunSynthesis calls the /synthesis API and returns the audio as WAV.
// queryBody is the query JSON obtained from RunAudioQuery, and styleID is the speaker's
// style ID.
func (c *Client) RunSynthesis(ctx context.Context, queryBody []byte, styleID int) ([]byte, error) {
	const endpoint = "/synthesis"

	u, err := c.buildURL(endpoint)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("speaker", strconv.Itoa(styleID))
	u.RawQuery = q.Encode()
	finalURL := u.String()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, finalURL, bytes.NewReader(queryBody))
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: fmt.Errorf("build request: %w", err)}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/wav")

	wavData, err := c.client.SendBytes(req)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: err}
	}

	// audio v1.4.0 made wav.TotalHeaderSize unexported. wav.Inspect states the intent better
	// than computing lengths by hand, and it checks more (it looks at the RIFF/WAVE identifiers
	// and the fmt/data chunks, so a broken response that merely has 44 bytes or more is
	// rejected too). Inspect is the same validation CombineWavData runs, so what passes here
	// will not fail at combine time.
	if _, err := wav.Inspect(wavData); err != nil {
		return nil, &wav.ErrInvalidWAVHeader{
			Index:   -1,
			Details: fmt.Sprintf("cannot parse WAV data (%d bytes): %v", len(wavData), err),
		}
	}

	return wavData, nil
}

// GetSpeakers calls the /speakers API and returns every speaker the VOICEVOX engine
// provides, as JSON bytes.
func (c *Client) GetSpeakers(ctx context.Context) ([]byte, error) {
	const endpoint = "/speakers"

	u, err := c.buildURL(endpoint)
	if err != nil {
		return nil, err
	}
	speakersURL := u.String()

	bodyBytes, err := c.client.GetBytes(ctx, speakersURL)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: err}
	}

	return bodyBytes, nil
}
