package engine

// ScriptLine は、呼び出し側が構造化データ(JSON など)から直接組み立てる
// 1発言分のデータです。Engine.Run にそのまま渡されます。
type ScriptLine struct {
	// Speaker は話者名です（例: "ずんだもん"）。角括弧は付けません。
	Speaker string `json:"speaker"`
	// Style はVOICEVOXのスタイル名です（例: "ノーマル"）。角括弧は付けません。
	Style string `json:"style"`
	// Text は合成対象のテキストです。
	Text string `json:"text"`
}
