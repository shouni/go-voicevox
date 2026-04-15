package parser

import (
	"log/slog"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/shouni/go-voicevox/internal/contracts"
)

const (
	// maxSegmentCharLength は VOICEVOX が安全に処理できる最大文字数の目安です。
	maxSegmentCharLength = 200
	// emotionTagsPattern は正規表現で利用する感情タグのパターンです。
	emotionTagsPattern = `(解説|疑問|驚き|理解|落ち着き|納得|断定|呼びかけ|まとめ|通常|喜び|怒り|ノーマル|あまあま|ツンツン|セクシー|ヒソヒソ|ささやき)`
)

var (
	// reScriptParse は [話者タグ][スタイルタグ] テキスト の形式を解析します。
	reScriptParse = regexp.MustCompile(`^(\[.+?\])\s*(\[.+?\])\s*(.*)`)
	// reEmotionParse はテキストから感情タグ（[通常] など）を取り除くための正規表現です。
	reEmotionParse = regexp.MustCompile(`\[` + emotionTagsPattern + `\]`)
	// reBaseSpeakerTag はタグの先頭から話者名（最初の括弧部分）のみを抽出します。
	reBaseSpeakerTag = regexp.MustCompile(`^(\[.+?\])`)
)

// textParser はスクリプトの解析状態を管理し、セグメント化を実行する実体です。
type textParser struct {
	segments    []contracts.Segment
	currentTag  string
	currentText *strings.Builder
	textBuffer  string
	fallbackTag string
}

// NewParser は新しい textParser インスタンスを生成し、Parser インターフェースとして返します。
func NewParser() *textParser {
	return &textParser{
		currentText: &strings.Builder{},
	}
}

// Parse は Parser インターフェースのメソッド実装です。
// 入力されたスクリプトを解析し、Segment のスライスを返します。
// fallbackTag には [話者][スタイル] の完全なタグを指定する必要があります。
//
// 呼び出しごとに内部状態（バッファや現在のタグなど）は完全にリセットされるため、
// 同じ Parser インスタンスを安全に再利用できます。
func (p *textParser) Parse(scriptContent string, fallbackTag string) ([]contracts.Segment, error) {
	// 内部状態の完全初期化
	p.fallbackTag = fallbackTag
	p.segments = nil
	p.currentTag = ""
	p.textBuffer = ""
	p.currentText.Reset() // strings.Builder のリセット

	lines := strings.Split(scriptContent, "\n")
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}
		p.processLine(trimmedLine)
	}

	p.finishParsing()
	return p.segments, nil
}

// processLine はスクリプトの1行を処理します。
func (p *textParser) processLine(line string) {
	if line == "" {
		return
	}

	textToProcess := line
	if p.textBuffer != "" {
		textToProcess = p.textBuffer + " " + line
		p.textBuffer = ""
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
				"char_limit", maxSegmentCharLength,
				"tag", p.currentTag)

			p.flushCurrentSegment()
			textToAppend = remainder
		} else {
			textToAppend = ""
		}
	}
}

// splitTextByPunctuation は文字数制限と句読点に基づき、テキストを分割します。
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

// flushCurrentSegment は現在のテキストバッファをセグメントとして確定します。
func (p *textParser) flushCurrentSegment() {
	if p.currentText.Len() > 0 && p.currentTag != "" {
		p.addSegment(p.currentTag, p.currentText.String())
	}
	p.currentText.Reset()
}

// addSegment はテキストのクレンジングを行い、セグメントリストに追加します。
func (p *textParser) addSegment(tag string, text string) {
	finalText := reEmotionParse.ReplaceAllString(text, "")
	finalText = strings.TrimSpace(finalText)

	if finalText != "" {
		baseTag := ""
		baseMatch := reBaseSpeakerTag.FindStringSubmatch(tag)

		if len(baseMatch) > 1 {
			baseTag = baseMatch[1]
		} else {
			slog.Warn("SpeakerTagからBaseSpeakerTagの抽出に失敗しました。SpeakerTag全体をBaseSpeakerTagとして使用します。", "tag", tag)
			baseTag = tag
		}

		p.segments = append(p.segments, contracts.Segment{
			SpeakerTag:     tag,
			BaseSpeakerTag: baseTag,
			Text:           finalText,
		})
	}
}

// finishParsing は解析終了時のバッファ残処理を行います。
func (p *textParser) finishParsing() {
	p.flushCurrentSegment()

	if p.textBuffer != "" {
		if len(p.segments) > 0 {
			lastTag := p.segments[len(p.segments)-1].SpeakerTag
			slog.Warn("スクリプトの最後にタグのないテキストが残りました。最後のタグを流用します。",
				"lost_text", p.textBuffer, "used_tag", lastTag)
			p.addSegment(lastTag, p.textBuffer)
		} else {
			if p.fallbackTag != "" {
				slog.Warn("デフォルトタグを使用してテキスト全体を合成します。", "default_tag", p.fallbackTag)
				p.addSegment(p.fallbackTag, p.textBuffer)
			} else {
				slog.Error("タグおよびフォールバックタグが見つかりません。")
			}
		}
	}
}
