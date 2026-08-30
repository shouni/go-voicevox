package speaker

// vvStyle は /speakers 応答のスタイル1件です。
type vvStyle struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
	// Type は "talk" や歌唱系を区別します。古い応答には無いため、空は talk 扱いです。
	Type string `json:"type"`
}

// vvSpeaker は /speakers 応答の話者1件です。
//
// 埋め込みの speakers.json と、起動時にエンジンから取得する応答の両方をこの型で読みます。
// 同じエンドポイントの同じ形なので、型を分ける理由がありません。
type vvSpeaker struct {
	Name   string    `json:"name"`
	Styles []vvStyle `json:"styles"`
}

// talkStyles は読み上げに使えるスタイルだけを宣言順に返します。
func (s vvSpeaker) talkStyles() []vvStyle {
	styles := make([]vvStyle, 0, len(s.Styles))
	for _, style := range s.Styles {
		if style.Type == "" || style.Type == styleTypeTalk {
			styles = append(styles, style)
		}
	}
	return styles
}
