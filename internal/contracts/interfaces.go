package contracts

import (
	"context"
	"io"
)

type Engine interface {
	Run(ctx context.Context, outputURI, content string, opts ...RunOption) error
	// RunScript は、構造化された ScriptLine を直接受け取り、テキストパーサーを経由せずに
	// 音声合成を実行します。
	RunScript(ctx context.Context, outputURI string, lines []ScriptLine, opts ...RunOption) error
}

type AudioQueryClient interface {
	RunAudioQuery(ctx context.Context, text string, styleID int) ([]byte, error)
	RunSynthesis(ctx context.Context, queryBody []byte, styleID int) ([]byte, error)
}

type SpeakerClient interface {
	GetSpeakers(ctx context.Context) ([]byte, error)
}

type APIClient interface {
	AudioQueryClient
	SpeakerClient
}

type DataFinder interface {
	GetStyleID(combinedTag string) (int, bool)
	GetDefaultTag(speakerToolTag string) (string, bool)
}

type Parser interface {
	Parse(scriptContent string, fallbackTag string) ([]Segment, error)
}

type Writer interface {
	Write(ctx context.Context, path string, contentReader io.Reader, opts ...WriteOption) error
}
