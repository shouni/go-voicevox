package voicevox

import (
	"context"
	"sync"
	"time"

	"github.com/shouni/go-voicevox/pkg/voicevox/parser"
)

// ----------------------------------------------------------------------
// インターフェース
// ----------------------------------------------------------------------

// EngineExecutor は、スクリプトを実行して音声ファイルを生成するための契約を定義します。
// オプションの処理（例: フォールバックタグ）は、Functional Options Patternを通じて提供されます。
type EngineExecutor interface {
	// Execute はスクリプトを実行し、WAVファイルを生成します。
	// opts には ExecuteOption 型の可変長引数を取ります。
	Execute(ctx context.Context, scriptContent string, outputWavFile string, opts ...ExecuteOption) error
}

// DataFinder は、Engine が Style ID を検索するために SpeakerData に要求するメソッドを定義します。
type DataFinder interface {
	GetStyleID(combinedTag string) (int, bool)
	GetDefaultTag(speakerToolTag string) (string, bool)
}

type Engine struct {
	client AudioQueryClient
	data   DataFinder
	parser parser.Parser
	config EngineConfig

	// 内部キャッシュ状態
	styleIDCache      map[string]int
	styleIDCacheMutex sync.RWMutex
}

type EngineConfig struct {
	MaxParallelSegments int
	SegmentTimeout      time.Duration
}

// AudioQueryClient は Client が満たすべき API 呼び出しインターフェース
type AudioQueryClient interface {
	RunAudioQuery(text string, styleID int, ctx context.Context) ([]byte, error)
	RunSynthesis(queryBody []byte, styleID int, ctx context.Context) ([]byte, error)
}

// segmentResult は Goroutineの結果を格納します。
type segmentResult struct {
	index   int
	wavData []byte
	err     error
}
