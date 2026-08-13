package speaker

import (
	"reflect"
	"slices"
	"testing"
)

// testRegistryJSON は /speakers 応答を切り詰めたものです。
// ノーマルを持たない話者（白上虎太郎）を含めてあります。
const testRegistryJSON = `[
	{"name":"四国めたん","styles":[
		{"name":"ノーマル","id":2,"type":"talk"},
		{"name":"あまあま","id":0,"type":"talk"},
		{"name":"ヒソヒソ","id":37,"type":"talk"}
	]},
	{"name":"ずんだもん","styles":[
		{"name":"ノーマル","id":3,"type":"talk"},
		{"name":"ささやき","id":22,"type":"talk"},
		{"name":"なみだめ","id":76,"type":"talk"}
	]},
	{"name":"春日部つむぎ","styles":[{"name":"ノーマル","id":8,"type":"talk"}]},
	{"name":"白上虎太郎","styles":[
		{"name":"ふつう","id":12,"type":"talk"},
		{"name":"わーい","id":32,"type":"talk"}
	]}
]`

func testRegistry(t *testing.T) *Registry {
	t.Helper()

	reg, err := NewRegistry([]byte(testRegistryJSON))
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	return reg
}

// 話者名は VOICEVOX の表記そのままです。短縮しません。
func TestRegistrySpeakerNames(t *testing.T) {
	got := testRegistry(t).SpeakerNames()
	want := []string{"四国めたん", "ずんだもん", "春日部つむぎ", "白上虎太郎"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SpeakerNames() = %v, want %v", got, want)
	}
}

// 返すのは「いずれかの話者が持つ」スタイルの和集合です。宣言順を保ち、重複は畳みます。
func TestRegistryStyleNames(t *testing.T) {
	got := testRegistry(t).StyleNames()
	want := []string{"ノーマル", "あまあま", "ヒソヒソ", "ささやき", "なみだめ", "ふつう", "わーい"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("StyleNames() = %v, want %v", got, want)
	}
}

// 話者ごとの組み合わせが取れることが、生の /speakers を扱う狙いです。
func TestRegistryStylesFor(t *testing.T) {
	reg := testRegistry(t)

	t.Run("つむぎはノーマルだけ", func(t *testing.T) {
		got, ok := reg.StylesFor("春日部つむぎ")
		if !ok {
			t.Fatal("春日部つむぎ が見つからない")
		}
		if !reflect.DeepEqual(got, []string{"ノーマル"}) {
			t.Fatalf("StylesFor(春日部つむぎ) = %v", got)
		}
	})

	// 和集合をそのまま渡すと、実在しない組み合わせを提示することになります。
	t.Run("めたんは和集合より狭い", func(t *testing.T) {
		got, ok := reg.StylesFor("四国めたん")
		if !ok {
			t.Fatal("四国めたん が見つからない")
		}
		for _, unreal := range []string{"なみだめ", "ふつう", "ささやき"} {
			if slices.Contains(got, unreal) {
				t.Errorf("四国めたんに %s が含まれている", unreal)
			}
		}
	})

	t.Run("未知の話者", func(t *testing.T) {
		if _, ok := reg.StylesFor("存在しない"); ok {
			t.Fatal("未知の話者が見つかったことになっている")
		}
	})
}

// 既定スタイルは先頭のスタイルです。ノーマルを持たない話者が実在するため、
// ノーマル固定では成立しません。
func TestRegistryDefaultStyleFor(t *testing.T) {
	reg := testRegistry(t)

	for _, tt := range []struct {
		speaker string
		want    string
	}{
		{speaker: "四国めたん", want: "ノーマル"},
		{speaker: "春日部つむぎ", want: "ノーマル"},
		{speaker: "白上虎太郎", want: "ふつう"},
	} {
		got, ok := reg.DefaultStyleFor(tt.speaker)
		if !ok {
			t.Errorf("%s が見つからない", tt.speaker)
			continue
		}
		if got != tt.want {
			t.Errorf("DefaultStyleFor(%s) = %q, want %q", tt.speaker, got, tt.want)
		}
	}

	if _, ok := reg.DefaultStyleFor("存在しない"); ok {
		t.Error("未知の話者に既定スタイルが返った")
	}
}

// nil の Registry は「絞り込み無し」を表すため、参照しても落ちてはいけません。
func TestRegistryNilIsSafe(t *testing.T) {
	var reg *Registry

	if _, ok := reg.StylesFor("四国めたん"); ok {
		t.Error("nil Registry が話者を返した")
	}
	if _, ok := reg.DefaultStyleFor("四国めたん"); ok {
		t.Error("nil Registry が既定スタイルを返した")
	}
}

func TestNewRegistryRejectsBadInput(t *testing.T) {
	for _, tt := range []struct {
		name string
		raw  string
	}{
		{name: "壊れたJSON", raw: "{"},
		{name: "空配列", raw: "[]"},
		{name: "名前が無い", raw: `[{"name":"","styles":[{"name":"ノーマル","id":1,"type":"talk"}]}]`},
		{name: "読み上げスタイルが無い", raw: `[{"name":"歌専用","styles":[{"name":"うた","id":1,"type":"sing"}]}]`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewRegistry([]byte(tt.raw)); err == nil {
				t.Fatal("不正な一覧が素通りした")
			}
		})
	}
}

// 歌唱系のスタイルは読み上げに使えないため除外します。
func TestTalkStylesFilterByType(t *testing.T) {
	s := vvSpeaker{
		Name: "テスト",
		Styles: []vvStyle{
			{Name: "ノーマル", ID: 1, Type: "talk"},
			{Name: "ハミング", ID: 2, Type: "frame_decode"},
			{Name: "うた", ID: 3, Type: "sing"},
			{Name: "type無し", ID: 4},
		},
	}

	got := s.talkStyles()
	want := []string{"ノーマル", "type無し"}
	if len(got) != len(want) {
		t.Fatalf("talkStyles() = %d件, want %d件: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Name != w {
			t.Errorf("talkStyles()[%d] = %q, want %q", i, got[i].Name, w)
		}
	}
}
