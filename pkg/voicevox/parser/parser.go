package parser

import (
	"log/slog"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Parser は、様々な形式の入力から音声合成用のセグメントを解析するインターフェースです。
type Parser interface {
	Parse(scriptContent string, fallbackTag string) ([]Segment, error)
}

// ----------------------------------------------------------------------
// データモデル (スクリプト処理)
// ----------------------------------------------------------------------

// Segment は解析されたスクリプトの一片を表す構造体です。
// BaseSpeakerTag はスタイルタグを含まない話者名 ([ずんだもん]) を格納します。
type Segment struct {
	SpeakerTag     string // 例: "[ずんだもん][ノーマル]"
	BaseSpeakerTag string // 例: "[ずんだもん]"
	Text           string
}

var (
	// スクリプトの基本形式: [話者タグ][スタイルタグ] テキスト
	reScriptParse = regexp.MustCompile(`^(\[.+?\])\s*(\[.+?\])\s*(.*)`)
	// テキストから感情タグを取り除くための正規表現
	reEmotionParse = regexp.MustCompile(`\[` + EmotionTagsPattern + `\]`)
	// BaseSpeakerTag 抽出のための正規表現: ^(\[.+?\])
	reBaseSpeakerTag = regexp.MustCompile(`^(\[.+?\])`)

	maxSegmentCharLength = MaxSegmentCharLength
)

// ----------------------------------------------------------------------
// textParser 構造体（Parser インターフェースの実装）
// ----------------------------------------------------------------------

// textParser はスクリプトの解析状態を管理し、セグメント化を実行します。
type textParser struct {
	segments    []Segment
	currentTag  string
	currentText *strings.Builder
	textBuffer  string
	fallbackTag string
}

// NewParser は textParser インスタンスを生成し、Parser インターフェースとして返します。
func NewParser() *textParser {
	return &textParser{
		currentText: &strings.Builder{},
	}
}

// Parse は Parser インターフェースのメソッド実装です。
func (p *textParser) Parse(scriptContent string, fallbackTag string) ([]Segment, error) {
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

	// エラー処理は内部でログ出力しているため、ここでは nil を返す設計を維持
	return p.segments, nil
}

// ----------------------------------------------------------------------
// 内部処理ロジック
// ----------------------------------------------------------------------

// processLine はスクリプトの1行を処理します。
func (p *textParser) processLine(line string) {
	if line == "" {
		return
	}

	textToProcess := line
	if p.textBuffer != "" {
		// バッファされたテキストがある場合、結合時にスペースを入れる
		textToProcess = p.textBuffer + " " + line
		p.textBuffer = ""
	}

	matches := reScriptParse.FindStringSubmatch(textToProcess)
	if len(matches) > 3 {
		speakerTag := matches[1] // 例: [ずんだもん]
		vvStyleTag := matches[2] // 例: [ノーマル]
		textPart := matches[3]
		newCombinedTag := speakerTag + vvStyleTag // 例: [ずんだもん][ノーマル]
		p.processTaggedLine(newCombinedTag, textPart)
	} else {
		p.processUntaggedLine(textToProcess)
	}
}

// processTaggedLine はタグ付きの行を処理します。
func (p *textParser) processTaggedLine(tag, text string) {
	// 既存のセグメントがある場合、強制的に確定（一行一セグメントを強制する設計）
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
		// タグなしの行をバッファリングし、次のタグ付きセグメントに結合
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

// splitTextByPunctuation は、現在のセグメントの文字数制限と句読点に基づき、追記するテキストを分割します。
func (p *textParser) splitTextByPunctuation(text string) (partToAdd string, remainder string) {
	currentRuneCount := utf8.RuneCountInString(p.currentText.String())

	space := 0
	if currentRuneCount > 0 {
		space = 1
	}

	// 結合後の文字数が制限内ならそのまま返す
	if currentRuneCount+space+utf8.RuneCountInString(text) <= maxSegmentCharLength {
		return text, ""
	}

	// 現在のバッファに追加できる残りの文字数
	maxCapacity := maxSegmentCharLength - currentRuneCount - space

	if maxCapacity <= 0 {
		// currentText が既に文字数を超えている場合 (エラーケースだが、次のセグメントとして全量を残す)
		return "", text
	}

	runes := []rune(text)
	bestSplitIndex := -1

	// 句読点で分割できる最適な位置を探す
	for i := 0; i < len(runes); i++ {
		if currentRuneCount+space+(i+1) > maxSegmentCharLength {
			break
		}

		r := runes[i]
		// 句読点（。、！？）で分割
		if r == '。' || r == '、' || r == '！' || r == '？' {
			bestSplitIndex = i + 1
		}
	}

	if bestSplitIndex > 0 {
		// 最適な句読点分割を適用
		partToAdd = string(runes[:bestSplitIndex])
		remainder = string(runes[bestSplitIndex:])
		return partToAdd, remainder
	}

	// 句読点が見つからなかった、または制限を超えてしまう場合、強制的に maxCapacity で分割
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
	// 感情タグを削除し、トリム
	finalText := reEmotionParse.ReplaceAllString(text, "")
	finalText = strings.TrimSpace(finalText)

	if finalText != "" {
		// BaseSpeakerTag を計算 (タグの最初の [..] 部分を抽出)
		baseTag := ""
		baseMatch := reBaseSpeakerTag.FindStringSubmatch(tag)
		if len(baseMatch) > 1 {
			baseTag = baseMatch[1] // 例: "[ずんだもん][ノーマル]" から "[ずんだもん]" を抽出
		} else {
			slog.Error("SpeakerTagからBaseSpeakerTagの抽出に失敗しました。", "tag", tag)
			// 抽出失敗時は BaseTag を空のままにするか、SpeakerTag全体を使用する
		}

		p.segments = append(p.segments, Segment{
			SpeakerTag:     tag,
			BaseSpeakerTag: baseTag,
			Text:           finalText,
		})
	}
}

// finishParsing は解析終了時に残っているバッファを処理します。
func (p *textParser) finishParsing() {
	p.flushCurrentSegment()

	if p.textBuffer != "" {
		if len(p.segments) > 0 {
			// 既存のセグメントがある場合、最後のタグを流用
			lastTag := p.segments[len(p.segments)-1].SpeakerTag
			slog.Warn("スクリプトの最後にタグのないテキストが残りました。最後のタグを流用して最終セグメントとして合成します。",
				"lost_text", p.textBuffer, "used_tag", lastTag)
			p.addSegment(lastTag, p.textBuffer)
		} else {
			// 既存のセグメントがない場合、フォールバックタグを使用
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
