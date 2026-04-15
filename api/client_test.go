package api

import (
	"context"
	"encoding/binary"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type stubRequester struct {
	doRequestFunc   func(req *http.Request) ([]byte, error)
	fetchBytesFunc  func(ctx context.Context, url string) ([]byte, error)
	lastRequest     *http.Request
	lastFetchTarget string
}

func (s *stubRequester) DoRequest(req *http.Request) ([]byte, error) {
	s.lastRequest = req
	return s.doRequestFunc(req)
}

func (s *stubRequester) FetchBytes(ctx context.Context, target string) ([]byte, error) {
	s.lastFetchTarget = target
	return s.fetchBytesFunc(ctx, target)
}

func (s *stubRequester) FetchAndDecodeJSON(ctx context.Context, url string, v any) error {
	return nil
}

func (s *stubRequester) PostJSONAndFetchBytes(ctx context.Context, url string, data any) ([]byte, error) {
	return nil, nil
}

func (s *stubRequester) PostRawBodyAndFetchBytes(ctx context.Context, url string, body []byte, contentType string) ([]byte, error) {
	return nil, nil
}

func TestRunAudioQueryBuildsRequestAndReturnsBody(t *testing.T) {
	reqer := &stubRequester{
		doRequestFunc: func(req *http.Request) ([]byte, error) {
			return []byte(`{"accent_phrases":[],"speedScale":1.0}`), nil
		},
		fetchBytesFunc: func(ctx context.Context, url string) ([]byte, error) {
			return nil, nil
		},
	}
	client := New(reqer, "http://localhost:50021/api")

	body, err := client.RunAudioQuery(context.Background(), "こんにちは", 7)
	if err != nil {
		t.Fatalf("RunAudioQuery() error = %v", err)
	}
	if string(body) == "" {
		t.Fatal("RunAudioQuery() returned empty body")
	}
	if reqer.lastRequest == nil {
		t.Fatal("DoRequest was not called")
	}
	if reqer.lastRequest.Method != http.MethodPost {
		t.Fatalf("method = %s, want POST", reqer.lastRequest.Method)
	}

	gotURL, err := url.Parse(reqer.lastRequest.URL.String())
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	if gotURL.Path != "/api/audio_query" {
		t.Fatalf("path = %q, want /api/audio_query", gotURL.Path)
	}
	if gotURL.Query().Get("text") != "こんにちは" {
		t.Fatalf("text query = %q", gotURL.Query().Get("text"))
	}
	if gotURL.Query().Get("speaker") != "7" {
		t.Fatalf("speaker query = %q, want 7", gotURL.Query().Get("speaker"))
	}
}

func TestRunAudioQueryReturnsInvalidJSON(t *testing.T) {
	reqer := &stubRequester{
		doRequestFunc: func(req *http.Request) ([]byte, error) {
			return []byte("not-json"), nil
		},
		fetchBytesFunc: func(ctx context.Context, url string) ([]byte, error) {
			return nil, nil
		},
	}
	client := New(reqer, "http://localhost:50021")

	_, err := client.RunAudioQuery(context.Background(), "x", 1)
	if err == nil {
		t.Fatal("RunAudioQuery() error = nil, want invalid json error")
	}
	var invalidJSON *ErrInvalidJSON
	if !errors.As(err, &invalidJSON) {
		t.Fatalf("error type = %T, want *ErrInvalidJSON", err)
	}
}

func TestRunSynthesisRejectsShortWAV(t *testing.T) {
	reqer := &stubRequester{
		doRequestFunc: func(req *http.Request) ([]byte, error) {
			return []byte("short"), nil
		},
		fetchBytesFunc: func(ctx context.Context, url string) ([]byte, error) {
			return nil, nil
		},
	}
	client := New(reqer, "http://localhost:50021")

	_, err := client.RunSynthesis(context.Background(), []byte(`{}`), 3)
	if err == nil {
		t.Fatal("RunSynthesis() error = nil, want invalid wav error")
	}
	var invalidWAV *ErrInvalidWAVHeader
	if !errors.As(err, &invalidWAV) {
		t.Fatalf("error type = %T, want *ErrInvalidWAVHeader", err)
	}
}

func TestGetSpeakersFetchesExpectedEndpoint(t *testing.T) {
	reqer := &stubRequester{
		doRequestFunc: func(req *http.Request) ([]byte, error) {
			return nil, nil
		},
		fetchBytesFunc: func(ctx context.Context, target string) ([]byte, error) {
			return []byte(`[]`), nil
		},
	}
	client := New(reqer, "http://localhost:50021/base")

	body, err := client.GetSpeakers(context.Background())
	if err != nil {
		t.Fatalf("GetSpeakers() error = %v", err)
	}
	if string(body) != "[]" {
		t.Fatalf("body = %q, want []", string(body))
	}
	if reqer.lastFetchTarget != "http://localhost:50021/base/speakers" {
		t.Fatalf("fetch target = %q", reqer.lastFetchTarget)
	}
}

func TestCombineWavDataConcatenatesAudioPayloads(t *testing.T) {
	first := testWAV([]byte{1, 2, 3})
	second := testWAV([]byte{4, 5})

	combined, err := CombineWavData([][]byte{first, second})
	if err != nil {
		t.Fatalf("CombineWavData() error = %v", err)
	}

	if string(combined[:4]) != "RIFF" {
		t.Fatalf("header chunk = %q, want RIFF", string(combined[:4]))
	}
	if !strings.Contains(string(combined[:16]), "WAVE") {
		t.Fatal("combined wav does not contain WAVE header")
	}
	gotAudio := combined[len(combined)-5:]
	wantAudio := []byte{1, 2, 3, 4, 5}
	dataSize := binary.LittleEndian.Uint32(combined[len(combined)-5-4 : len(combined)-5])
	expectedSize := uint32(len(wantAudio))
	if dataSize != expectedSize {
		t.Fatalf("data size = %d, want %d", dataSize, expectedSize)
	}
	if string(gotAudio) != string(wantAudio) {
		t.Fatalf("audio payload = %v, want %v", gotAudio, wantAudio)
	}
}

func TestCombineWavDataReturnsErrorOnEmptyInput(t *testing.T) {
	_, err := CombineWavData(nil)
	if err == nil {
		t.Fatal("CombineWavData() error = nil, want ErrNoAudioData")
	}
	var noAudio *ErrNoAudioData
	if !errors.As(err, &noAudio) {
		t.Fatalf("error type = %T, want *ErrNoAudioData", err)
	}
}

func testWAV(audio []byte) []byte {
	header := make([]byte, 44)
	copy(header[0:], []byte("RIFF"))
	binary.LittleEndian.PutUint32(header[4:], uint32(36+len(audio)))
	copy(header[8:], []byte("WAVE"))
	copy(header[12:], []byte("fmt "))
	binary.LittleEndian.PutUint32(header[16:], 16)
	binary.LittleEndian.PutUint16(header[20:], 1)
	binary.LittleEndian.PutUint16(header[22:], 1)
	binary.LittleEndian.PutUint32(header[24:], 24000)
	binary.LittleEndian.PutUint32(header[28:], 48000)
	binary.LittleEndian.PutUint16(header[32:], 2)
	binary.LittleEndian.PutUint16(header[34:], 16)
	copy(header[36:], []byte("data"))
	binary.LittleEndian.PutUint32(header[40:], uint32(len(audio)))
	return append(header, audio...)
}
