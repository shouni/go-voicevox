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

func TestLoadSpeakersBuildsStyleMaps(t *testing.T) {
	client := stubSpeakerClient{
		body: []byte(`[
			{"name":"四国めたん","styles":[{"name":"ノーマル","id":2},{"name":"あまあま","id":3}]},
			{"name":"ずんだもん","styles":[{"name":"ノーマル","id":1},{"name":"ささやき","id":5}]},
			{"name":"未対応キャラ","styles":[{"name":"ノーマル","id":99}]}
		]`),
	}

	data, err := LoadSpeakers(context.Background(), client)
	if err != nil {
		t.Fatalf("LoadSpeakers() error = %v", err)
	}
	if got, ok := data.GetStyleID("[めたん][あまあま]"); !ok || got != 3 {
		t.Fatalf("GetStyleID([めたん][あまあま]) = (%d, %v), want (3, true)", got, ok)
	}
	if got, ok := data.GetDefaultTag("[ずんだもん]"); !ok || got != "[ずんだもん][ノーマル]" {
		t.Fatalf("GetDefaultTag([ずんだもん]) = (%q, %v)", got, ok)
	}
	if _, ok := data.GetStyleID("[未対応キャラ][ノーマル]"); ok {
		t.Fatal("unsupported speaker should be ignored")
	}
}

func TestLoadSpeakersReturnsErrorWhenDefaultStyleMissing(t *testing.T) {
	client := stubSpeakerClient{
		body: []byte(`[
			{"name":"四国めたん","styles":[{"name":"あまあま","id":3}]},
			{"name":"ずんだもん","styles":[{"name":"ノーマル","id":1}]}
		]`),
	}

	_, err := LoadSpeakers(context.Background(), client)
	if err == nil {
		t.Fatal("LoadSpeakers() error = nil, want missing field error")
	}
	var missing *ErrMissingRequiredField
	if !errors.As(err, &missing) {
		t.Fatalf("error type = %T, want *ErrMissingRequiredField", err)
	}
}

func TestLoadSpeakersReturnsInvalidJSON(t *testing.T) {
	client := stubSpeakerClient{body: []byte("{")}

	_, err := LoadSpeakers(context.Background(), client)
	if err == nil {
		t.Fatal("LoadSpeakers() error = nil, want invalid JSON error")
	}
}
