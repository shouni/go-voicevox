package ports

// Segment は解析されたスクリプトの一片を表す構造体です。
type Segment struct {
	// SpeakerTag はスタイル名を含むフルタグを格納します（例: "[ずんだもん][ノーマル]"）。
	SpeakerTag string
	// BaseSpeakerTag はスタイル名を含まない話者名のみを格納します（例: "[ずんだもん]"）。
	BaseSpeakerTag string
	// Text は合成対象の正規化されたテキストです。
	Text string
}
