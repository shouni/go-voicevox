package ports

import "context"

// EngineRunner はスクリプトから音声ファイルを生成するためのインターフェースです。
type EngineRunner interface {
	Run(ctx context.Context, outputURI, content string, opts ...RunOption) error
}
