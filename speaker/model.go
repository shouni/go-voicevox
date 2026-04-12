package speaker

// DataFinder はスタイル ID やデフォルトスタイルの検索機能を抽象化します。
// Engine はこのインターフェースに依存して音声合成に必要な ID を特定します。
type DataFinder interface {
	GetStyleID(tag string) (styleID int, ok bool)
	GetDefaultTag(baseSpeakerTag string) (fallbackKey string, ok bool)
}

// SpeakerMapping は、VOICEVOX API で使用される名前と、
// ツール内で使用する短縮タグのペアを定義します。
type SpeakerMapping struct {
	// APIName は VOICEVOX 側の名称です（例: "四国めたん"）。
	APIName string
	// ToolTag はツール内で使用する識別タグです（例: "[めたん]"）。
	ToolTag string
}

// VVSpeaker は VOICEVOX の /speakers API 応答 JSON 構造をパースするための型です。
type VVSpeaker struct {
	Name   string `json:"name"`
	Styles []struct {
		Name string `json:"name"`
		ID   int    `json:"id"`
	} `json:"styles"`
}

// SpeakerData は VOICEVOX から動的に取得した全話者・スタイル情報を保持するデータ構造です。
// この型は DataFinder インターフェースを実装します。
type SpeakerData struct {
	// StyleIDMap は完全なタグ名からスタイル ID へのマップです（例: "[めたん][ノーマル]" -> 2）。
	StyleIDMap map[string]int
	// DefaultStyleMap は話者タグからそのデフォルトスタイルタグへのマップです（例: "[めたん]" -> "[めたん][ノーマル]"）。
	DefaultStyleMap map[string]string
}

// GetStyleID は指定されたタグに対応するスタイル ID を検索します。
func (d *SpeakerData) GetStyleID(tag string) (styleID int, ok bool) {
	id, found := d.StyleIDMap[tag]
	return id, found
}

// GetDefaultTag は話者のベースタグから、デフォルトとして使用すべきスタイルタグを検索します。
func (d *SpeakerData) GetDefaultTag(baseSpeakerTag string) (fallbackKey string, ok bool) {
	key, found := d.DefaultStyleMap[baseSpeakerTag]
	return key, found
}
