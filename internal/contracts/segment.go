package contracts

type Segment struct {
	SpeakerTag     string
	BaseSpeakerTag string
	Text           string
}

// ScriptLine は、呼び出し側が構造化データ(JSON など)から直接組み立てる
// 1発言分のデータです。テキストパーサーを経由せず Engine.RunScript に渡されます。
type ScriptLine struct {
	// Speaker は話者名です（例: "ずんだもん"）。角括弧は付けません。
	Speaker string `json:"speaker"`
	// Style はVOICEVOXのスタイル名です（例: "ノーマル"）。角括弧は付けません。
	Style string `json:"style"`
	// Direction は任意の演出用感情タグです（例: "呼びかけ"）。合成テキストには含まれません。
	Direction string `json:"direction,omitempty"`
	// Text は合成対象のテキストです。
	Text string `json:"text"`
}
