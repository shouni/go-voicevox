// Package speaker は、VOICEVOX の話者・スタイルとツール内タグの対応を扱います。
package speaker

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
)

// ----------------------------------------------------------------------
// 対応話者レジストリ
// ----------------------------------------------------------------------

// speakersJSON は対応話者の一覧です。
//
// 話者・スタイルの語彙はここが唯一の出所です。Go の変数として手で並べていたときは、
// 「対応話者」「対応スタイル」「話者ごとにどのスタイルを持つか」が別々の場所にあり、
// 三つ目はどこにも無かったため、呼び出し側（AI のレスポンススキーマなど）は
// 全話者に全スタイルを許すしかありませんでした。実在しない組み合わせを提示すると、
// 選ばれたときに getStyleID がノーマルへ落とすため、動きはしても指示は無視されます。
//
//go:embed speakers.json
var speakersJSON []byte

// registrySpeaker は speakers.json の話者1件です。
type registrySpeaker struct {
	// APIName は VOICEVOX 側の名称です（例: "四国めたん"）。
	APIName string `json:"api_name"`
	// Tag はツール内で使う短縮名です（例: "めたん"）。角括弧は付けません。
	Tag string `json:"tag"`
	// Styles はこの話者が実際に持つスタイル名です。宣言順を保ちます。
	Styles []string `json:"styles"`
}

type registry struct {
	Speakers []registrySpeaker `json:"speakers"`
}

var supported = mustLoadRegistry(speakersJSON)

// mustLoadRegistry は埋め込み済みの話者一覧を読み込みます。
//
// 埋め込みリソースが壊れているのはビルド時点の誤りで、実行時に回復しようがないため
// panic します。ここを通らないと語彙が空のまま起動し、全セグメントがフォールバックする
// という分かりにくい壊れ方をします。
func mustLoadRegistry(raw []byte) registry {
	var reg registry
	if err := json.Unmarshal(raw, &reg); err != nil {
		panic(fmt.Sprintf("speakers.json のデコードに失敗しました: %v", err))
	}
	if len(reg.Speakers) == 0 {
		panic("speakers.json に話者が1件も定義されていません")
	}

	for _, s := range reg.Speakers {
		if s.APIName == "" || s.Tag == "" {
			panic(fmt.Sprintf("speakers.json の話者に api_name か tag がありません: %+v", s))
		}
		// LoadSpeakers はノーマルをフォールバック先として必須にしており、
		// 欠けていると起動時にエンジンへ問い合わせて初めて失敗します。
		// 一覧の側で完結する誤りなので、ここで先に落とします。
		if !slices.Contains(s.Styles, styleNormal) {
			panic(fmt.Sprintf("speakers.json の %q に %q がありません", s.APIName, styleNormal))
		}
	}

	return reg
}

// ----------------------------------------------------------------------
// スクリプト解析定数
// ----------------------------------------------------------------------

// styleNormal は、話者ごとの既定スタイル名です（角括弧なし）。
const styleNormal = "ノーマル"

// VvTagNormal は、フォールバック先として使う既定スタイルのタグです。
const VvTagNormal = "[" + styleNormal + "]"

// SupportedSpeakers は、このツールがサポートするすべての話者の一覧です。
var SupportedSpeakers = buildSupportedSpeakers()

// StyleAPINameToToolTag は、VOICEVOX API のスタイル名からツール内のタグ定数への対応表です。
// SpeakerLoader が参照するため公開しています。
var StyleAPINameToToolTag = buildStyleTagMap()

func buildSupportedSpeakers() []Mapping {
	mappings := make([]Mapping, len(supported.Speakers))
	for i, s := range supported.Speakers {
		mappings[i] = Mapping{APIName: s.APIName, ToolTag: bracket(s.Tag)}
	}
	return mappings
}

// buildStyleTagMap は全話者のスタイルを集めた対応表を返します。
// 話者をまたいで同じスタイル名は同じタグになるため、重複はそのまま上書きで構いません。
func buildStyleTagMap() map[string]string {
	tags := make(map[string]string)
	for _, s := range supported.Speakers {
		for _, style := range s.Styles {
			tags[style] = bracket(style)
		}
	}
	return tags
}

func bracket(name string) string {
	return "[" + name + "]"
}

// SupportedSpeakerNames は、このツールがサポートする話者名を角括弧なしで返します（例: "ずんだもん"）。
// AIへのプロンプト/レスポンススキーマ構築など、呼び出し側で許可語彙を列挙する用途を想定しています。
func SupportedSpeakerNames() []string {
	names := make([]string, len(supported.Speakers))
	for i, s := range supported.Speakers {
		names[i] = s.Tag
	}
	return names
}

// SupportedStyleNames は、いずれかの話者が持つスタイル名を角括弧なしで、宣言順に返します。
//
// **全話者がすべてを持つわけではありません。** 話者ごとの組み合わせが要る場合は
// StylesForSpeaker を使ってください。
func SupportedStyleNames() []string {
	names := make([]string, 0, len(StyleAPINameToToolTag))
	for _, s := range supported.Speakers {
		for _, style := range s.Styles {
			if !slices.Contains(names, style) {
				names = append(names, style)
			}
		}
	}
	return names
}

// StylesForSpeaker は、指定した話者が実際に持つスタイル名を宣言順に返します。
// 話者名は角括弧なしの短縮名（SupportedSpeakerNames が返す形）で指定します。
//
// 実在しない組み合わせを AI に提示しないための一覧です。存在しない話者には false を返します。
func StylesForSpeaker(name string) ([]string, bool) {
	for _, s := range supported.Speakers {
		if s.Tag != name {
			continue
		}
		return slices.Clone(s.Styles), true
	}
	return nil, false
}
