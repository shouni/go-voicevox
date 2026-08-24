package speaker

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
)

// Client は、話者一覧を取得するクライアントです。
//
// **利用する側のパッケージで定義します。** LoadStyles は公開メソッドなので、
// 引数の型が internal パッケージにあると、呼び出し側は渡せても名前を書けません。
// 満たすのはメソッド 1 つなので、自前のクライアントを渡すのも難しくありません。
type Client interface {
	GetSpeakers(ctx context.Context) ([]byte, error)
}

// Styles は、エンジンに問い合わせて解決した「タグ → スタイル ID」の対応です。
// この型は internal/engine が要求するスタイル ID 解決の口を満たします。
//
// **中身は読めるだけです。** 以前は map を公開フィールドで持っていましたが、
// 組み立てるのは LoadStyles だけなので、合成の最中に書き換えられる口を
// 開けておく理由がありません。
type Styles struct {
	// byTag は完全なタグからスタイル ID へのマップです（例: "[四国めたん][ノーマル]" -> 2）。
	byTag map[string]int
	// defaultByTag は話者タグからそのデフォルトスタイルタグへのマップです
	// （例: "[四国めたん]" -> "[四国めたん][ノーマル]"）。
	defaultByTag map[string]string
}

// GetStyleID は指定されたタグに対応するスタイル ID を検索します。
func (s *Styles) GetStyleID(tag string) (styleID int, ok bool) {
	id, found := s.byTag[tag]
	return id, found
}

// GetDefaultTag は話者のベースタグから、フォールバック先として使用すべき
// スタイルタグを検索します。
func (s *Styles) GetDefaultTag(baseSpeakerTag string) (fallbackTag string, ok bool) {
	tag, found := s.defaultByTag[baseSpeakerTag]
	return tag, found
}

// LoadStyles は /speakers エンドポイントを引き、この一覧に載っている話者・スタイルの
// スタイル ID を解決します。
//
// **スタイル ID は必ず実物のエンジンから取ります。** ID はエンジンのビルドで変わりうるため、
// 保存しておいた値をそのまま使うと、更新の遅れが「別のキャラの声で喋る」形で出ます。
//
// レシーバが nil なら絞り込まず、エンジンが返したものをすべて受け入れます。どちらの
// 場合も、エンジンに無い組み合わせは組みません。
//
// **Registry のメソッドなのは、一覧そのものが絞り込みだからです。** 以前は
// LoadSpeakers(ctx, client, allowed) という関数で、主語である一覧が 3 番目の引数に
// 置かれ、nil を渡す意味も呼び出し側からは読めませんでした。
func (r *Registry) LoadStyles(ctx context.Context, client Client) (*Styles, error) {
	bodyBytes, err := client.GetSpeakers(ctx)
	if err != nil {
		return nil, err
	}

	var engineSpeakers []vvSpeaker
	if err := json.Unmarshal(bodyBytes, &engineSpeakers); err != nil {
		return nil, &ErrInvalidPayload{Context: "/speakers 応答", WrappedErr: err}
	}

	styles := &Styles{
		byTag:        make(map[string]int),
		defaultByTag: make(map[string]string),
	}

	for _, spk := range engineSpeakers {
		wanted, restricted := r.allowedStyles(spk.Name)
		if restricted && wanted == nil {
			slog.Debug("一覧に無い話者をスキップします", "speaker", spk.Name)
			continue
		}

		for _, style := range spk.talkStyles() {
			if restricted && !slices.Contains(wanted, style.Name) {
				slog.Debug("一覧に無いスタイルをスキップします", "speaker", spk.Name, "style", style.Name)
				continue
			}
			styles.byTag[styleTag(spk.Name, style.Name)] = style.ID
		}

		if tag, ok := r.resolveDefaultTag(styles.byTag, spk); ok {
			styles.defaultByTag[speakerTag(spk.Name)] = tag
		}
	}

	// 1人も組めなければ、以降のセグメントは全滅します。合成を始める前に止めます。
	if len(styles.defaultByTag) == 0 {
		return nil, &ErrMissingRequiredField{
			Field:   "利用可能な話者",
			Context: "エンジンの /speakers 応答から使用できる話者を1人も組み立てられませんでした",
		}
	}

	slog.InfoContext(ctx, "VOICEVOXスタイルデータが正常にロードされました",
		"speakers_count", len(styles.defaultByTag), "styles_count", len(styles.byTag))

	return styles, nil
}

// allowedStyles は、この話者に許可されたスタイル名と、絞り込みが働いているかを返します。
// 一覧が nil のときは絞り込み無し（restricted=false）です。
func (r *Registry) allowedStyles(name string) (styles []string, restricted bool) {
	if r == nil {
		return nil, false
	}
	names, ok := r.StylesFor(name)
	if !ok {
		return nil, true
	}
	return names, true
}

// resolveDefaultTag は、話者のフォールバック先タグを決めます。
//
// 第一候補は一覧側の既定（先頭スタイル）ですが、エンジンがそれを返さなかった場合は、
// 実際に組めたスタイルの先頭を使います。保存した一覧がエンジンより新しいことは普通に
// 起こるため、そこで話者ごと使えなくする理由はありません。
func (r *Registry) resolveDefaultTag(styleIDs map[string]int, spk vvSpeaker) (string, bool) {
	if preferred, ok := r.DefaultStyleFor(spk.Name); ok {
		if tag := styleTag(spk.Name, preferred); hasTag(styleIDs, tag) {
			return tag, true
		}
	}

	for _, style := range spk.talkStyles() {
		if tag := styleTag(spk.Name, style.Name); hasTag(styleIDs, tag) {
			return tag, true
		}
	}
	return "", false
}

// hasTag は、スタイル ID が 0（あまあまの ID など実在する値）でも取り違えないよう、
// キーの有無だけを見ます。
func hasTag(m map[string]int, key string) bool {
	_, ok := m[key]
	return ok
}
