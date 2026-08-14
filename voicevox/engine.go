package voicevox

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shouni/audio/phonetic"
	"github.com/shouni/go-http-kit/httpkit"

	"github.com/shouni/go-voicevox/internal/api"
	"github.com/shouni/go-voicevox/internal/contracts"
	internalengine "github.com/shouni/go-voicevox/internal/engine"
	"github.com/shouni/go-voicevox/speaker"
)

const defaultVoicevoxAPIURL = "http://localhost:50021"

// New は、依存関係を組み立てて Engine を返します。
// Engine.Run は結合済みのWAVバイト列を返すのみで、出力先への保存は呼び出し側の責務です。
// speakers に *speaker.Registry を渡すと、その一覧に載っている話者・スタイルだけを使います。
// nil ならエンジンが提供するものをすべて受け入れます。どの話者を使うかはアプリケーションの
// 方針なので、一覧はこのライブラリではなく呼び出し側が持ちます。
//
// **合成を止める切り替えは持ちません。** かつて no-op の Engine を返す真偽値を
// 取っていましたが、呼び出し側は定数 true を書くだけで、無効の経路に入ることが
// ありませんでした。止めたい呼び出し側は New を呼ばなければ済みます。
func New(
	ctx context.Context,
	httpClient httpkit.Requester,
	voicevoxAPIURL string,
	speakers *speaker.Registry,
	opts ...Option,
) (Engine, error) {
	if voicevoxAPIURL == "" {
		voicevoxAPIURL = defaultVoicevoxAPIURL
		slog.Warn("VOICEVOX_API_URL が指定されていません。デフォルトを使用します。", "url", voicevoxAPIURL)
	}

	engineConfig := contracts.NewEngineConfig(opts...)

	voicevoxClient := api.New(httpClient, voicevoxAPIURL)

	slog.Info("VOICEVOX話者スタイルデータをロード中...")
	speakerData, err := speaker.LoadSpeakers(ctx, voicevoxClient, speakers)
	if err != nil {
		return nil, fmt.Errorf("VOICEVOXデータのロードに失敗しました: %w", err)
	}
	slog.Info("VOICEVOX話者スタイルデータのロード完了。", "styles_count", len(speakerData.StyleIDMap))

	converter, err := phonetic.NewConverter()
	if err != nil {
		return nil, fmt.Errorf("読み変換コンバータの初期化に失敗しました: %w", err)
	}

	engine := internalengine.NewWithConfig(
		voicevoxClient,
		speakerData,
		converter,
		engineConfig,
	)

	return engine, nil
}
