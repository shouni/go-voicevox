package speaker

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// testRegistryJSON is a fictional list in the shape of a /speakers response.
//
// No real character names are used. The library ships no speaker list and only interprets
// what it is given, so there is no reason for the tests to depend on a particular cast.
// 話者デルタ is the case whose first style differs from the others (it has no "標準").
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

// Speaker names are returned exactly as /speakers spells them.
func TestRegistrySpeakerNames(t *testing.T) {
	got := testRegistry(t).SpeakerNames()
	want := []string{"話者アルファ", "話者ベータ", "話者ガンマ", "話者デルタ"}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("SpeakerNames() mismatch (-want +got):\n%s", diff)
	}
}

// It returns the union of styles "any speaker has", in declaration order with duplicates
// folded.
func TestRegistryStyleNames(t *testing.T) {
	got := testRegistry(t).StyleNames()
	want := []string{"標準", "甘め", "小声", "囁き", "涙目", "並", "陽気"}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("StyleNames() mismatch (-want +got):\n%s", diff)
	}
}

// Getting per-speaker combinations is the point of working with the raw /speakers.
func TestRegistryStylesFor(t *testing.T) {
	reg := testRegistry(t)

	t.Run("speaker with a single style", func(t *testing.T) {
		got, ok := reg.StylesFor("話者ガンマ")
		if !ok {
			t.Fatal("話者ガンマ not found")
		}
		if diff := cmp.Diff([]string{"標準"}, got); diff != "" {
			t.Fatalf("StylesFor(話者ガンマ) mismatch (-want +got):\n%s", diff)
		}
	})

	// Passing the union as is would offer combinations that do not exist.
	t.Run("speaker narrower than the union", func(t *testing.T) {
		got, ok := reg.StylesFor("話者アルファ")
		if !ok {
			t.Fatal("話者アルファ not found")
		}
		for _, unreal := range []string{"涙目", "並", "囁き"} {
			if slices.Contains(got, unreal) {
				t.Errorf("話者アルファ includes %s", unreal)
			}
		}
	})

	t.Run("unknown speaker", func(t *testing.T) {
		if _, ok := reg.StylesFor("存在しない"); ok {
			t.Fatal("an unknown speaker was reported as found")
		}
	})
}

// The default style is the first style. Requiring a particular name (such as "ノーマル") would
// not work, because real speakers exist without it.
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
			t.Errorf("%s not found", tt.speaker)
			continue
		}
		if got != tt.want {
			t.Errorf("DefaultStyleFor(%s) = %q, want %q", tt.speaker, got, tt.want)
		}
	}

	if _, ok := reg.DefaultStyleFor("存在しない"); ok {
		t.Error("a default style was returned for an unknown speaker")
	}
}

// A nil Registry means "no filter", so querying it must not panic.
func TestRegistryNilIsSafe(t *testing.T) {
	var reg *Registry

	if _, ok := reg.StylesFor("話者アルファ"); ok {
		t.Error("nil Registry returned a speaker")
	}
	if _, ok := reg.DefaultStyleFor("話者アルファ"); ok {
		t.Error("nil Registry returned a default style")
	}
}

func TestNewRegistryRejectsBadInput(t *testing.T) {
	for _, tt := range []struct {
		name string
		raw  string
	}{
		{name: "broken JSON", raw: "{"},
		{name: "empty array", raw: "[]"},
		{name: "missing name", raw: `[{"name":"","styles":[{"name":"標準","id":1,"type":"talk"}]}]`},
		{name: "no talk style", raw: `[{"name":"歌専用","styles":[{"name":"うた","id":1,"type":"sing"}]}]`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewRegistry([]byte(tt.raw)); err == nil {
				t.Fatal("an invalid list passed through")
			}
		})
	}
}

// Singing styles cannot be used for speech, so they are excluded.
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
		t.Fatalf("talkStyles() = %d items, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Name != w {
			t.Errorf("talkStyles()[%d] = %q, want %q", i, got[i].Name, w)
		}
	}
}
