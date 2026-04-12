package ports

import "context"

// Engine はスクリプトから音声ファイルを生成するためのインターフェースです。
type Engine interface {
	Run(ctx context.Context, outputURI, content string, opts ...RunOption) error
}

// AudioQueryClient は Client が満たすべき API 呼び出しインターフェース
type AudioQueryClient interface {
	RunAudioQuery(ctx context.Context, text string, styleID int) ([]byte, error)
	RunSynthesis(ctx context.Context, queryBody []byte, styleID int) ([]byte, error)
}

// SpeakerClient は /speakers エンドポイントを呼び出す能力を抽象化するインターフェースです。
type SpeakerClient interface {
	GetSpeakers(ctx context.Context) ([]byte, error)
}

// APIClient は Client が満たすべき API 呼び出しインターフェース
type APIClient interface {
	AudioQueryClient
	SpeakerClient
}

// DataFinder は、Engine が Style ID を検索するために SpeakerData に要求するメソッドを定義します。
type DataFinder interface {
	GetStyleID(combinedTag string) (int, bool)
	GetDefaultTag(speakerToolTag string) (string, bool)
}

// Parser は、様々な形式の入力から音声合成用のセグメントを解析するインターフェースです。
type Parser interface {
	// Parse はスクリプト内容を解析し、話者ごとのセグメントに分割して返します。
	Parse(scriptContent string, fallbackTag string) ([]Segment, error)
}
