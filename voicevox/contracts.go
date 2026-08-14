// Package voicevox は、VOICEVOX エンジンを利用した音声合成の公開APIです。
// 内部パッケージの型を再公開し、利用側が internal を意識せずに済むようにしています。
package voicevox

import (
	"time"

	"github.com/shouni/go-voicevox/internal/contracts"
)

// 内部パッケージ contracts の型をそのまま公開するエイリアスです。
//
// **公開するのは、公開 API から実際に触れるものだけです。**
// AudioQueryClient / SpeakerClient / DataFinder といった内部の継ぎ目は再公開しません。
// New がクライアントを自前で組み立てる以上、差し替える口が無く、
// 並べておくと「実装すれば差し替えられる」という誤った期待を与えるためです。
// Segment も同様で、タグを組み立てるのは内部の仕事です。入口は ScriptLine です。
type (
	// Engine は、スクリプト行から結合済みWAVを生成する合成エンジンです。
	Engine = contracts.Engine
	// ScriptLine は、話者・スタイル・本文からなる入力の1行です。
	ScriptLine = contracts.ScriptLine
	// Option は、EngineConfig を変更する関数型です。
	Option = contracts.Option
)

// エンジンの既定値です。値の出所は内部パッケージ contracts に一本化しています。
const (
	// DefaultMaxParallelSegments は、セグメント合成の既定の並列数です。
	DefaultMaxParallelSegments = contracts.DefaultMaxParallelSegments
	// DefaultSegmentTimeout は、セグメント1件あたりの既定のタイムアウトです。
	DefaultSegmentTimeout = contracts.DefaultSegmentTimeout
	// DefaultSegmentRateLimit は、セグメント合成の既定のレート制限間隔です。
	DefaultSegmentRateLimit = contracts.DefaultSegmentRateLimit
)

// WithMaxParallelSegments は、セグメント合成の並列数を設定します。
func WithMaxParallelSegments(n int) Option {
	return contracts.WithMaxParallelSegments(n)
}

// WithSegmentTimeout は、セグメント1件あたりのタイムアウトを設定します。
func WithSegmentTimeout(dur time.Duration) Option {
	return contracts.WithSegmentTimeout(dur)
}

// WithSegmentRateLimit は、セグメント合成のレート制限間隔を設定します。
func WithSegmentRateLimit(dur time.Duration) Option {
	return contracts.WithSegmentRateLimit(dur)
}
