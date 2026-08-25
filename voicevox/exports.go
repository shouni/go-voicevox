// Package voicevox は、VOICEVOX エンジンを利用した音声合成の公開APIです。
// 内部パッケージの型を再公開し、利用側が internal を意識せずに済むようにしています。
package voicevox

import (
	"context"

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

// ScriptLine は、話者・スタイル・本文からなる入力の1行です。
//
// **公開するのは、公開 API から実際に触れるものだけです。** engine が持つ
// AudioQueryClient / StyleFinder といった依存の口は再公開しません。New が
// クライアントを自前で組み立てる以上、差し替える口が無く、並べておくと
// 「実装すれば差し替えられる」という誤った期待を与えるためです。
// 内部のセグメント型も同様で、タグを組み立てるのは内部の仕事です。入口はこの型です。
//
// **既定値の定数も公開しません。** かつて DefaultMaxParallelSegments などを
// 並べていましたが、既定を使いたい呼び出し側はオプションを渡さなければ済むので、
// WithX(既定値) は何もしない呼び出しにしかなりませんでした。既定の実際の値は
// 各オプションのコメントにあります（options.go）。
type ScriptLine = internalengine.ScriptLine
