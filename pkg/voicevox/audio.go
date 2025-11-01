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
		// メタデータチャンク（LISTなど）をスキップし、純粋なオーディオデータのみを抽出
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
// 内部ヘルパー関数 (修正済み: チャンク探索ロジックを導入)
// ----------------------------------------------------------------------

// extractAudioData はWAVファイルバイトスライスからフォーマットヘッダー情報とオーディオデータ部分を抽出します。
// LISTチャンクなどのメタデータをスキップし、dataチャンクを動的に探します。
func extractAudioData(wavBytes []byte, index int) (formatHeader []byte, audioData []byte, err error) {
	// RIFFヘッダー (12バイト: RIFF + file size + WAVE) の存在確認
	if len(wavBytes) < WavRiffHeaderSize {
		return nil, nil, &ErrInvalidWAVHeader{
			Index:   index,
			Details: fmt.Sprintf("WAVファイルサイズが短すぎます (RIFFヘッダー不足: %dバイト)", len(wavBytes)),
		}
	}

	// フォーマットヘッダーを抽出 (0から36バイト目まで: RIFF + fmt)
	// WavFmtChunkSize (24) + WavRiffHeaderSize (12) = DataChunkOffset (36)
	formatHeader = wavBytes[0:DataChunkOffset]

	// data チャンクの動的な探索は fmt チャンクの直後 (36バイト目) から開始
	offset := DataChunkOffset
	dataChunkFound := false

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

		// data チャンクの確認
		if chunkID == "data" {
			dataChunkFound = true

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

			// 抽出されたデータサイズがヘッダーの記載と一致するか最終確認
			if len(audioData) != int(chunkSize) {
				return nil, nil, &ErrInvalidWAVHeader{
					Index:   index,
					Details: fmt.Sprintf("抽出データサイズ (%d) がdataチャンクサイズ (%d) と一致しません", len(audioData), chunkSize),
				}
			}

			break // data チャンクが見つかったためループ終了
		}

		// data チャンクでない場合 (LIST, fact など) はスキップ
		// 次のチャンクヘッダーの開始位置までオフセットを移動
		offset += DataChunkHeaderSize + int(chunkSize)

		// パディングバイトの考慮 (奇数長のチャンクデータの後)
		if chunkSize%2 != 0 {
			offset += 1
		}
	}

	if !dataChunkFound {
		return nil, nil, &ErrInvalidWAVHeader{
			Index:   index,
			Details: "WAVファイル内に 'data' チャンクが見つかりませんでした",
		}
	}

	return formatHeader, audioData, nil
}

// buildCombinedWav はフォーマットヘッダー情報と結合されたオーディオデータから、
// 正しいヘッダーを持つ単一のWAVファイルを構築します。
func buildCombinedWav(formatHeader, combinedAudioData []byte, totalAudioSize int) ([]byte, error) {
	// 最終的なWAVファイルの総サイズ
	// RIFFチャンクサイズは (totalAudioSize + WavTotalHeaderSize) - 8
	fileSize := totalAudioSize + WavTotalHeaderSize - (RiffChunkIDSize + WaveIDSize)

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
