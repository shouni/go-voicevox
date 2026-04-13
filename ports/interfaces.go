package ports

import (
	"context"
	"io"
)

// Engine はスクリプトから音声ファイルを生成するためのインターフェースです。
type Engine interface {
	Run(ctx context.Context, outputURI, content string, opts ...RunOption) error
}

// AudioQueryClient は音声合成クエリの生成と音声合成を実行するためのインターフェースです。
type AudioQueryClient interface {
	RunAudioQuery(ctx context.Context, text string, styleID int) ([]byte, error)
	RunSynthesis(ctx context.Context, queryBody []byte, styleID int) ([]byte, error)
}

// SpeakerClient は /speakers エンドポイントを呼び出す能力を抽象化するインターフェースです。
type SpeakerClient interface {
	GetSpeakers(ctx context.Context) ([]byte, error)
}

// APIClient は Client が満たすべきすべての API 呼び出し（音声合成および話者取得）を統合したインターフェースです。
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

// Writer は単一のリソースを書き込む機能に特化します
type Writer interface {
	// Write は、指定された path に応じて GCS、S3、またはローカルファイルへデータを書き込みます。
	Write(ctx context.Context, path string, contentReader io.Reader, contentType string) error
}
