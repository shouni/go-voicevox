package audio

// ----------------------------------------------------------------------
// WAV ファイル定数 (動的チャンク探索ベースに統一)
// ----------------------------------------------------------------------

const (
	// RIFF 構造の必須サイズ定数
	RiffChunkIDSize   = 4 // "RIFF" チャンクIDのサイズ
	RiffChunkSizeSize = 4 // ファイルサイズフィールドのサイズ
	WaveIDSize        = 4 // "WAVE" 識別子のサイズ

	// データチャンクの必須サイズ定数
	DataChunkIDSize   = 4 // "data" チャンクIDのサイズ
	DataChunkSizeSize = 4 // データサイズフィールドのサイズ (DataChunkHeaderSize - DataChunkIDSize)
)

const (
	// 必須複合サイズ (ロジックで利用)
	DataChunkHeaderSize = DataChunkIDSize + DataChunkSizeSize              // "data"チャンクヘッダーの合計サイズ (8バイト)
	WavRiffHeaderSize   = RiffChunkIDSize + RiffChunkSizeSize + WaveIDSize // RIFFヘッダーの合計サイズ (12バイト)
	WavTotalHeaderSize  = 44
)

const (
	// 必須オフセット (ロジックで利用)
	// ファイル結合時に RIFF チャンクサイズを更新するために必要
	RiffChunkSizeOffset = RiffChunkIDSize // RIFFチャンクサイズが書き込まれるオフセット (4バイト目)
)
