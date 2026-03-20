package speaker

// ----------------------------------------------------------------------
// スクリプト解析定数
// ----------------------------------------------------------------------

// SupportedSpeakers は、このツールがサポートするすべて話者の一覧です。
var SupportedSpeakers = []SpeakerMapping{
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

// VOICEVOX APIのスタイル名からツールのタグ定数へのマッピング
// SpeakerLoader が利用するために公開 (大文字始まり)
var StyleApiNameToToolTag = map[string]string{
	"ノーマル": VvTagNormal,
	"あまあま": VvTagAmaama,
	"ツンツン": VvTagTsuntsun,
	"セクシー": VvTagSexy,
	"ささやき": VvTagWhisper,
}
