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
			{"name":"春日部つむぎ","styles":[{"name":"ノーマル","id":8}]},
			{"name":"知らないキャラ","styles":[{"name":"ノーマル","id":99}]}
		]`),
	}

	data, err := LoadSpeakers(context.Background(), client, testRegistry(t))
	if err != nil {
		t.Fatalf("LoadSpeakers() error = %v", err)
	}

	// タグは VOICEVOX の表記そのままで組みます。
	if got, ok := data.GetStyleID("[四国めたん][あまあま]"); !ok || got != 3 {
		t.Fatalf("GetStyleID([四国めたん][あまあま]) = (%d, %v), want (3, true)", got, ok)
	}
	if got, ok := data.GetDefaultTag("[ずんだもん]"); !ok || got != "[ずんだもん][ノーマル]" {
		t.Fatalf("GetDefaultTag([ずんだもん]) = (%q, %v)", got, ok)
	}

	// speakers.json に無い話者は組みません。
	if _, ok := data.GetStyleID("[知らないキャラ][ノーマル]"); ok {
		t.Fatal("speakers.json に無い話者が組まれている")
	}

	// エンジンが返さなかったスタイルも組みません（めたんの ヒソヒソ など）。
	if _, ok := data.GetStyleID("[四国めたん][ヒソヒソ]"); ok {
		t.Fatal("エンジンが返していないスタイルが組まれている")
	}
}

// 埋め込みの一覧がエンジンより新しいことは普通に起こります。既定スタイルを
// 返さないエンジンでも、実際に組めたスタイルの先頭をフォールバック先にします。
func TestLoadSpeakersFallsBackToAvailableStyle(t *testing.T) {
	client := stubSpeakerClient{
		body: []byte(`[
			{"name":"四国めたん","styles":[{"name":"あまあま","id":3}]}
		]`),
	}

	data, err := LoadSpeakers(context.Background(), client, testRegistry(t))
	if err != nil {
		t.Fatalf("LoadSpeakers() error = %v", err)
	}

	if got, ok := data.GetDefaultTag("[四国めたん]"); !ok || got != "[四国めたん][あまあま]" {
		t.Fatalf("GetDefaultTag([四国めたん]) = (%q, %v), want [四国めたん][あまあま]", got, ok)
	}
}

// 1人も組めなければ、以降のセグメントは全滅します。合成を始める前に止めます。
func TestLoadSpeakersReturnsErrorWhenNoSpeakerMatches(t *testing.T) {
	client := stubSpeakerClient{
		body: []byte(`[{"name":"知らないキャラ","styles":[{"name":"ノーマル","id":99}]}]`),
	}

	_, err := LoadSpeakers(context.Background(), client, testRegistry(t))
	if err == nil {
		t.Fatal("LoadSpeakers() error = nil, want missing field error")
	}
	var missing *ErrMissingRequiredField
	if !errors.As(err, &missing) {
		t.Fatalf("error type = %T, want *ErrMissingRequiredField", err)
	}
}

// 歌唱系のスタイルは /synthesis で使えないため、エンジンが返しても組みません。
func TestLoadSpeakersSkipsNonTalkStyles(t *testing.T) {
	client := stubSpeakerClient{
		body: []byte(`[
			{"name":"四国めたん","styles":[
				{"name":"ノーマル","id":2,"type":"talk"},
				{"name":"あまあま","id":3,"type":"sing"}
			]}
		]`),
	}

	data, err := LoadSpeakers(context.Background(), client, testRegistry(t))
	if err != nil {
		t.Fatalf("LoadSpeakers() error = %v", err)
	}
	if _, ok := data.GetStyleID("[四国めたん][あまあま]"); ok {
		t.Fatal("歌唱スタイルが組まれている")
	}
}

func TestLoadSpeakersReturnsInvalidJSON(t *testing.T) {
	client := stubSpeakerClient{body: []byte("{")}

	_, err := LoadSpeakers(context.Background(), client, testRegistry(t))
	if err == nil {
		t.Fatal("LoadSpeakers() error = nil, want invalid JSON error")
	}
}
