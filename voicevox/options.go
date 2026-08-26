package voicevox

import (
	"time"

	"github.com/shouni/audio/phonetic"

	internalengine "github.com/shouni/go-voicevox/internal/engine"
)

// Option は、New が組み立てるエンジンの動作設定を変更します。
//
// **internal/engine の Option の別名ではありません。** 設定の宛先が 1 つではないためです。
// 並列数・タイムアウト・レート制限は合成本体（internal/engine）が読みますが、読みの
// 上書きは New が組み立てる読み変換器（audio/phonetic）が読みます。別名のままにすると、
// engine の Config へ engine が誰も読まないフィールドを置くことになり、
// 「設定はできるが見る者がいない」という、このリポジトリが繰り返し削ってきた形になります。
type Option func(*options)

// options は、New が受け取った設定を宛先ごとに束ねたものです。
type options struct {
	engine    []internalengine.Option
	converter []phonetic.Option
}

// newOptions は、渡された Option を宛先ごとに振り分けます。
func newOptions(opts ...Option) options {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// WithMaxParallelSegments は、セグメント合成の並列数を設定します（既定 5、0以下は無視）。
func WithMaxParallelSegments(n int) Option {
	return func(o *options) {
		o.engine = append(o.engine, internalengine.WithMaxParallelSegments(n))
	}
}

// WithSegmentTimeout は、セグメント1件あたりのタイムアウトを設定します（既定 180 秒、0以下は無視）。
func WithSegmentTimeout(dur time.Duration) Option {
	return func(o *options) {
		o.engine = append(o.engine, internalengine.WithSegmentTimeout(dur))
	}
}

// WithSegmentRateLimit は、セグメント合成のレート制限間隔を設定します（既定 100ms、0以下は無視）。
//
// **スループットを上げる目的では使えません。** 同時実行数は
// WithMaxParallelSegments が縛っており、この間隔は起動時の一斉接続をならすだけです。
func WithSegmentRateLimit(dur time.Duration) Option {
	return func(o *options) {
		o.engine = append(o.engine, internalengine.WithSegmentRateLimit(dur))
	}
}

// WithNumberReading は、算用数字を日本語の読みへ変換します。
//
// 形態素解析器の辞書は算用数字に読みを持たないため、**既定では "2026年8月" が
// "2026ネン8ツキ" になります。** この設定を付けると "ニセンニジュウロクネンハチガツ"
// のように、数の読みと助数詞の音の変化（一回→イッカイ、三本→サンボン）まで当てます。
//
// 日付・人数・年齢を含む台本では、これが無いと VOICEVOX が数字を字面どおりに読みます
// （8日→ハチニチ、1人→イチニン、20歳→ニジュッサイ）。語ごとに WithReadingOverrides で
// 与えることもできますが、数は無限にあるので規則で当てるほうが先です。
//
// **既定で有効にはしていません。** 読みが変わるのは観測できる挙動の変化なので、
// 有効にするかどうかは呼び出し側が選びます。
func WithNumberReading() Option {
	return func(o *options) {
		o.converter = append(o.converter, phonetic.WithNumberReading())
	}
}

// WithReadingOverrides は、読み変換で使う表層形ごとの読みを追加します。
// キーは本文に現れる表記、値はその読み（カタカナ）です。
//
//	voicevox.WithReadingOverrides(map[string]string{"8日": "ヨウカ", "1人": "ヒトリ"})
//
// **固有名詞と、規則で当たらない語のためにあります。** 数字と助数詞は
// WithNumberReading が規則で読むので、そちらで足ります。ここで与えるのは、作品ごとの
// 人名・地名や、規則では当たらない読み（8日→ヨウカ のような特別な読み方）です。
// プロンプトで「カタカナで書かせる」のは台本そのものを歪めますし、変換結果の確認では
// 元の表記のまま見えるため、合成するまで誤読に気づけません。
//
// 同梱の既定辞書に**重ねて**適用され、同じ表記を指定した場合はこちらが勝ちます。
// 空のキーまたは値は無視されます。複数回指定した場合は後の指定が優先されます。
//
// **どの語をどう読ませるかはアプリケーションの語彙です。** 話者一覧と同じ理由で、
// ライブラリは中身を持たず受け取るだけにしてあります。作品ごとに固有名詞は違いますし、
// 語を 1 つ足すたびにライブラリのリリースが要るのでは追いつきません。
func WithReadingOverrides(overrides map[string]string) Option {
	return func(o *options) {
		if len(overrides) == 0 {
			return
		}
		o.converter = append(o.converter, phonetic.WithReadingOverrides(overrides))
	}
}
