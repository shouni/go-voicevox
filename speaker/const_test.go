package speaker

import (
	"reflect"
	"slices"
	"testing"
)

func TestSupportedSpeakerNames(t *testing.T) {
	got := SupportedSpeakerNames()
	want := []string{"めたん", "ずんだもん", "つむぎ"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SupportedSpeakerNames() = %v, want %v", got, want)
	}
}

// 返すのは「いずれかの話者が持つ」スタイルの和集合です。宣言順を保ち、重複は畳みます。
// ヘロヘロ・なみだめ は ずんだもん しか持たないため、和集合にだけ現れます。
func TestSupportedStyleNames(t *testing.T) {
	got := SupportedStyleNames()
	want := []string{"ノーマル", "あまあま", "ツンツン", "セクシー", "ささやき", "ヒソヒソ", "ヘロヘロ", "なみだめ"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SupportedStyleNames() = %v, want %v", got, want)
	}
}

// 話者ごとの組み合わせが取れることが JSON 化の目的です。和集合をそのまま
// 全話者に許すと、実在しない組み合わせを AI に提示してしまいます。
func TestStylesForSpeaker(t *testing.T) {
	t.Run("つむぎはノーマルだけ", func(t *testing.T) {
		got, ok := StylesForSpeaker("つむぎ")
		if !ok {
			t.Fatal("つむぎ が見つからない")
		}
		if !reflect.DeepEqual(got, []string{"ノーマル"}) {
			t.Fatalf("StylesForSpeaker(つむぎ) = %v", got)
		}
	})

	// めたんは ヘロヘロ・なみだめ を持たないため、和集合をそのまま渡すと
	// 実在しない組み合わせを提示することになります。
	t.Run("めたんは和集合より狭い", func(t *testing.T) {
		got, ok := StylesForSpeaker("めたん")
		if !ok {
			t.Fatal("めたん が見つからない")
		}
		if len(got) >= len(SupportedStyleNames()) {
			t.Fatalf("めたんが和集合と同じ広さになっている: %v", got)
		}
		for _, unreal := range []string{"ヘロヘロ", "なみだめ"} {
			if slices.Contains(got, unreal) {
				t.Errorf("めたんに %s が含まれている", unreal)
			}
		}
	})

	t.Run("未知の話者", func(t *testing.T) {
		if _, ok := StylesForSpeaker("存在しない"); ok {
			t.Fatal("未知の話者が見つかったことになっている")
		}
	})

	// 返した slice を書き換えても内部の一覧が壊れないこと。
	t.Run("呼び出し側の変更が漏れない", func(t *testing.T) {
		got, _ := StylesForSpeaker("つむぎ")
		got[0] = "書き換え"

		again, _ := StylesForSpeaker("つむぎ")
		if again[0] != "ノーマル" {
			t.Fatalf("内部の一覧が書き換えられた: %v", again)
		}
	})
}

// LoadSpeakers はノーマルをフォールバック先として必須にしているため、
// 一覧の時点で全話者が持っていなければなりません。
func TestRegistryEverySpeakerHasNormal(t *testing.T) {
	for _, name := range SupportedSpeakerNames() {
		styles, ok := StylesForSpeaker(name)
		if !ok {
			t.Fatalf("%s の一覧が引けない", name)
		}
		if !slices.Contains(styles, "ノーマル") {
			t.Errorf("%s に ノーマル がない: %v", name, styles)
		}
	}
}

// SupportedSpeakers と StyleAPINameToToolTag は JSON から導出されるため、
// 角括弧付きのタグ表記が保たれていることを確かめます。
func TestDerivedTagsKeepBrackets(t *testing.T) {
	for _, m := range SupportedSpeakers {
		if m.ToolTag[0] != '[' || m.ToolTag[len(m.ToolTag)-1] != ']' {
			t.Errorf("ToolTag に角括弧がない: %q (%s)", m.ToolTag, m.APIName)
		}
	}
	if got := StyleAPINameToToolTag["ノーマル"]; got != VvTagNormal {
		t.Errorf("StyleAPINameToToolTag[ノーマル] = %q, want %q", got, VvTagNormal)
	}
}
