package speaker

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
)

// LoadSpeakers は /speakers エンドポイントからデータを取得し、Data を構築します。
//
// **スタイル ID は必ず実物のエンジンから取ります。** ID はエンジンのビルドで変わりうるため、
// 保存しておいた値をそのまま使うと、更新の遅れが「別のキャラの声で喋る」形で出ます。
//
// allowed に Registry を渡すと、そこに載っている話者・スタイルだけを組み立てます。
// nil ならエンジンが返したものをすべて受け入れます。どちらの場合も、エンジンに無い
// 組み合わせは組みません。
func LoadSpeakers(ctx context.Context, client Client, allowed *Registry) (*Data, error) {
	bodyBytes, err := client.GetSpeakers(ctx)
	if err != nil {
		return nil, err
	}

	var engineSpeakers []vvSpeaker
	if err := json.Unmarshal(bodyBytes, &engineSpeakers); err != nil {
		return nil, &ErrInvalidPayload{Context: "/speakers 応答", WrappedErr: err}
	}

	data := &Data{
		StyleIDMap:      make(map[string]int),
		DefaultStyleMap: make(map[string]string),
	}

	for _, spk := range engineSpeakers {
		wanted, restricted := allowedStyles(allowed, spk.Name)
		if restricted && wanted == nil {
			slog.Debug("一覧に無い話者をスキップします", "speaker", spk.Name)
			continue
		}

		for _, style := range spk.talkStyles() {
			if restricted && !slices.Contains(wanted, style.Name) {
				slog.Debug("一覧に無いスタイルをスキップします", "speaker", spk.Name, "style", style.Name)
				continue
			}
			data.StyleIDMap[styleTag(spk.Name, style.Name)] = style.ID
		}

		if tag, ok := resolveDefaultTag(data.StyleIDMap, allowed, spk); ok {
			data.DefaultStyleMap[speakerTag(spk.Name)] = tag
		}
	}

	// 1人も組めなければ、以降のセグメントは全滅します。合成を始める前に止めます。
	if len(data.DefaultStyleMap) == 0 {
		return nil, &ErrMissingRequiredField{
			Field:   "利用可能な話者",
			Context: "エンジンの /speakers 応答から使用できる話者を1人も組み立てられませんでした",
		}
	}

	slog.InfoContext(ctx, "VOICEVOXスタイルデータが正常にロードされました",
		"speakers_count", len(data.DefaultStyleMap), "styles_count", len(data.StyleIDMap))

	return data, nil
}

// allowedStyles は、この話者に許可されたスタイル名と、絞り込みが働いているかを返します。
// allowed が nil のときは絞り込み無し（restricted=false）です。
func allowedStyles(allowed *Registry, name string) (styles []string, restricted bool) {
	if allowed == nil {
		return nil, false
	}
	names, ok := allowed.StylesFor(name)
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
func resolveDefaultTag(styleIDs map[string]int, allowed *Registry, spk vvSpeaker) (string, bool) {
	if preferred, ok := allowed.DefaultStyleFor(spk.Name); ok {
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
