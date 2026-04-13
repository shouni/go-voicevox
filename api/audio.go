package api

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

// RIFF 構造および WAV ファイルの解析に必要なサイズ定数です。
const (
	// riffChunkIDSize は "RIFF" チャンクIDのサイズ（バイト）です。
	riffChunkIDSize = 4
	// riffChunkSizeSize はファイルサイズフィールドのサイズ（バイト）です。
	riffChunkSizeSize = 4
	// waveIDSize は "WAVE" 識別子のサイズ（バイト）です。
	waveIDSize = 4

	// dataChunkIDSize は "data" チャンクIDのサイズ（バイト）です。
	dataChunkIDSize = 4
	// dataChunkSizeSize はデータサイズフィールドのサイズ（バイト）です。
	dataChunkSizeSize = 4
)

// WAV ファイルのヘッダー計算やロジックで使用される複合サイズ定数です。
const (
	// dataChunkHeaderSize は "data" チャンクヘッダーの合計サイズ（8バイト）です。
	dataChunkHeaderSize = dataChunkIDSize + dataChunkSizeSize
	// wavRiffHeaderSize は RIFF ヘッダーの合計サイズ（12バイト）です。
	wavRiffHeaderSize = riffChunkIDSize + riffChunkSizeSize + waveIDSize
	// wavTotalHeaderSize は一般的な WAV ファイルの最小ヘッダーサイズ（44バイト）です。
	wavTotalHeaderSize = 44
)

// ファイルのバイナリ操作時に使用されるオフセット定数です。
const (
	// riffChunkSizeOffset は、ファイル結合時に RIFF チャンクサイズを更新するために必要な、
	// RIFF チャンクサイズが書き込まれるオフセット位置（4バイト目）です。
	riffChunkSizeOffset = riffChunkIDSize
)

// ErrNoAudioData は、結合対象の音声データがない場合に発生します。
type ErrNoAudioData struct{}

func (e *ErrNoAudioData) Error() string {
	return "結合対象の音声データがありません"
}

// ErrInvalidWAVHeader は、WAV ヘッダーの検証に失敗した場合に発生します。
type ErrInvalidWAVHeader struct {
	// Index はエラーが発生した WAV ファイルのインデックスです。
	Index int
	// Details はエラーの詳細情報です。
	Details string
}

func (e *ErrInvalidWAVHeader) Error() string {
	if e.Index >= 0 {
		return fmt.Sprintf("WAVファイル #%d のヘッダーが無効です: %s", e.Index, e.Details)
	}
	return fmt.Sprintf("WAVヘッダーが無効です: %s", e.Details)
}

