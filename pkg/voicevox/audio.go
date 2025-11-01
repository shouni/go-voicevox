package voicevox

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

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
	// 修正された extractAudioData が、fmt/data チャンクを動的に探索し、メタデータをスキップ
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
// 内部ヘルパー関数 (最終修正版: fmt/data チャンクの両方を動的探索)
// ----------------------------------------------------------------------

// extractAudioData はWAVファイルバイトスライスからフォーマットヘッダー情報とオーディオデータ部分を抽出します。
// fmt/data チャンクを動的に探索し、dataチャンクの直前までを formatHeader とします。
func extractAudioData(wavBytes []byte, index int) (formatHeader []byte, audioData []byte, err error) {

	// RIFFヘッダー (12バイト: RIFF + file size + WAVE) の存在確認
	if len(wavBytes) < WavRiffHeaderSize {
		return nil, nil, &ErrInvalidWAVHeader{
			Index:   index,
			Details: fmt.Sprintf("WAVファイルサイズが短すぎます (RIFFヘッダー不足: %dバイト)", len(wavBytes)),
		}
	}

	var fmtChunkFound, dataChunkFound bool
	var dataChunkStart int

	offset := WavRiffHeaderSize // RIFFヘッダーの直後 (12バイト目) からチャンク探索を開始

	// ファイル終端まで、または data チャンクが見つかるまでループ
	for offset < len(wavBytes) {
		// チャンクヘッダー (チャンクID 4 + サイズ 4 = 8バイト) の読み込みチェック
		if offset+DataChunkHeaderSize > len(wavBytes) {
			break // チャンクヘッダーを読み込むのに十分なバイトがない
		}

		// チャンクID (4バイト) を抽出
		chunkID := string(wavBytes[offset : offset+DataChunkIDSize])

		// チャンクサイズ (次の4バイト) を抽出 (リトルエンディアン)
		chunkSize := binary.LittleEndian.Uint32(wavBytes[offset+DataChunkIDSize : offset+DataChunkHeaderSize])

		// fmt チャンクの確認 (formatHeaderの整合性を確認するため)
		if chunkID == "fmt " {
			fmtChunkFound = true
		}

		// data チャンクの確認
		if chunkID == "data" {
			dataChunkFound = true
			dataChunkStart = offset // dataチャンクの開始位置を記録

			// data チャンクヘッダー（8バイト）の直後からオーディオデータが開始
			audioDataStart := offset + DataChunkHeaderSize
			audioDataEnd := audioDataStart + int(chunkSize)

			if audioDataEnd > len(wavBytes) {
				return nil, nil, &ErrInvalidWAVHeader{
					Index:   index,
					Details: "dataチャンクのデータ長がファイルサイズを超過しています",
				}
			}

			// オーディオデータ部分を抽出
			audioData = wavBytes[audioDataStart:audioDataEnd]
			break // data チャンクが見つかったためループ終了
		}

		// data チャンクでない場合 (LIST, fact, JUNK など) はスキップ
		// 次のチャンクヘッダーの開始位置までオフセットを移動
		offset += DataChunkHeaderSize + int(chunkSize)

		// パディングバイトの考慮 (奇数長のチャンクデータの後)
		if chunkSize%2 != 0 {
			offset += 1
		}
	}

	// 必要なチャンクが見つかったか最終チェック
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

	// formatHeader は RIFFヘッダーから data チャンクの直前まで
	// これにより、fmtチャンクの前後の任意のメタデータ(JUNK/LIST)を含めることができる
	formatHeader = wavBytes[0:dataChunkStart]

	// 抽出されたデータサイズがヘッダーの記載と一致するか最終確認
	// このチェックはすでに data チャンクが見つかったブロック内で行われているが、冗長性を排除するため最終結果をチェック
	if len(audioData) != int(binary.LittleEndian.Uint32(wavBytes[dataChunkStart+DataChunkIDSize:dataChunkStart+DataChunkHeaderSize])) {
		return nil, nil, &ErrInvalidWAVHeader{
			Index:   index,
			Details: "最終的な抽出データサイズがヘッダー記載サイズと一致しません",
		}
	}

	return formatHeader, audioData, nil
}

// buildCombinedWav はフォーマットヘッダー情報と結合されたオーディオデータから、
// 正しいヘッダーを持つ単一のWAVファイルを構築します。
// 修正: formatHeader のサイズが固定でないため、dataChunkSizeOffset の算出ロジックを変更
func buildCombinedWav(formatHeader, combinedAudioData []byte, totalAudioSize int) ([]byte, error) {
	// formatHeader の長さは now dataChunkStart と同等である
	dataChunkStart := len(formatHeader)

	// dataチャンクのサイズが書き込まれるオフセットは formatHeader の終端 + dataチャンクIDのサイズ
	dataChunkSizeOffset := dataChunkStart + DataChunkIDSize

	// 最終的なWAVファイルの総ヘッダーサイズは dataChunkStart + DataChunkHeaderSize
	finalWavHeaderSize := dataChunkStart + DataChunkHeaderSize

	// 最終的なWAVファイルの総サイズ
	// RIFFチャンクサイズは (totalAudioSize + finalWavHeaderSize) - 8 (RIFFID + Size + WAVEID)
	fileSize := totalAudioSize + finalWavHeaderSize - (RiffChunkIDSize + WaveIDSize)

	// 新しいヘッダーをコピー
	combinedWav := make([]byte, finalWavHeaderSize+totalAudioSize)
	copy(combinedWav, formatHeader)

	// dataチャンクヘッダー（"data" + size）を追加
	copy(combinedWav[dataChunkStart:], []byte("data"))

	// RIFFチャンクサイズ (File Size - 8) の更新 (4-8バイト目)
	binary.LittleEndian.PutUint32(combinedWav[RiffChunkSizeOffset:RiffChunkSizeOffset+4], uint32(fileSize))

	// dataチャンクサイズ (Audio Data Size) の更新 (dataChunkSizeOffsetの位置)
	binary.LittleEndian.PutUint32(combinedWav[dataChunkSizeOffset:dataChunkSizeOffset+4], uint32(totalAudioSize))

	// オーディオデータ本体をコピー
	copy(combinedWav[finalWavHeaderSize:], combinedAudioData)

	return combinedWav, nil
}
