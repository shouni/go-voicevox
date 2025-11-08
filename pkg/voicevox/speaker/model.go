package speaker

import "context"

// ----------------------------------------------------------------------
// インターフェース定義
// ----------------------------------------------------------------------

// SpeakerClient は /speakers エンドポイントを呼び出す能力を抽象化するインターフェースです。
// api.Client がこれを満たす必要があります。
type SpeakerClient interface {
	GetSpeakers(ctx context.Context) ([]byte, error)
}

// DataFinder は Style ID やデフォルトスタイルの検索機能を抽象化します。
// Engine はこのインターフェースに依存します。
type DataFinder interface {
	GetStyleID(tag string) (styleID int, ok bool)
	GetDefaultTag(baseSpeakerTag string) (fallbackKey string, ok bool)
}

// ----------------------------------------------------------------------
// 構造体定義
// ----------------------------------------------------------------------

// SpeakerMapping は、VOICEVOX API名とツールで使用する短縮タグのペアを定義します。
type SpeakerMapping struct {
	APIName string // 例: "四国めたん"
	ToolTag string // 例: "[めたん]"
}

// VVSpeaker はVOICEVOXの /speakers APIの応答JSON構造の一部に対応する型です。
type VVSpeaker struct {
	Name   string `json:"name"`
	Styles []struct {
		Name string `json:"name"`
		ID   int    `json:"id"`
	} `json:"styles"`
}

// SpeakerData はVOICEVOXから動的に取得した全話者・スタイル情報を保持するメインのデータ構造です。
// これは DataFinder インターフェースを満たします。
type SpeakerData struct {
	StyleIDMap      map[string]int    // 例: "[めたん][ノーマル]" -> 2
	DefaultStyleMap map[string]string // 例: "[めたん]" -> "[めたん][ノーマル]" (フォールバック用)
}

// StyleIDMap から StyleID を検索します (DataFinder 実装)
func (d *SpeakerData) GetStyleID(tag string) (styleID int, ok bool) {
	id, found := d.StyleIDMap[tag]
	return id, found
}

// BaseSpeakerTag からデフォルトスタイルタグを検索します (DataFinder 実装)
func (d *SpeakerData) GetDefaultTag(baseSpeakerTag string) (fallbackKey string, ok bool) {
	key, found := d.DefaultStyleMap[baseSpeakerTag]
	return key, found
}
