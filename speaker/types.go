package speaker

import "context"

// Client は、話者一覧を取得するクライアントです。
//
// **利用する側のパッケージで定義します。** LoadSpeakers は公開関数なので、
// 引数の型が internal パッケージにあると、呼び出し側は渡せても名前を書けません
// （実際 contracts.SpeakerClient がシグネチャに出ていました）。
// 満たすのはメソッド 1 つなので、自前のクライアントを渡すのも難しくありません。
type Client interface {
	GetSpeakers(ctx context.Context) ([]byte, error)
}

// vvStyle は /speakers 応答のスタイル1件です。
type vvStyle struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
	// Type は "talk" や歌唱系を区別します。古い応答には無いため、空は talk 扱いです。
	Type string `json:"type"`
}

// vvSpeaker は /speakers 応答の話者1件です。
//
// 埋め込みの speakers.json と、起動時にエンジンから取得する応答の**両方**をこの型で読みます。
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

// Data は VOICEVOX から動的に取得した全話者・スタイル情報を保持するデータ構造です。
// この型は DataFinder インターフェースを実装します。
type Data struct {
	// StyleIDMap は完全なタグ名からスタイル ID へのマップです（例: "[四国めたん][ノーマル]" -> 2）。
	StyleIDMap map[string]int
	// DefaultStyleMap は話者タグからそのデフォルトスタイルタグへのマップです
	// （例: "[四国めたん]" -> "[四国めたん][ノーマル]"）。
	DefaultStyleMap map[string]string
}

// GetStyleID は指定されたタグに対応するスタイル ID を検索します。
func (d *Data) GetStyleID(tag string) (styleID int, ok bool) {
	id, found := d.StyleIDMap[tag]
	return id, found
}

// GetDefaultTag は話者のベースタグから、デフォルトとして使用すべきスタイルタグを検索します。
func (d *Data) GetDefaultTag(baseSpeakerTag string) (fallbackKey string, ok bool) {
	key, found := d.DefaultStyleMap[baseSpeakerTag]
	return key, found
}
