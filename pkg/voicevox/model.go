package voicevox

import "context"

// ----------------------------------------------------------------------
// インターフェース
// ----------------------------------------------------------------------

// EngineExecutor は、Functional Options Patternの導入に伴い、Executeメソッドのシグネチャを変更します。
type EngineExecutor interface {
	// Execute はスクリプトを実行し、WAVファイルを生成します。
	// オプション（WithFallbackTagなど）は可変長引数として渡されます。
	Execute(ctx context.Context, scriptContent string, outputWavFile string, opts ...ExecuteOption) error
}

// SpeakerClient は /speakers エンドポイントを呼び出す能力を抽象化するインターフェースです。
type SpeakerClient interface {
	GetSpeakers(ctx context.Context) ([]byte, error)
}

// DataFinder は、Engine が Style ID を検索するために SpeakerData に要求するメソッドを定義します。
type DataFinder interface {
	GetStyleID(combinedTag string) (int, bool)
	GetDefaultTag(speakerToolTag string) (string, bool)
}

// Parser は、様々な形式の入力から音声合成用のセグメントを解析するインターフェースです。
type Parser interface {
	Parse(scriptContent string, fallbackTag string) ([]scriptSegment, error)
}

// ----------------------------------------------------------------------
// データモデル (スクリプト処理)
// ----------------------------------------------------------------------

// scriptSegment は解析されたスクリプトの一片を表す構造体です。
// StyleID と Err は、engine.go での高速化のために事前計算されます。
type scriptSegment struct {
	SpeakerTag     string // 例: "[ずんだもん][ノーマル]"
	BaseSpeakerTag string // 例: "[ずんだもん]"
	Text           string
	StyleID        int   // 並列処理に渡すための事前計算された Style ID
	Err            error // Style ID決定時に発生したエラー
}

// ----------------------------------------------------------------------
// データモデル (話者/スタイル)
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
type SpeakerData struct {
	StyleIDMap      map[string]int    // 例: "[めたん][ノーマル]" -> 2
	DefaultStyleMap map[string]string // 例: "[めたん]" -> "[めたん][ノーマル]" (フォールバック用)
}

// GetStyleID は DataFinder インターフェースの要件を満たします。
func (d *SpeakerData) GetStyleID(combinedTag string) (int, bool) {
	id, ok := d.StyleIDMap[combinedTag]
	return id, ok
}

// GetDefaultTag は DataFinder インターフェースの要件を満たします。
func (d *SpeakerData) GetDefaultTag(speakerToolTag string) (string, bool) {
	tag, ok := d.DefaultStyleMap[speakerToolTag]
	return tag, ok
}

// AudioQueryClient は Client が満たすべき API 呼び出しインターフェース
type AudioQueryClient interface {
	runAudioQuery(text string, styleID int, ctx context.Context) ([]byte, error)
	runSynthesis(queryBody []byte, styleID int, ctx context.Context) ([]byte, error)
}

// SegmentResult は Goroutineの結果を格納します。
type SegmentResult struct {
	index   int
	wavData []byte
	err     error
}

// ----------------------------------------------------------------------
// データモデル (API応答)
// ----------------------------------------------------------------------

// AudioQueryResponse は /audio_query APIの応答構造の一部に対応する型です。
type AudioQueryResponse struct {
	AccentPhrases []map[string]interface{} `json:"accent_phrases"`
	SpeedScale    float64                  `json:"speedScale"`
	// ... 他のフィールドは必要に応じて追加
}
