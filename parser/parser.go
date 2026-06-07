package parser

import (
	"log/slog"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/shouni/audio/phonetic"

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

// Option は textParser の動作を調整します。
type Option func(*textParser)

// textParser はスクリプトを解析する stateless な Parser 実装です。
type textParser struct {
	preprocessText func(string) string
}

// parseSession は単一回の Parse 呼び出しに閉じた解析状態です。
type parseSession struct {
	segments       []contracts.Segment
	currentTag     string
	currentText    strings.Builder
	textBuffer     string
	fallbackTag    string
	preprocessText func(string) string
}

// NewParser は新しい textParser インスタンスを生成し、Parser インターフェースとして返します。
func NewParser(opts ...Option) *textParser {
	p := &textParser{}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// NewPhoneticParser はセグメント本文をカタカナ読みに変換する Parser を生成します。
func NewPhoneticParser() (contracts.Parser, error) {
	converter, err := phonetic.NewConverter()
	if err != nil {
		return nil, err
	}
	return NewParser(WithTextPreprocessor(converter.ConvertToReading)), nil
}

// WithTextPreprocessor はセグメント確定時に本文へ適用する前処理を設定します。
func WithTextPreprocessor(preprocess func(string) string) Option {
	return func(p *textParser) {
		if preprocess != nil {
			p.preprocessText = preprocess
		}
	}
}

// Parse は Parser インターフェースのメソッド実装です。
// 入力されたスクリプトを解析し、Segment のスライスを返します。
// fallbackTag には [話者][スタイル] の完全なタグを指定する必要があります。
//
// 呼び出しごとに内部状態（バッファや現在のタグなど）は完全にリセットされるため、
// 同じ Parser インスタンスを安全に再利用できます。
func (p *textParser) Parse(scriptContent string, fallbackTag string) ([]contracts.Segment, error) {
	session := newParseSession(fallbackTag, p.preprocessText)

	lines := strings.Split(scriptContent, "\n")
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}
		session.processLine(trimmedLine)
	}

	session.finish()
	return session.segments, nil
}

// processLine はスクリプトの1行を処理します。
func (s *parseSession) processLine(line string) {
	if line == "" {
		return
	}

	textToProcess := line
	if s.textBuffer != "" {
		textToProcess = s.textBuffer + " " + line
		s.textBuffer = ""
	}

	matches := reScriptParse.FindStringSubmatch(textToProcess)
	if len(matches) > 3 {
		speakerTag := matches[1]
		vvStyleTag := matches[2]
		textPart := matches[3]
		newCombinedTag := speakerTag + vvStyleTag
		s.processTaggedLine(newCombinedTag, textPart)
	} else {
		s.processUntaggedLine(textToProcess)
	}
}

// processTaggedLine はタグ付きの行を処理します。
func (s *parseSession) processTaggedLine(tag, text string) {
	if s.currentTag != "" {
		s.flushCurrentSegment()
	}
	s.currentTag = tag
	s.appendAndSplitText(text)
}

// processUntaggedLine はタグのない行を処理します。
func (s *parseSession) processUntaggedLine(text string) {
	if s.currentTag != "" {
		s.appendAndSplitText(text)
	} else {
		s.textBuffer = text
		slog.Warn("タグのないテキスト行が検出されました。次のタグ付きセグメントに結合されます。", "text", text)
	}
}

// appendAndSplitText はテキストを現在のセグメントに追記し、必要に応じて分割します。
func (s *parseSession) appendAndSplitText(text string) {
	textToAppend := text
	for textToAppend != "" {
		partToAdd, remainder := s.splitTextByPunctuation(textToAppend)

		if partToAdd != "" {
			if s.currentText.Len() > 0 {
				s.currentText.WriteString(" ")
			}
			s.currentText.WriteString(partToAdd)
		}

		if remainder != "" {
			slog.Warn("テキストが最大文字数を超過したため、セグメントを強制的に確定し、残りのテキストを分割します。",
				"char_limit", maxSegmentCharLength,
				"tag", s.currentTag)

			s.flushCurrentSegment()
			textToAppend = remainder
		} else {
			textToAppend = ""
		}
	}
}

// splitTextByPunctuation は文字数制限と句読点に基づき、テキストを分割します。
func (s *parseSession) splitTextByPunctuation(text string) (partToAdd string, remainder string) {
	currentRuneCount := utf8.RuneCountInString(s.currentText.String())
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
func (s *parseSession) flushCurrentSegment() {
	if s.currentText.Len() > 0 && s.currentTag != "" {
		s.addSegment(s.currentTag, s.currentText.String())
	}
	s.currentText.Reset()
}

// addSegment はテキストのクレンジングを行い、セグメントリストに追加します。
func (s *parseSession) addSegment(tag string, text string) {
	finalText := reEmotionParse.ReplaceAllString(text, "")
	finalText = strings.TrimSpace(finalText)
	if s.preprocessText != nil {
		finalText = s.preprocessText(finalText)
		finalText = strings.TrimSpace(finalText)
	}

	if finalText != "" {
		baseTag := ""
		baseMatch := reBaseSpeakerTag.FindStringSubmatch(tag)

		if len(baseMatch) > 1 {
			baseTag = baseMatch[1]
		} else {
			slog.Warn("SpeakerTagからBaseSpeakerTagの抽出に失敗しました。SpeakerTag全体をBaseSpeakerTagとして使用します。", "tag", tag)
			baseTag = tag
		}

		s.segments = append(s.segments, contracts.Segment{
			SpeakerTag:     tag,
			BaseSpeakerTag: baseTag,
			Text:           finalText,
		})
	}
}

// finishParsing は解析終了時のバッファ残処理を行います。
func (s *parseSession) finish() {
	s.flushCurrentSegment()

	if s.textBuffer != "" {
		if len(s.segments) > 0 {
			lastTag := s.segments[len(s.segments)-1].SpeakerTag
			slog.Warn("スクリプトの最後にタグのないテキストが残りました。最後のタグを流用します。",
				"lost_text", s.textBuffer, "used_tag", lastTag)
			s.addSegment(lastTag, s.textBuffer)
		} else {
			if s.fallbackTag != "" {
				slog.Warn("デフォルトタグを使用してテキスト全体を合成します。", "default_tag", s.fallbackTag)
				s.addSegment(s.fallbackTag, s.textBuffer)
			} else {
				slog.Error("タグおよびフォールバックタグが見つかりません。")
			}
		}
	}
}

func newParseSession(fallbackTag string, preprocessText func(string) string) *parseSession {
	return &parseSession{
		fallbackTag:    fallbackTag,
		preprocessText: preprocessText,
	}
}
