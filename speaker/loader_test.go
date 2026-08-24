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

	// タグは VOICEVOX の表記そのままで組みます。
	if got, ok := data.GetStyleID("[話者アルファ][甘め]"); !ok || got != 3 {
		t.Fatalf("GetStyleID([話者アルファ][甘め]) = (%d, %v), want (3, true)", got, ok)
	}
	if got, ok := data.GetDefaultTag("[話者ベータ]"); !ok || got != "[話者ベータ][標準]" {
		t.Fatalf("GetDefaultTag([話者ベータ]) = (%q, %v)", got, ok)
	}

	// speakers.json に無い話者は組みません。
	if _, ok := data.GetStyleID("[一覧に無い話者][標準]"); ok {
		t.Fatal("speakers.json に無い話者が組まれている")
	}

	// エンジンが返さなかったスタイルも組みません。
	if _, ok := data.GetStyleID("[話者アルファ][小声]"); ok {
		t.Fatal("エンジンが返していないスタイルが組まれている")
	}
}

// 埋め込みの一覧がエンジンより新しいことは普通に起こります。既定スタイルを
// 返さないエンジンでも、実際に組めたスタイルの先頭をフォールバック先にします。
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

// 1人も組めなければ、以降のセグメントは全滅します。合成を始める前に止めます。
func TestLoadStylesReturnsErrorWhenNoSpeakerMatches(t *testing.T) {
	client := stubSpeakerClient{
		body: []byte(`[{"name":"一覧に無い話者","styles":[{"name":"標準","id":99}]}]`),
	}

	_, err := testRegistry(t).LoadStyles(context.Background(), client)
	if err == nil {
		t.Fatal("LoadStyles() error = nil, want missing field error")
	}
	var missing *ErrMissingRequiredField
	if !errors.As(err, &missing) {
		t.Fatalf("error type = %T, want *ErrMissingRequiredField", err)
	}
}

// 歌唱系のスタイルは /synthesis で使えないため、エンジンが返しても組みません。
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
		t.Fatal("歌唱スタイルが組まれている")
	}
}

func TestLoadStylesReturnsInvalidJSON(t *testing.T) {
	client := stubSpeakerClient{body: []byte("{")}

	_, err := testRegistry(t).LoadStyles(context.Background(), client)
	if err == nil {
		t.Fatal("LoadStyles() error = nil, want invalid JSON error")
	}
}

// 一覧が nil なら絞り込みません。エンジンが返した話者をそのまま受け入れます。
//
// **nil のレシーバで呼べることが前提です。** voicevox.New は一覧を省略した
// 呼び出し側からそのまま nil を渡します。
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
