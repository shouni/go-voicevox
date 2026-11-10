package speaker

import (
	"context"
	"errors"
	"testing"
)

type stubSpeakerClient struct {
	body []byte
	err  error
}

func (s stubSpeakerClient) GetSpeakers(_ context.Context) ([]byte, error) {
	return s.body, s.err
}

func TestLoadStylesBuildsStyleMaps(t *testing.T) {
	client := stubSpeakerClient{
		body: []byte(`[
			{"name":"話者アルファ","styles":[{"name":"標準","id":2},{"name":"甘め","id":3}]},
			{"name":"話者ベータ","styles":[{"name":"標準","id":1},{"name":"囁き","id":5}]},
			{"name":"話者ガンマ","styles":[{"name":"標準","id":8}]},
			{"name":"一覧に無い話者","styles":[{"name":"標準","id":99}]}
		]`),
	}

	data, err := testRegistry(t).LoadStyles(context.Background(), client)
	if err != nil {
		t.Fatalf("LoadStyles() error = %v", err)
	}

	// Tags are built from the VOICEVOX spelling as is.
	if got, ok := data.GetStyleID("[話者アルファ][甘め]"); !ok || got != 3 {
		t.Fatalf("GetStyleID([話者アルファ][甘め]) = (%d, %v), want (3, true)", got, ok)
	}
	if got, ok := data.GetDefaultTag("[話者ベータ]"); !ok || got != "[話者ベータ][標準]" {
		t.Fatalf("GetDefaultTag([話者ベータ]) = (%q, %v)", got, ok)
	}

	// A speaker not in speakers.json is not built.
	if _, ok := data.GetStyleID("[一覧に無い話者][標準]"); ok {
		t.Fatal("a speaker not in speakers.json was built")
	}

	// Nor is a style the engine did not return.
	if _, ok := data.GetStyleID("[話者アルファ][小声]"); ok {
		t.Fatal("a style the engine did not return was built")
	}
}

// An embedded list being newer than the engine is normal. Even when the engine does not return
// the default style, the first style actually built becomes the fallback.
func TestLoadStylesFallsBackToAvailableStyle(t *testing.T) {
	client := stubSpeakerClient{
		body: []byte(`[
			{"name":"話者アルファ","styles":[{"name":"甘め","id":3}]}
		]`),
	}

	data, err := testRegistry(t).LoadStyles(context.Background(), client)
	if err != nil {
		t.Fatalf("LoadStyles() error = %v", err)
	}

	if got, ok := data.GetDefaultTag("[話者アルファ]"); !ok || got != "[話者アルファ][甘め]" {
		t.Fatalf("GetDefaultTag([話者アルファ]) = (%q, %v), want [話者アルファ][甘め]", got, ok)
	}
}

// If no speaker could be built, every later segment would fail. Stop before synthesis starts.
func TestLoadStylesReturnsErrorWhenNoSpeakerMatches(t *testing.T) {
	client := stubSpeakerClient{
		body: []byte(`[{"name":"一覧に無い話者","styles":[{"name":"標準","id":99}]}]`),
	}

	_, err := testRegistry(t).LoadStyles(context.Background(), client)
	if err == nil {
		t.Fatal("LoadStyles() error = nil, want missing field error")
	}
	if _, ok := errors.AsType[*ErrMissingRequiredField](err); !ok {
		t.Fatalf("error type = %T, want *ErrMissingRequiredField", err)
	}
}

// Singing styles cannot be used with /synthesis, so they are not built even when the engine
// returns them.
func TestLoadStylesSkipsNonTalkStyles(t *testing.T) {
	client := stubSpeakerClient{
		body: []byte(`[
			{"name":"話者アルファ","styles":[
				{"name":"標準","id":2,"type":"talk"},
				{"name":"甘め","id":3,"type":"sing"}
			]}
		]`),
	}

	data, err := testRegistry(t).LoadStyles(context.Background(), client)
	if err != nil {
		t.Fatalf("LoadStyles() error = %v", err)
	}
	if _, ok := data.GetStyleID("[話者アルファ][甘め]"); ok {
		t.Fatal("a singing style was built")
	}
}

func TestLoadStylesReturnsInvalidJSON(t *testing.T) {
	client := stubSpeakerClient{body: []byte("{")}

	_, err := testRegistry(t).LoadStyles(context.Background(), client)
	if err == nil {
		t.Fatal("LoadStyles() error = nil, want invalid JSON error")
	}
}

// A nil list applies no filter; the speakers the engine returned are accepted as is.
//
// This relies on calling with a nil receiver: voicevox.New passes nil straight through from a
// caller that omitted the list.
func TestLoadStylesWithoutRegistryAcceptsEveryone(t *testing.T) {
	client := stubSpeakerClient{
		body: []byte(`[{"name":"一覧に無い話者","styles":[{"name":"標準","id":99}]}]`),
	}

	var reg *Registry
	styles, err := reg.LoadStyles(context.Background(), client)
	if err != nil {
		t.Fatalf("LoadStyles() error = %v", err)
	}
	if got, ok := styles.GetStyleID("[一覧に無い話者][標準]"); !ok || got != 99 {
		t.Fatalf("GetStyleID = (%d, %v), want (99, true)", got, ok)
	}
}
