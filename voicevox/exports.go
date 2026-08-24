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
// AudioQueryClient / StyleFinder といった依存の口は再公開しません。New が
// クライアントを自前で組み立てる以上、差し替える口が無く、並べておくと
// 「実装すれば差し替えられる」という誤った期待を与えるためです。
// 内部のセグメント型も同様で、タグを組み立てるのは内部の仕事です。入口は ScriptLine です。
//
// **既定値の定数も公開しません。** かつて DefaultMaxParallelSegments などを
// 並べていましたが、既定を使いたい呼び出し側はオプションを渡さなければ済むので、
// WithX(DefaultX) は何もしない呼び出しにしかなりませんでした。既定の実際の値は
// 各オプションのコメントにあります。
type (
	// ScriptLine は、話者・スタイル・本文からなる入力の1行です。
	ScriptLine = internalengine.ScriptLine
	// Option は、エンジンの動作設定を変更する関数型です。
	Option = internalengine.Option
)

// WithMaxParallelSegments は、セグメント合成の並列数を設定します（既定 5、0以下は無視）。
func WithMaxParallelSegments(n int) Option {
	return internalengine.WithMaxParallelSegments(n)
}

// WithSegmentTimeout は、セグメント1件あたりのタイムアウトを設定します（既定 180 秒、0以下は無視）。
func WithSegmentTimeout(dur time.Duration) Option {
	return internalengine.WithSegmentTimeout(dur)
}

// WithSegmentRateLimit は、セグメント合成のレート制限間隔を設定します（既定 100ms、0以下は無視）。
//
// **スループットを上げる目的では使えません。** 同時実行数は
// WithMaxParallelSegments が縛っており、この間隔は起動時の一斉接続をならすだけです。
func WithSegmentRateLimit(dur time.Duration) Option {
	return internalengine.WithSegmentRateLimit(dur)
}
