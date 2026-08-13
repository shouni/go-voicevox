package speaker

import (
	"reflect"
	"slices"
	"testing"
)

// testRegistryJSON は /speakers 応答の形をした架空の一覧です。
//
// 実在のキャラクター名は使いません。ライブラリは話者一覧を持たず、渡されたものを
// 解釈するだけなので、テストが特定の配役に依存する理由がありません。
// 話者デルタは、先頭スタイルが他と揃っていない（"標準" を持たない）ケースです。
const testRegistryJSON = `[
	{"name":"話者アルファ","styles":[
		{"name":"標準","id":2,"type":"talk"},
		{"name":"甘め","id":0,"type":"talk"},
		{"name":"小声","id":37,"type":"talk"}
	]},
	{"name":"話者ベータ","styles":[
		{"name":"標準","id":3,"type":"talk"},
		{"name":"囁き","id":22,"type":"talk"},
		{"name":"涙目","id":76,"type":"talk"}
	]},
	{"name":"話者ガンマ","styles":[{"name":"標準","id":8,"type":"talk"}]},
	{"name":"話者デルタ","styles":[
		{"name":"並","id":12,"type":"talk"},
		{"name":"陽気","id":32,"type":"talk"}
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

// 話者名は /speakers の表記そのままを返します。
func TestRegistrySpeakerNames(t *testing.T) {
	got := testRegistry(t).SpeakerNames()
	want := []string{"話者アルファ", "話者ベータ", "話者ガンマ", "話者デルタ"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SpeakerNames() = %v, want %v", got, want)
	}
}

// 返すのは「いずれかの話者が持つ」スタイルの和集合です。宣言順を保ち、重複は畳みます。
func TestRegistryStyleNames(t *testing.T) {
	got := testRegistry(t).StyleNames()
	want := []string{"標準", "甘め", "小声", "囁き", "涙目", "並", "陽気"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("StyleNames() = %v, want %v", got, want)
	}
}

// 話者ごとの組み合わせが取れることが、生の /speakers を扱う狙いです。
func TestRegistryStylesFor(t *testing.T) {
	reg := testRegistry(t)

	t.Run("スタイルが1つだけの話者", func(t *testing.T) {
		got, ok := reg.StylesFor("話者ガンマ")
		if !ok {
			t.Fatal("話者ガンマ が見つからない")
		}
		if !reflect.DeepEqual(got, []string{"標準"}) {
			t.Fatalf("StylesFor(話者ガンマ) = %v", got)
		}
	})

	// 和集合をそのまま渡すと、実在しない組み合わせを提示することになります。
	t.Run("和集合より狭い話者", func(t *testing.T) {
		got, ok := reg.StylesFor("話者アルファ")
		if !ok {
			t.Fatal("話者アルファ が見つからない")
		}
		for _, unreal := range []string{"涙目", "並", "囁き"} {
			if slices.Contains(got, unreal) {
				t.Errorf("話者アルファに %s が含まれている", unreal)
			}
		}
	})

	t.Run("未知の話者", func(t *testing.T) {
		if _, ok := reg.StylesFor("存在しない"); ok {
			t.Fatal("未知の話者が見つかったことになっている")
		}
	})
}

// 既定スタイルは先頭のスタイルです。特定の名前（"ノーマル" など）を必須にすると、
// それを持たない話者が実在するため成立しません。
func TestRegistryDefaultStyleFor(t *testing.T) {
	reg := testRegistry(t)

	for _, tt := range []struct {
		speaker string
		want    string
	}{
		{speaker: "話者アルファ", want: "標準"},
		{speaker: "話者ガンマ", want: "標準"},
		{speaker: "話者デルタ", want: "並"},
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

	if _, ok := reg.StylesFor("話者アルファ"); ok {
		t.Error("nil Registry が話者を返した")
	}
	if _, ok := reg.DefaultStyleFor("話者アルファ"); ok {
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
		{name: "名前が無い", raw: `[{"name":"","styles":[{"name":"標準","id":1,"type":"talk"}]}]`},
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
			{Name: "標準", ID: 1, Type: "talk"},
			{Name: "ハミング", ID: 2, Type: "frame_decode"},
			{Name: "うた", ID: 3, Type: "sing"},
			{Name: "type無し", ID: 4},
		},
	}

	got := s.talkStyles()
	want := []string{"標準", "type無し"}
	if len(got) != len(want) {
		t.Fatalf("talkStyles() = %d件, want %d件: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Name != w {
			t.Errorf("talkStyles()[%d] = %q, want %q", i, got[i].Name, w)
		}
	}
}
