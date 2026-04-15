package speaker

// ----------------------------------------------------------------------
// スクリプト解析定数
// ----------------------------------------------------------------------

// SupportedSpeakers は、このツールがサポートするすべて話者の一覧です。
var SupportedSpeakers = []SpeakerMapping{
	{APIName: "四国めたん", ToolTag: "[めたん]"},
	{APIName: "ずんだもん", ToolTag: "[ずんだもん]"},
}

// スクリプトやフォールバック設定で利用する話者タグ定数。
const (
	SpeakerTagMetan    = "[めたん]"
	SpeakerTagZundamon = "[ずんだもん]"
)

// VOICEVOXのスタイル名と一致させる定数（ツールタグ）
const (
	VvTagNormal   = "[ノーマル]"
	VvTagAmaama   = "[あまあま]"
	VvTagTsuntsun = "[ツンツン]"
	VvTagSexy     = "[セクシー]"
	VvTagWhisper  = "[ささやき]"
)

// DefaultFallbackCombinedTag はタグなしスクリプトを合成する際の推奨フォールバックです。
// フォールバックには [話者][スタイル] の完全な組み合わせを指定する必要があります。
const DefaultFallbackCombinedTag = SpeakerTagZundamon + VvTagNormal

// CombineTag は話者タグとスタイルタグを連結し、完全な [話者][スタイル] タグを返します。
func CombineTag(speakerTag, styleTag string) string {
	return speakerTag + styleTag
}

// VOICEVOX APIのスタイル名からツールのタグ定数へのマッピング
// SpeakerLoader が利用するために公開 (大文字始まり)
var StyleApiNameToToolTag = map[string]string{
	"ノーマル": VvTagNormal,
	"あまあま": VvTagAmaama,
	"ツンツン": VvTagTsuntsun,
	"セクシー": VvTagSexy,
	"ささやき": VvTagWhisper,
}
