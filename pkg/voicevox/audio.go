package voicevox

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// ----------------------------------------------------------------------
// 公開ロジック
// ----------------------------------------------------------------------

// combineWavData は複数のWAVデータ（バイトスライス）を結合し、
// 正しいヘッダーを持つ単一のWAVファイル（バイトスライス）を生成します。
// 最初のWAVファイルからフォーマット情報（サンプリングレート、チャンネル数など）を抽出します。
func combineWavData(wavDataList [][]byte) ([]byte, error) {
	if len(wavDataList) == 0 {
		// ErrNoAudioData を利用
		return nil, &ErrNoAudioData{}
	}

	// 1. 最初のWAVからフォーマット情報を抽出
	firstWav := wavDataList[0]
	// 最初のWAVファイルはインデックス0で解析
	formatHeader, audioData, err := extractAudioData(firstWav, 0)
	if err != nil {
		return nil, fmt.Errorf("最初のWAVファイルの解析に失敗しました: %w", err)
	}

	// 2. すべてのオーディオデータを連結
	var audioDataWriter bytes.Buffer
	totalAudioSize := len(audioData)
	audioDataWriter.Write(audioData)

	// 2番目以降のWAVデータを処理
	for i := 1; i < len(wavDataList); i++ {
		currentWav := wavDataList[i]
		// i + 1 は元のセグメント番号
		_, currentAudioData, err := extractAudioData(currentWav, i+1)
		if err != nil {
			return nil, fmt.Errorf("WAVファイル #%d の解析に失敗しました: %w", i+1, err)
		}

		audioDataWriter.Write(currentAudioData)
		totalAudioSize += len(currentAudioData)
	}

	// 3. 結合されたデータと最初のフォーマットヘッダーから新しいWAVファイルを構築
	combinedWavBytes, err := buildCombinedWav(formatHeader, audioDataWriter.Bytes(), totalAudioSize)
	if err != nil {
		return nil, fmt.Errorf("最終的なWAVファイルの構築に失敗しました: %w", err)
	}

	return combinedWavBytes, nil
}

// ----------------------------------------------------------------------
// 内部ヘルパー関数 (プライベート化)
// ----------------------------------------------------------------------

// extractAudioData はWAVファイルバイトスライスからフォーマットヘッダー情報とオーディオデータ部分を抽出します。
func extractAudioData(wavBytes []byte, index int) (formatHeader []byte, audioData []byte, err error) {
	// ⬅️ const.go からインポートされた定数を使用
	// FmtChunkOffset は 12、DataChunkOffset は 36

	if len(wavBytes) < WavTotalHeaderSize {
		// ErrInvalidWAVHeader を利用
		return nil, nil, &ErrInvalidWAVHeader{
			Index:   index,
			Details: fmt.Sprintf("WAVファイルサイズが短すぎます (%dバイト)", len(wavBytes)),
		}
	}

	// フォーマットヘッダーを抽出 (RIFFからdataチャンク開始直前まで)
	// WavTotalHeaderSize - DataChunkHeaderSize = 36 (DataChunkOffset)
	formatHeader = wavBytes[0:DataChunkOffset]

	// "data" チャンクヘッダー（DataChunkOffset (36) から）を読み取り、データ長を取得
	dataChunkHeader := wavBytes[DataChunkOffset : DataChunkOffset+DataChunkHeaderSize]

	// dataチャンクのサイズを読み取り (リトルエンディアン)
	// dataChunkHeader[4:8] は DataChunkSizeOffset (40) の位置
	dataSize := binary.LittleEndian.Uint32(dataChunkHeader[DataChunkIDSize:DataChunkHeaderSize]) // 4:8バイト目の4バイト

	// データ部分を抽出
	audioDataStart := WavTotalHeaderSize // 44
	audioDataEnd := audioDataStart + int(dataSize)

	if audioDataEnd > len(wavBytes) {
		return nil, nil, &ErrInvalidWAVHeader{
			Index:   index,
			Details: "WAVヘッダーのデータ長がファイルサイズを超過しています",
		}
	}

	audioData = wavBytes[audioDataStart:audioDataEnd]

	// 抽出されたデータサイズがヘッダーの記載と一致するか最終確認
	if len(audioData) != int(dataSize) {
		return nil, nil, &ErrInvalidWAVHeader{
			Index:   index,
			Details: fmt.Sprintf("抽出データサイズ (%d) がヘッダー記載サイズ (%d) と一致しません", len(audioData), dataSize),
		}
	}

	return formatHeader, audioData, nil
}

// buildCombinedWav はフォーマットヘッダー情報と結合されたオーディオデータから、
// 正しいヘッダーを持つ単一のWAVファイルを構築します。
func buildCombinedWav(formatHeader, combinedAudioData []byte, totalAudioSize int) ([]byte, error) {
	// ⬅️ const.go からインポートされた定数を使用

	// 最終的なWAVファイルの総サイズ
	// RIFFチャンクサイズは (totalAudioSize + WavTotalHeaderSize) - 8
	fileSize := totalAudioSize + WavTotalHeaderSize - (RiffChunkIDSize + WaveIDSize)

	// 新しいヘッダーをコピー
	combinedWav := make([]byte, WavTotalHeaderSize+totalAudioSize)
	copy(combinedWav, formatHeader)

	// RIFFチャンクサイズ (File Size - 8) の更新 (4-8バイト目)
	// ⬅️ const.go の RiffChunkSizeOffset (4) を使用
	binary.LittleEndian.PutUint32(combinedWav[RiffChunkSizeOffset:RiffChunkSizeOffset+4], uint32(fileSize))

	// dataチャンクサイズ (Audio Data Size) の更新 (40-44バイト目)
	// ⬅️ const.go の DataChunkSizeOffset (40) を使用
	binary.LittleEndian.PutUint32(combinedWav[DataChunkSizeOffset:DataChunkSizeOffset+4], uint32(totalAudioSize))

	// オーディオデータ本体をコピー
	copy(combinedWav[WavTotalHeaderSize:], combinedAudioData)

	return combinedWav, nil
}
