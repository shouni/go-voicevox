// Package contracts は、層をまたいで共有するインターフェースと型を定義します。
package contracts

import (
	"context"
)

// Engine は、スクリプト行から結合済みWAVを生成する合成エンジンです。
type Engine interface {
	// Run は、構造化された ScriptLine を受け取り、結合済みのWAVバイト列を返します。
	// 出力先への書き込みは呼び出し側の責務です。
	Run(ctx context.Context, lines []ScriptLine) ([]byte, error)
}

// AudioQueryClient は、音声クエリの作成と合成を行うAPIクライアントです。
type AudioQueryClient interface {
	RunAudioQuery(ctx context.Context, text string, styleID int) ([]byte, error)
	RunSynthesis(ctx context.Context, queryBody []byte, styleID int) ([]byte, error)
}

// DataFinder は、話者・スタイルのタグからスタイルIDを解決します。
type DataFinder interface {
	GetStyleID(combinedTag string) (int, bool)
	GetDefaultTag(speakerToolTag string) (string, bool)
}

// TextConverter は、合成前のセグメントテキストを VOICEVOX の誤読を避けるための
// 読み(カタカナ)へ変換します。
type TextConverter interface {
	ConvertToReading(text string) string
}
