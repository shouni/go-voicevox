package ports

import "context"

// EngineRunner はスクリプトから音声ファイルを生成するためのインターフェースです。
type EngineRunner interface {
	Run(ctx context.Context, outputURI, content string, opts ...RunOption) error
}

// APIClient は Client が満たすべき API 呼び出しインターフェース
type APIClient interface {
	// RunAudioQuery は指定されたテキストとスタイルIDから音声合成クエリを生成し、クエリデータをバイト列として返します。
	RunAudioQuery(ctx context.Context, text string, styleID int) ([]byte, error)
	// RunSynthesis は提供されたクエリデータから音声を合成し、合成された音声データをバイト列として返します。
	RunSynthesis(ctx context.Context, queryBody []byte, styleID int) ([]byte, error)
	// GetSpeakers は利用可能な話者のリストをバイト列として取得します。
	GetSpeakers(ctx context.Context) ([]byte, error)
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
