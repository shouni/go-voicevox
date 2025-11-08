package parser

const (
	// VOICEVOXが安全に処理できる最大文字数の目安。
	MaxSegmentCharLength = 200
	// 正規表現で利用する感情タグのパターン
	EmotionTagsPattern = `(解説|疑問|驚き|理解|落ち着き|納得|断定|呼びかけ|まとめ|通常|喜び|怒り|ノーマル|あまあま|ツンツン|セクシー|ヒソヒソ|ささやき)`
)
