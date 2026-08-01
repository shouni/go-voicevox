// Package speaker は、VOICEVOX の話者・スタイルとツール内タグの対応を扱います。
package speaker

import "strings"

// ----------------------------------------------------------------------
// スクリプト解析定数
// ----------------------------------------------------------------------

// SupportedSpeakers は、このツールがサポートするすべて話者の一覧です。
var SupportedSpeakers = []Mapping{
	{APIName: "四国めたん", ToolTag: "[めたん]"},
	{APIName: "ずんだもん", ToolTag: "[ずんだもん]"},
}

// VOICEVOXのスタイル名と一致させる定数（ツールタグ）
const (
	VvTagNormal   = "[ノーマル]"
	VvTagAmaama   = "[あまあま]"
	VvTagTsuntsun = "[ツンツン]"
	VvTagSexy     = "[セクシー]"
	VvTagWhisper  = "[ささやき]"
)

// StyleAPINameToToolTag は、VOICEVOX API のスタイル名からツール内のタグ定数への対応表です。
// SpeakerLoader が参照するため公開しています。
var StyleAPINameToToolTag = map[string]string{
	"ノーマル": VvTagNormal,
	"あまあま": VvTagAmaama,
	"ツンツン": VvTagTsuntsun,
	"セクシー": VvTagSexy,
	"ささやき": VvTagWhisper,
}

// SupportedSpeakerNames は、このツールがサポートする話者名を角括弧なしで返します（例: "ずんだもん"）。
// AIへのプロンプト/レスポンススキーマ構築など、呼び出し側で許可語彙を列挙する用途を想定しています。
func SupportedSpeakerNames() []string {
	names := make([]string, len(SupportedSpeakers))
	for i, s := range SupportedSpeakers {
		names[i] = strings.Trim(s.ToolTag, "[]")
	}
	return names
}

// SupportedStyleNames は、VOICEVOXでサポートするスタイル名を角括弧なしで、宣言順に返します。
func SupportedStyleNames() []string {
	styleTags := []string{VvTagNormal, VvTagAmaama, VvTagTsuntsun, VvTagSexy, VvTagWhisper}
	names := make([]string, len(styleTags))
	for i, tag := range styleTags {
		names[i] = strings.Trim(tag, "[]")
	}
	return names
}