// CombineWavData は複数の WAV データ（バイト列）を結合し、
// 正しいヘッダーを持つ単一の WAV データを生成します。
// 最初の WAV ファイルからフォーマット情報（サンプリングレート等）を抽出し、
// 以降のデータの音声部分のみを連結します。
func (c *Client) CombineWavData(wavDataList [][]byte) ([]byte, error) {
	if len(wavDataList) == 0 {
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

	for i := 1; i < len(wavDataList); i++ {
		currentWav := wavDataList[i]
		_, currentAudioData, err := extractAudioData(currentWav, i)
		if err != nil {
			return nil, fmt.Errorf("WAVファイル #%d の解析に失敗しました: %w", i, err)
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

// extractAudioData は WAV ファイルからフォーマットヘッダー情報と音声データ部分を抽出します。
// fmt および data チャンクを動的に探索し、data チャンクの直前までを formatHeader とします。
func extractAudioData(wavBytes []byte, index int) (formatHeader []byte, audioData []byte, err error) {
	if len(wavBytes) < wavRiffHeaderSize {
		return nil, nil, &ErrInvalidWAVHeader{
			Index:   index,
			Details: fmt.Sprintf("WAVファイルサイズが短すぎます (RIFFヘッダー不足: %dバイト)", len(wavBytes)),
		}
	}

	var fmtChunkFound, dataChunkFound bool
	var dataChunkStart int

	offset := wavRiffHeaderSize

	for offset < len(wavBytes) {
		if offset+dataChunkHeaderSize > len(wavBytes) {
			break
		}

		chunkID := string(wavBytes[offset : offset+dataChunkIDSize])
		chunkSize := binary.LittleEndian.Uint32(wavBytes[offset+dataChunkIDSize : offset+dataChunkHeaderSize])

		if chunkID == "fmt " {
			fmtChunkFound = true
		}

		if chunkID == "data" {
			dataChunkFound = true
			dataChunkStart = offset

			audioDataStart := offset + dataChunkHeaderSize
			// audioDataStart + int(chunkSize) > len(wavBytes) という比較は
			// chunkSize が巨大な場合に int の範囲を超えて負数になるリスクがあるため、
			// 以下の通り「現在のスライスの残り容量」と uint32 のまま比較を行います。
			remainingBytes := uint32(len(wavBytes) - audioDataStart)
			if chunkSize > remainingBytes {
				return nil, nil, &ErrInvalidWAVHeader{
					Index:   index,
					Details: "dataチャンクのデータ長が実際のファイルサイズを超過しています",
				}
			}

			audioDataEnd := audioDataStart + int(chunkSize)
			audioData = wavBytes[audioDataStart:audioDataEnd]
			break
		}

		// 次のチャンクへ移動
		// chunkSize が巨大な場合のオーバーフローを考慮し、ここでもチェックが必要です
		nextOffset := uint64(offset) + uint64(dataChunkHeaderSize) + uint64(chunkSize)
		if chunkSize%2 != 0 {
			nextOffset++
		}

		if nextOffset > uint64(len(wavBytes)) && !dataChunkFound {
			// dataチャンクが見つかる前に末尾を超えてしまう場合
			break
		}
		offset = int(nextOffset)
	}

	if !fmtChunkFound || !dataChunkFound {
		missingChunk := ""
		if !fmtChunkFound {
			missingChunk += "'fmt '"
		}
		if !dataChunkFound {
			if missingChunk != "" {
				missingChunk += " and "
			}
			missingChunk += "'data'"
		}
		return nil, nil, &ErrInvalidWAVHeader{
			Index:   index,
			Details: fmt.Sprintf("WAVファイル内に必要なチャンク (%s) が見つかりませんでした", missingChunk),
		}
	}

	formatHeader = wavBytes[0:dataChunkStart]

	// 抽出されたデータサイズがヘッダーの記載と一致するか最終確認
	headerDataSize := binary.LittleEndian.Uint32(wavBytes[dataChunkStart+dataChunkIDSize : dataChunkStart+dataChunkHeaderSize])
	if len(audioData) != int(headerDataSize) {
		return nil, nil, &ErrInvalidWAVHeader{
			Index:   index,
			Details: "最終的な抽出データサイズがヘッダー記載サイズと一致しません",
		}
	}

	return formatHeader, audioData, nil
}

// buildCombinedWav はフォーマットヘッダー情報と結合されたオーディオデータから WAV ファイルを再構築します。
func buildCombinedWav(formatHeader, combinedAudioData []byte, totalAudioSize int) ([]byte, error) {
	dataChunkStart := len(formatHeader)
	dataChunkSizeOffset := dataChunkStart + dataChunkIDSize
	finalWavHeaderSize := dataChunkStart + dataChunkHeaderSize

	// RIFFチャンクサイズ = (総サイズ) - 8
	fileSize := totalAudioSize + finalWavHeaderSize - (riffChunkIDSize + riffChunkSizeSize)

	if uint64(fileSize) > math.MaxUint32 {
		return nil, fmt.Errorf("結合後のWAVファイルサイズが4GBを超過しています")
	}

	combinedWav := make([]byte, finalWavHeaderSize+totalAudioSize)
	copy(combinedWav, formatHeader)

	// dataチャンクヘッダーの書き込み
	copy(combinedWav[dataChunkStart:], []byte("data"))

	// RIFFサイズとdataサイズの更新
	binary.LittleEndian.PutUint32(combinedWav[riffChunkSizeOffset:riffChunkSizeOffset+4], uint32(fileSize))
	binary.LittleEndian.PutUint32(combinedWav[dataChunkSizeOffset:dataChunkSizeOffset+4], uint32(totalAudioSize))

	// 音声データ本体のコピー
	copy(combinedWav[finalWavHeaderSize:], combinedAudioData)

	return combinedWav, nil
}
