package engine

import "unicode/utf8"

// maxSegmentCharLength は VOICEVOX が安全に処理できる最大文字数の目安です。
//
// **このパッケージの外には出しません。** 呼び出し側が上限を選べるわけではなく、
// prepareSegments が唯一の利用者です。
const maxSegmentCharLength = 200

// splitPoints は、区切ってよい文字です。読点まで含めるのは、句点だけでは
// 上限に収まらない長文があるためです。
var splitPoints = map[rune]bool{'。': true, '、': true, '！': true, '？': true}

// splitByCharLimit は、句読点（。、！？）優先で text を limit 文字以内のチャンクに分割します。
// 句読点が見つからない場合は limit 文字で機械的に分割します。
func splitByCharLimit(text string, limit int) []string {
	if limit <= 0 || utf8.RuneCountInString(text) <= limit {
		return []string{text}
	}

	var chunks []string
	for remaining := text; remaining != ""; {
		head, rest := cutHead(remaining, limit)
		chunks = append(chunks, head)
		remaining = rest
	}
	return chunks
}

// cutHead は、limit 文字以内に収まる先頭部分と残りを返します。
//
// 上限内にある**最後の**句読点で切ります。最初の句読点で切ると必要以上に
// 短い断片が並び、合成の間が不自然になります。句読点が 1 つも無い場合だけ、
// 上限の位置で機械的に切ります。
//
// head は必ず 1 文字以上を返します。空を返すと呼び出し側が進まなくなるためです。
func cutHead(text string, limit int) (head, rest string) {
	runes := []rune(text)
	if len(runes) <= limit {
		return text, ""
	}

	for i := limit - 1; i >= 0; i-- {
		if splitPoints[runes[i]] {
			return string(runes[:i+1]), string(runes[i+1:])
		}
	}

	return string(runes[:limit]), string(runes[limit:])
}
