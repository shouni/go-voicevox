package voicevox

import "time"

// ----------------------------------------------------------------------
// WAV ファイル定数
// ----------------------------------------------------------------------

const (
	WavTotalHeaderSize  = 44
	DataChunkHeaderSize = 8  // "data" + data_size (8 bytes)
	FmtChunkSize        = 16 // format sub-chunk data size (16 bytes)

	// RIFF/WAVE チャンク (12 bytes)
	RiffChunkIDSize    = 4                                                 // "RIFF"
	RiffChunkSizeField = 4                                                 // File size - 8
	WaveIDSize         = 4                                                 // "WAVE"
	WavRiffHeaderSize  = RiffChunkIDSize + RiffChunkSizeField + WaveIDSize // 12 bytes

	// fmt チャンク (24 bytes)
	FmtChunkIDSize    = 4                                                 // "fmt "
	FmtChunkSizeField = 4                                                 // 16
	WavFmtChunkSize   = FmtChunkIDSize + FmtChunkSizeField + FmtChunkSize // 24 bytes

	// data チャンク (8 bytes)
	DataChunkIDSize = 4 // "data"
	// DataChunkSizeField は DataChunkHeaderSize - DataChunkIDSize と同じ

	// オフセット (audio.go のロジックで利用)
	RiffChunkSizeOffset = 4                                   // ファイルサイズが書き込まれる位置
	FmtChunkOffset      = WavRiffHeaderSize                   // "fmt "チャンクの開始位置 (12)
	DataChunkOffset     = WavRiffHeaderSize + WavFmtChunkSize // "data" チャンクの開始位置 (12 + 24 = 36)
	DataChunkSizeOffset = DataChunkOffset + DataChunkIDSize   // data チャンクのサイズが書き込まれる位置 (36 + 4 = 40)
)

// ----------------------------------------------------------------------
// エンジン処理定数
// ----------------------------------------------------------------------

const (
	DefaultMaxParallelSegments = 6
	DefaultSegmentTimeout      = 300 * time.Second
)

// ----------------------------------------------------------------------
// スクリプト解析定数
// ----------------------------------------------------------------------

const (
	MaxSegmentCharLength = 250
	EmotionTagsPattern   = `(解説|疑問|驚き|理解|落ち着き|納得|断定|呼びかけ|まとめ|通常|喜び|怒り|ノーマル|あまあま|ツンツン|セクシー|ヒソヒソ|ささやき)`
)

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
