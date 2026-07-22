package engine

import "unicode/utf8"

// MaxSegmentCharLength は VOICEVOX が安全に処理できる最大文字数の目安です。
const MaxSegmentCharLength = 200

// SplitByCharLimit は、句読点（。、！？）優先で text を limit 文字以内のチャンクに分割します。
// 句読点が見つからない場合は limit 文字で機械的に分割します。
func SplitByCharLimit(text string, limit int) []string {
	if limit <= 0 || utf8.RuneCountInString(text) <= limit {
		return []string{text}
	}

	var chunks []string
	remaining := text
	for remaining != "" {
		part, rest := splitByCharLimit(remaining, limit, 0)
		if part != "" {
			chunks = append(chunks, part)
		}
		if rest == remaining {
			// 分割できない場合は無限ループを避けて残りをそのまま追加します。
			if part == "" {
				chunks = append(chunks, rest)
			}
			break
		}
		remaining = rest
	}
	return chunks
}

// splitByCharLimit は、既に使用済みの文字数(usedRuneCount)を考慮しつつ、
// limit 文字以内に収まる先頭部分(partToAdd)と残り(remainder)を句読点優先で求めます。
func splitByCharLimit(text string, limit int, usedRuneCount int) (partToAdd string, remainder string) {
	if usedRuneCount+utf8.RuneCountInString(text) <= limit {
		return text, ""
	}

	maxCapacity := limit - usedRuneCount
	if maxCapacity <= 0 {
		return "", text
	}

	runes := []rune(text)
	bestSplitIndex := -1

	for i := range runes {
		if usedRuneCount+(i+1) > limit {
			break
		}
		r := runes[i]
		if r == '。' || r == '、' || r == '！' || r == '？' {
			bestSplitIndex = i + 1
		}
	}

	if bestSplitIndex > 0 {
		partToAdd = string(runes[:bestSplitIndex])
		remainder = string(runes[bestSplitIndex:])
		return partToAdd, remainder
	}

	if maxCapacity > 0 && maxCapacity < len(runes) {
		partToAdd = string(runes[:maxCapacity])
		remainder = string(runes[maxCapacity:])
		return partToAdd, remainder
	}

	return text, ""
}
