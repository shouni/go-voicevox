package voicevox

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// NOTE: WAVヘッダーに関するすべての定数（RiffChunkIDSize, WavTotalHeaderSizeなど）は
// const.go に移動されたと仮定し、ここでは直接利用しています。

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
		_, currentAudioData, err := extractAudioData(currentWav, i)
		if err != nil {
			// i + 1 は元のセグメント番号
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
// audio.go の元のロジックに従い、プライベート関数（小文字開始）とします。
func extractAudioData(wavBytes []byte, index int) (formatHeader []byte, audioData []byte, err error) {
	// NOTE: 実際には、以下の定数は const.go に定義されている必要があります。
	const (
		WavTotalHeaderSize  = 44
		DataChunkHeaderSize = 8
		FmtChunkSize        = 16
		FmtChunkOffset      = 12 // "fmt "チャンクの開始位置
		DataChunkOffset     = 36 // "data" チャンクの開始位置 (WavTotalHeaderSize - DataChunkHeaderSize)
	)

	if len(wavBytes) < WavTotalHeaderSize {
		// ErrInvalidWAVHeader を利用
		return nil, nil, &ErrInvalidWAVHeader{
			Index:   index,
			Details: fmt.Sprintf("WAVファイルサイズが短すぎます (%dバイト)", len(wavBytes)),
		}
	}

	// フォーマットヘッダーを抽出 (RIFFからdataチャンク開始直前まで)
	formatHeader = wavBytes[0:DataChunkOffset]

	// "data" チャンクヘッダー（36バイト目から）を読み取り、データ長を取得
	dataChunkHeader := wavBytes[DataChunkOffset : DataChunkOffset+DataChunkHeaderSize]

	// dataチャンクIDの確認（省略可能だが、ここではデータ長抽出にフォーカス）

	// dataチャンクのサイズを読み取り (リトルエンディアン)
	dataSize := binary.LittleEndian.Uint32(dataChunkHeader[4:8])

	// データ部分を抽出
	audioDataStart := WavTotalHeaderSize
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
// audio.go の元のロジックに従い、プライベート関数（小文字開始）とします。
func buildCombinedWav(formatHeader, combinedAudioData []byte, totalAudioSize int) ([]byte, error) {
	// NOTE: 実際には、以下の定数は const.go に定義されている必要があります。
	const (
		WavTotalHeaderSize  = 44
		DataChunkHeaderSize = 8
		FmtChunkSize        = 16
		RiffChunkSizeOffset = 4
		DataChunkSizeOffset = 40 // WavTotalHeaderSize - 4
	)

	// 最終的なWAVファイルの総サイズ
	fileSize := totalAudioSize + WavTotalHeaderSize - 8 // RIFFチャンクサイズはファイルサイズ-8バイト

	// 新しいヘッダーをコピー
	combinedWav := make([]byte, WavTotalHeaderSize+totalAudioSize)
	copy(combinedWav, formatHeader)

	// RIFFチャンクサイズ (File Size - 8) の更新 (4-8バイト目)
	binary.LittleEndian.PutUint32(combinedWav[RiffChunkSizeOffset:RiffChunkSizeOffset+4], uint32(fileSize))

	// dataチャンクサイズ (Audio Data Size) の更新 (40-44バイト目)
	binary.LittleEndian.PutUint32(combinedWav[DataChunkSizeOffset:DataChunkSizeOffset+4], uint32(totalAudioSize))

	// オーディオデータ本体をコピー
	copy(combinedWav[WavTotalHeaderSize:], combinedAudioData)

	return combinedWav, nil
}
