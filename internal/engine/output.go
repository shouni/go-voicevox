package engine

import (
	"errors"
	"fmt"

	"github.com/shouni/audio/wav"
)

// errNoAudioData は、結合できる音声が 1 つも無いことを示します。
//
// 埋め込む値が無いので fmt.Errorf ではなく変数として持ちます。errNoSegments と同じく、
// 呼び出し側が errors.Is で識別できる形にしておきます。
var errNoAudioData = errors.New("有効な合成データが生成されませんでした")

// combineOutput は各セグメントの合成結果を結合し、WAVバイト列を返します。
// 書き込み先への保存は呼び出し側の責務です。
func combineOutput(orderedAudioDataList [][]byte, preCalcErrors []error, runtimeErrors []error) ([]byte, error) {
	if err := newErrSynthesisBatch(preCalcErrors, runtimeErrors); err != nil {
		return nil, err
	}

	finalAudioDataList, segmentIndexes := nonNilAudioData(orderedAudioDataList)
	if len(finalAudioDataList) == 0 {
		return nil, errNoAudioData
	}

	combinedWavBytes, err := wav.CombineWavData(finalAudioDataList)
	if err != nil {
		return nil, fmt.Errorf("WAVデータの結合に失敗しました: %w", withSegmentIndex(err, segmentIndexes))
	}

	return combinedWavBytes, nil
}

// nonNilAudioData は合成済みのデータだけを取り出し、あわせて各データが
// 元の何番目のセグメントだったかを返します。
//
// 空テキストのセグメントは合成されず nil のまま残るため、詰めた後の位置は
// セグメント番号と一致しません。エラー報告で元の番号へ戻すために対応表が要ります。
func nonNilAudioData(audioDataList [][]byte) (data [][]byte, segmentIndexes []int) {
	data = make([][]byte, 0, len(audioDataList))
	segmentIndexes = make([]int, 0, len(audioDataList))

	for i, d := range audioDataList {
		if d == nil {
			continue
		}
		data = append(data, d)
		segmentIndexes = append(segmentIndexes, i)
	}

	return data, segmentIndexes
}

// withSegmentIndex は、結合時のエラーが指す位置を元のセグメント番号へ言い換えます。
//
// wav パッケージは詰めた後のリスト内の位置しか知らないため、そのまま提示すると
// 空テキストのセグメントがある場合に実際とずれた番号を報告してしまいます。
// 元のエラーは包んだままにして、errors.As での判別を保ちます。
func withSegmentIndex(err error, segmentIndexes []int) error {
	segmentOf := func(i int) int {
		if i < 0 || i >= len(segmentIndexes) {
			return i
		}
		return segmentIndexes[i]
	}

	if mismatch, ok := errors.AsType[*wav.ErrMismatchedWAVFormat](err); ok {
		return fmt.Errorf("セグメント %d の音声形式が先頭のセグメントと揃っていません: %w",
			segmentOf(mismatch.Index), err)
	}

	if header, ok := errors.AsType[*wav.ErrInvalidWAVHeader](err); ok {
		return fmt.Errorf("セグメント %d の音声データが不正です: %w", segmentOf(header.Index), err)
	}

	return err
}
