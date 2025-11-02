package voicevox

import (
	"log/slog"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	// スクリプトの基本形式: [話者タグ][スタイルタグ] テキスト
	reScriptParse = regexp.MustCompile(`^(\[.+?\])\s*(\[.+?\])\s*(.*)`)
	// テキストから感情タグを取り除くための正規表現
	reEmotionParse = regexp.MustCompile(`\[` + EmotionTagsPattern + `\]`)
	// 最大テキスト長（文字数）。VOICEVOXが安全に処理できる最大文字数の目安。
	maxSegmentCharLength = MaxSegmentCharLength
)

// ----------------------------------------------------------------------
// textParser 構造体（Parser インターフェースの実装）
// ----------------------------------------------------------------------

// textParser はスクリプトの解析状態を管理し、セグメント化を実行します。
// これは model.Parser インターフェースのテキスト形式実装です。
type textParser struct {
	segments    []scriptSegment // model.scriptSegment を利用
	currentTag  string
	currentText *strings.Builder
	textBuffer  string
	fallbackTag string
}

// NewTextParser は textParser インスタンスを生成し、model.Parser インターフェースとして返します。
// 今回は単純な New 関数として実装しますが、必要に応じてファクトリパターンにできます。
func NewTextParser() *textParser {
	return &textParser{
		currentText: &strings.Builder{},
	}
}

// Parse は model.Parser インターフェースのメソッド実装です。
// スクリプト文字列を解析し、scriptSegment のスライスを返します。
func (p *textParser) Parse(scriptContent string, fallbackTag string) ([]scriptSegment, error) {
	p.fallbackTag = fallbackTag
	p.segments = nil // 過去のセグメントをリセット

	lines := strings.Split(scriptContent, "\n")

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}
		p.processLine(trimmedLine)
	}

	p.finishParsing()

	// エラーは finishParsing 内でログ出力し、ここでは nil を返す設計を維持
	return p.segments, nil
}

// ----------------------------------------------------------------------
// 内部処理ロジック (変更点あり)
// ----------------------------------------------------------------------

// processLine はスクリプトの1行を処理します。
func (p *textParser) processLine(line string) {
	if line == "" {
		return
	}

	textToProcess := line
	if p.textBuffer != "" {
		textToProcess = p.textBuffer + " " + line
		p.textBuffer = "" // バッファをクリア
	}

	matches := reScriptParse.FindStringSubmatch(textToProcess)
	if len(matches) > 3 {
		speakerTag := matches[1]
		vvStyleTag := matches[2]
		textPart := matches[3]
		newCombinedTag := speakerTag + vvStyleTag
		p.processTaggedLine(newCombinedTag, textPart)
	} else {
		p.processUntaggedLine(textToProcess)
	}
}

// processTaggedLine はタグ付きの行を処理します。
func (p *textParser) processTaggedLine(tag, text string) {
	// タグが変わった場合、またはタグが変わっていなくても前のセグメントが存在する場合 (一行一セグメントを強制)
	if p.currentTag != "" {
		p.flushCurrentSegment()
	}

	p.currentTag = tag
	p.appendAndSplitText(text)
}

// processUntaggedLine はタグのない行を処理します。
func (p *textParser) processUntaggedLine(text string) {
	if p.currentTag != "" {
		p.appendAndSplitText(text)
	} else {
		p.textBuffer = text
		slog.Warn("タグのないテキスト行が検出されました。次のタグ付きセグメントに結合されます。", "text", text)
	}
}

// appendAndSplitText はテキストを現在のセグメントに追記し、必要に応じて分割します。
func (p *textParser) appendAndSplitText(text string) {
	textToAppend := text
	for textToAppend != "" {
		partToAdd, remainder := p.splitTextByPunctuation(textToAppend)

		if partToAdd != "" {
			if p.currentText.Len() > 0 {
				p.currentText.WriteString(" ")
			}
			p.currentText.WriteString(partToAdd)
		}

		if remainder != "" {
			slog.Warn("テキストが最大文字数を超過したため、セグメントを強制的に確定し、残りのテキストを分割します。",
				"max_chars", maxSegmentCharLength, "tag", p.currentTag)
			p.flushCurrentSegment()
			textToAppend = remainder
		} else {
			textToAppend = ""
		}
	}
}

// splitTextByPunctuation は、現在のセグメントの文字数制限と句読点に基づき、追記するテキストを分割します。
func (p *textParser) splitTextByPunctuation(text string) (partToAdd string, remainder string) {
	currentRuneCount := utf8.RuneCountInString(p.currentText.String())

	space := 0
	if currentRuneCount > 0 {
		space = 1
	}

	if currentRuneCount+space+utf8.RuneCountInString(text) <= maxSegmentCharLength {
		return text, ""
	}

	maxCapacity := maxSegmentCharLength - currentRuneCount - space

	if maxCapacity <= 0 {
		// currentText が既に maxSegmentCharLength を超えている場合
		return "", text
	}

	runes := []rune(text)

	bestSplitIndex := -1
	for i := 0; i < len(runes); i++ {
		if currentRuneCount+space+(i+1) > maxSegmentCharLength {
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

// flushCurrentSegment は現在のテキストバッファを新しいセグメントとして確定し、バッファをリセットします。
func (p *textParser) flushCurrentSegment() {
	if p.currentText.Len() > 0 && p.currentTag != "" {
		p.addSegment(p.currentTag, p.currentText.String())
	}
	p.currentText.Reset()
}

// addSegment は整形後のテキストからセグメントを作成し、リストに追加します。
func (p *textParser) addSegment(tag string, text string) {
	finalText := reEmotionParse.ReplaceAllString(text, "")
	finalText = strings.TrimSpace(finalText)
	if finalText != "" {
		// model.scriptSegment を利用
		p.segments = append(p.segments, scriptSegment{
			SpeakerTag: tag,
			Text:       finalText,
		})
	}
}

// finishParsing は解析終了時に残っているバッファを処理します。
func (p *textParser) finishParsing() {
	p.flushCurrentSegment()

	if p.textBuffer != "" {
		if len(p.segments) > 0 {
			lastTag := p.segments[len(p.segments)-1].SpeakerTag
			slog.Warn("スクリプトの最後にタグのないテキストが残りました。最後のタグを流用して最終セグメントとして合成します。",
				"lost_text", p.textBuffer, "used_tag", lastTag)
			p.addSegment(lastTag, p.textBuffer)
		} else {
			slog.Warn("スクリプトにタグ付きセグメントがありませんでした。デフォルトタグを使用してテキスト全体を合成します。",
				"text_content", p.textBuffer, "default_tag", p.fallbackTag)
			if p.fallbackTag != "" {
				p.addSegment(p.fallbackTag, p.textBuffer)
			} else {
				slog.Error("スクリプトに有効なタグがなく、フォールバックタグも設定されていません。テキストは合成されません。", "lost_text", p.textBuffer)
			}
		}
	}
}
