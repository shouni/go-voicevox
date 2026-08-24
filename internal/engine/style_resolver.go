package engine

import (
	"context"
	"fmt"
	"log/slog"
)

// getStyleID は話者タグから対応する Style ID を特定します（キャッシュ付き）。
func (e *Engine) getStyleID(ctx context.Context, tag string, baseSpeakerTag string, index int) (int, error) {
	e.styleIDCacheMutex.RLock()
	if id, ok := e.styleIDCache[tag]; ok {
		e.styleIDCacheMutex.RUnlock()
		return id, nil
	}
	e.styleIDCacheMutex.RUnlock()

	styleID, ok := e.styles.GetStyleID(tag)
	if ok {
		e.cacheStyleID(tag, styleID)
		return styleID, nil
	}

	if baseSpeakerTag == "" {
		return 0, fmt.Errorf("話者タグ %s の抽出失敗 (セグメント %d)", tag, index)
	}

	fallbackKey, defaultOk := e.styles.GetDefaultTag(baseSpeakerTag)
	if defaultOk {
		slog.WarnContext(ctx, "AI出力タグが未定義のためフォールバックを適用します",
			"segment_index", index,
			"original_tag", tag,
			"fallback_key", fallbackKey)

		styleID, styleOk := e.styles.GetStyleID(fallbackKey)
		if styleOk {
			e.cacheStyleID(tag, styleID)
			return styleID, nil
		}
	}

	return 0, fmt.Errorf("話者・スタイルタグ %s に対応する ID が見つかりません (セグメント %d)", tag, index)
}

func (e *Engine) cacheStyleID(tag string, styleID int) {
	e.styleIDCacheMutex.Lock()
	defer e.styleIDCacheMutex.Unlock()
	e.styleIDCache[tag] = styleID
}
