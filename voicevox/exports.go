// Package voicevox は、VOICEVOX エンジンを利用した音声合成の公開APIです。
// 内部パッケージの型を再公開し、利用側が internal を意識せずに済むようにしています。
package voicevox

import (
	"context"
	"time"

	internalengine "github.com/shouni/go-voicevox/internal/engine"
)

// Engine は、スクリプト行から結合済みWAVを生成する合成エンジンです。
//
// **この口は使う側のここで定義します。** 実装は internal/engine にありますが、
// 満たすかどうかは構造で決まるので、あちらがこの型を知る必要はありません。
type Engine interface {
	// Run は、構造化された ScriptLine を受け取り、結合済みのWAVバイト列を返します。
	// 出力先への書き込みは呼び出し側の責務です。
	Run(ctx context.Context, lines []ScriptLine) ([]byte, error)
}

// 内部実装の型をそのまま公開するエイリアスです。
//
// **公開するのは、公開 API から実際に触れるものだけです。** engine が持つ
// AudioQueryClient / DataFinder といった依存の口は再公開しません。New が
// クライアントを自前で組み立てる以上、差し替える口が無く、並べておくと
// 「実装すれば差し替えられる」という誤った期待を与えるためです。
// Segment も同様で、タグを組み立てるのは内部の仕事です。入口は ScriptLine です。
type (
	// ScriptLine は、話者・スタイル・本文からなる入力の1行です。
	ScriptLine = internalengine.ScriptLine
	// Option は、エンジンの動作設定を変更する関数型です。
	Option = internalengine.Option
)

// エンジンの既定値です。値の出所は内部パッケージ contracts に一本化しています。
const (
	// DefaultMaxParallelSegments は、セグメント合成の既定の並列数です。
	DefaultMaxParallelSegments = internalengine.DefaultMaxParallelSegments
	// DefaultSegmentTimeout は、セグメント1件あたりの既定のタイムアウトです。
	DefaultSegmentTimeout = internalengine.DefaultSegmentTimeout
	// DefaultSegmentRateLimit は、セグメント合成の既定のレート制限間隔です。
	DefaultSegmentRateLimit = internalengine.DefaultSegmentRateLimit
)

// WithMaxParallelSegments は、セグメント合成の並列数を設定します。
func WithMaxParallelSegments(n int) Option {
	return internalengine.WithMaxParallelSegments(n)
}

// WithSegmentTimeout は、セグメント1件あたりのタイムアウトを設定します。
func WithSegmentTimeout(dur time.Duration) Option {
	return internalengine.WithSegmentTimeout(dur)
}

// WithSegmentRateLimit は、セグメント合成のレート制限間隔を設定します。
func WithSegmentRateLimit(dur time.Duration) Option {
	return internalengine.WithSegmentRateLimit(dur)
}
