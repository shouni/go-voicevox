package engine

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"

	"github.com/shouni/go-voicevox/api"
)

// finalizeOutput は結果を結合して書き出します。
func (e *Engine) finalizeOutput(ctx context.Context, orderedAudioDataList [][]byte, outputWavFile string, preCalcErrors []string, runtimeErrors []string) error {
	allErrors := append([]string{}, preCalcErrors...)
	allErrors = append(allErrors, runtimeErrors...)
	if len(allErrors) > 0 {
		return &ErrSynthesisBatch{
			TotalErrors: len(allErrors),
			Details:     allErrors,
		}
	}

	finalAudioDataList := nonNilAudioData(orderedAudioDataList)
	if len(finalAudioDataList) == 0 {
		return fmt.Errorf("有効な合成データが生成されませんでした")
	}

	combinedWavBytes, err := api.CombineWavData(finalAudioDataList)
	if err != nil {
		return fmt.Errorf("WAVデータの結合に失敗しました: %w", err)
	}

	slog.InfoContext(ctx, "音声結合完了。出力先への書き込みを開始します。", "output_uri", outputWavFile)
	return e.writer.Write(ctx, outputWavFile, bytes.NewReader(combinedWavBytes), "audio/wav")
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
