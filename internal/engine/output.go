package engine

import (
	"fmt"

	"github.com/shouni/audio/wav"
)

// combineOutput は各セグメントの合成結果を結合し、WAVバイト列を返します。
// 書き込み先への保存は呼び出し側の責務です。
func combineOutput(orderedAudioDataList [][]byte, preCalcErrors []string, runtimeErrors []string) ([]byte, error) {
	allErrors := append([]string{}, preCalcErrors...)
	allErrors = append(allErrors, runtimeErrors...)
	if len(allErrors) > 0 {
		return nil, &ErrSynthesisBatch{
			TotalErrors: len(allErrors),
			Details:     allErrors,
		}
	}

	finalAudioDataList := nonNilAudioData(orderedAudioDataList)
	if len(finalAudioDataList) == 0 {
		return nil, fmt.Errorf("有効な合成データが生成されませんでした")
	}

	combinedWavBytes, err := wav.CombineWavData(finalAudioDataList)
	if err != nil {
		return nil, fmt.Errorf("WAVデータの結合に失敗しました: %w", err)
	}

	return combinedWavBytes, nil
}

func nonNilAudioData(audioDataList [][]byte) [][]byte {
	filtered := make([][]byte, 0, len(audioDataList))
	for _, data := range audioDataList {
		if data != nil {
			filtered = append(filtered, data)
		}
	}
	return filtered
}
