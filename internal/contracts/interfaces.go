package contracts

import "context"

type Engine interface {
	Run(ctx context.Context, outputURI, content string, opts ...RunOption) error
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
