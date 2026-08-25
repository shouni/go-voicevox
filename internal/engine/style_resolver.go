package engine

import (
	"context"
	"fmt"
	"log/slog"
)

// styleResolver は、1回の Run のあいだだけ生きるスタイル ID の解決器です。
//
// **キャッシュは Engine ではなくここが持ちます。** 以前は Engine が map と RWMutex を
// 抱えていました。しかし包んでいる先（StyleFinder.GetStyleID）自体が map 参照なので、
// Run をまたいで持ち越しても得るものはほとんどありません。一方、共有された可変状態は
// 「同じ Engine を 2 つの Run から同時に使えるか」を錠前の正しさの問題にしてしまいます。
// 1回分に閉じれば Engine は構築後に不変となり、その問いごと消えます。
//
// フォールバックの警告もバッチごとに 1 回ずつ出るようになります。Engine の寿命で
// 1 回だけだと、2 本目以降のジョブは同じ取りこぼしを黙って続けることになります。
type styleResolver struct {
	styles StyleFinder
	cache  map[string]int
}

func newStyleResolver(styles StyleFinder) *styleResolver {
	return &styleResolver{
		styles: styles,
		cache:  make(map[string]int),
	}
}

// resolve は話者タグから対応するスタイル ID を特定します（バッチ内キャッシュ付き）。
func (r *styleResolver) resolve(ctx context.Context, tag string, baseSpeakerTag string, index int) (int, error) {
	if id, ok := r.cache[tag]; ok {
		return id, nil
	}

	styleID, ok := r.styles.GetStyleID(tag)
	if ok {
		r.cache[tag] = styleID
		return styleID, nil
	}

	if baseSpeakerTag == "" {
		return 0, fmt.Errorf("話者タグ %s の抽出失敗 (セグメント %d)", tag, index)
	}

	fallbackKey, defaultOk := r.styles.GetDefaultTag(baseSpeakerTag)
	if defaultOk {
		slog.WarnContext(ctx, "AI出力タグが未定義のためフォールバックを適用します",
			"segment_index", index,
			"original_tag", tag,
			"fallback_key", fallbackKey)

		styleID, styleOk := r.styles.GetStyleID(fallbackKey)
		if styleOk {
			r.cache[tag] = styleID
			return styleID, nil
		}
	}

	return 0, fmt.Errorf("話者・スタイルタグ %s に対応する ID が見つかりません (セグメント %d)", tag, index)
}
