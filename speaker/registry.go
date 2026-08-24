// Package speaker は、VOICEVOX の話者・スタイルとツール内タグの対応を扱います。
package speaker

import (
	"encoding/json"
	"fmt"
	"slices"
)

// styleTypeTalk は、読み上げに使えるスタイルの type です。
// /audio_query と /synthesis は読み上げ用のため、歌唱系のスタイルは対象外です。
// 空文字を許すのは、type を持たない古い応答を扱う場合に talk とみなすためです。
const styleTypeTalk = "talk"

// Registry は、使用する話者とスタイルの一覧です。
//
// **このライブラリは一覧を持ちません。** どの話者を使うかはアプリケーションの方針であって
// 合成エンジンの都合ではないため、呼び出し側が /speakers 応答を保存して渡します。
// ライブラリに一覧を焼くと、話者を1人足すだけでライブラリのリリースが必要になり、
// アプリごとに違う配役を選ぶこともできません。
//
// 保持するのは語彙（誰がどのスタイルを持つか）だけで、**スタイル ID は使いません**。
// ID はエンジンのビルドによって変わりうるため、LoadStyles が起動時に実物へ問い合わせます。
type Registry struct {
	speakers []vvSpeaker
}

// NewRegistry は、VOICEVOX の /speakers 応答から Registry を構築します。
//
// 応答をそのまま渡せます。更新は保存し直すだけで済み、手で書き写した一覧のように
// エンジンが増やしたスタイルを取りこぼすことがありません。
//
//	curl -s "$VOICEVOX_API_URL/speakers" -o speakers.json
func NewRegistry(raw []byte) (*Registry, error) {
	var speakers []vvSpeaker
	if err := json.Unmarshal(raw, &speakers); err != nil {
		return nil, &ErrInvalidPayload{
			Context:    "保存された /speakers 応答",
			WrappedErr: err,
		}
	}
	if len(speakers) == 0 {
		return nil, &ErrMissingRequiredField{Field: "話者一覧", Context: "話者が1件も含まれていません"}
	}

	for _, s := range speakers {
		if s.Name == "" {
			return nil, &ErrMissingRequiredField{Field: "話者名", Context: "name の無い話者があります"}
		}
		// 読み上げスタイルが1つも無い話者は、フォールバック先を決められません。
		if len(s.talkStyles()) == 0 {
			return nil, &ErrMissingRequiredField{
				Field:   "読み上げスタイル",
				Context: fmt.Sprintf("話者 %q に読み上げスタイルがありません", s.Name),
			}
		}
	}

	return &Registry{speakers: speakers}, nil
}

// SpeakerNames は、この一覧に含まれる話者名を返します（例: "四国めたん"）。
// 名前は VOICEVOX の表記そのままです。
//
// AIへのプロンプト/レスポンススキーマ構築など、呼び出し側で許可語彙を列挙する用途を想定しています。
func (r *Registry) SpeakerNames() []string {
	names := make([]string, len(r.speakers))
	for i, s := range r.speakers {
		names[i] = s.Name
	}
	return names
}

// StyleNames は、いずれかの話者が持つスタイル名を宣言順に返します。
//
// **全話者がすべてを持つわけではありません。** 話者ごとの組み合わせが要る場合は
// StylesFor を使ってください。和集合をそのまま提示すると、実在しない組み合わせを
// AI に選ばせることになり、選ばれた分は既定スタイルへ落ちて指示が黙って無視されます。
func (r *Registry) StyleNames() []string {
	var names []string
	for _, s := range r.speakers {
		for _, style := range s.talkStyles() {
			if !slices.Contains(names, style.Name) {
				names = append(names, style.Name)
			}
		}
	}
	return names
}

// StylesFor は、指定した話者が持つ読み上げスタイル名を宣言順に返します。
// 含まれない話者には false を返します。
func (r *Registry) StylesFor(name string) ([]string, bool) {
	s, ok := r.find(name)
	if !ok {
		return nil, false
	}

	styles := s.talkStyles()
	names := make([]string, len(styles))
	for i, style := range styles {
		names[i] = style.Name
	}
	return names, true
}

// DefaultStyleFor は、指定した話者の既定スタイル名を返します。
//
// **先頭のスタイルを既定とします。** "ノーマル" を持たない話者が実在するためです
// （白上虎太郎はふつう、後鬼は人間ver.、里石ユカはつぼみ）。エンジンの一覧は
// どの話者も正規のスタイルを先頭に置いています。
func (r *Registry) DefaultStyleFor(name string) (string, bool) {
	s, ok := r.find(name)
	if !ok {
		return "", false
	}

	// NewRegistry が空でないことを保証しています。
	return s.talkStyles()[0].Name, true
}

func (r *Registry) find(name string) (vvSpeaker, bool) {
	if r == nil {
		return vvSpeaker{}, false
	}
	for _, s := range r.speakers {
		if s.Name == name {
			return s, true
		}
	}
	return vvSpeaker{}, false
}

// speakerTag は話者名からタグを組み立てます（例: "四国めたん" → "[四国めたん]"）。
func speakerTag(name string) string {
	return "[" + name + "]"
}

// styleTag は話者名とスタイル名から完全なタグを組み立てます。
func styleTag(speakerName, styleName string) string {
	return speakerTag(speakerName) + "[" + styleName + "]"
}
