package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// minimalWav は、結合可能な最小の WAV バイト列を返します。
func minimalWav() []byte {
	b := make([]byte, 44)
	copy(b[0:], "RIFF")
	copy(b[8:], "WAVEfmt ")
	b[16], b[20], b[22] = 16, 1, 1
	b[24], b[25] = 0x40, 0x1f // 8000Hz
	b[32], b[34] = 2, 16
	copy(b[36:], "data")
	return b
}

// instantClient は待たずに合成を返します。打ち切りを起こすのはレート制限側です。
type instantClient struct{}

func (instantClient) RunAudioQuery(context.Context, string, int) ([]byte, error) {
	return []byte("query"), nil
}

func (instantClient) RunSynthesis(context.Context, []byte, int) ([]byte, error) {
	return minimalWav(), nil
}

// testLines は、同じ話者・スタイルの行を n 件返します。
func testLines(n int) []ScriptLine {
	lines := make([]ScriptLine, n)
	for i := range lines {
		lines[i] = ScriptLine{Speaker: "話者アルファ", Style: "標準", Text: "テストの本文です。"}
	}
	return lines
}

// TestRunReportsSegmentsCancelledBeforeStart は、開始前に打ち切られたセグメントが
// エラーとして報告されることを検証します。
//
// これは実際に踏んだ取りこぼしです。レート制限の待機中に ctx が切れると、
// そのゴルーチンは結果を書かずに抜けます。集計側がそれを「無かったもの」として
// 捨てていたため、10 件中 1 件だけ合成された音声がエラー無しで返っていました。
// ap-voice の PIPELINE_TIMEOUT はまさにこの形で ctx を切るため、
// 途中までの WAV が保存され、完了通知まで飛びます。
func TestRunReportsSegmentsCancelledBeforeStart(t *testing.T) {
	t.Parallel()

	e := New(
		instantClient{},
		stubFinder{styleIDs: map[string]int{"[話者アルファ][標準]": 1}},
		stubConverter{},
		WithMaxParallelSegments(8),
		// 2 件目以降がレート制限で待たされ、合成へ入る前に ctx が切れます。
		WithSegmentRateLimit(10*time.Second),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	got, err := e.Run(ctx, testLines(10))
	if err == nil {
		t.Fatalf("打ち切られたのにエラーになりません（%d バイトを返しました）", len(got))
	}
	if got != nil {
		t.Errorf("エラー時に音声を返しています: %d バイト", len(got))
	}

	var batch *ErrSynthesisBatch
	if !errors.As(err, &batch) {
		t.Fatalf("ErrSynthesisBatch ではありません: %v", err)
	}
	// 通ったのは先頭 1 件だけなので、残り 9 件が報告されるはずです。
	if len(batch.Errors) != 9 {
		t.Errorf("報告されたエラーは %d 件です。取りこぼしがあります（want 9）", len(batch.Errors))
	}
	if !strings.Contains(err.Error(), "開始前に中断されました") {
		t.Errorf("中断の理由が読み取れません: %v", err)
	}
}

// TestCollectSynthesisResultsKeepsSegmentPositions は、合成しなかった位置が
// nil のまま残ることを検証します。
//
// 詰めてしまうと、結合時のエラーが指す位置を元のセグメント番号へ戻せません
// （output.go の nonNilAudioData / withSegmentIndex がその対応表を作ります）。
func TestCollectSynthesisResultsKeepsSegmentPositions(t *testing.T) {
	t.Parallel()

	segments := []segment{
		{Text: "あ"},
		{Text: ""}, // 空テキストは投げません
		{Text: "う"},
	}
	results := []*segmentResult{
		{wavData: []byte("wav-0")},
		nil,
		{wavData: []byte("wav-2")},
	}

	audio, errs := collectSynthesisResults(segments, results)
	if len(errs) != 0 {
		t.Fatalf("空テキストがエラーとして数えられています: %v", errs)
	}
	if len(audio) != len(segments) {
		t.Fatalf("長さがセグメント数と一致しません: %d", len(audio))
	}
	if audio[1] != nil {
		t.Error("空テキストの位置が詰められています")
	}
	if string(audio[0]) != "wav-0" || string(audio[2]) != "wav-2" {
		t.Errorf("位置がずれています: %q", audio)
	}
}

// TestCollectSynthesisResultsReportsMissingResult は、投げるはずのセグメントに
// 結果が無い場合をエラーとして数えることを検証します。集計側の最後の砦です。
func TestCollectSynthesisResultsReportsMissingResult(t *testing.T) {
	t.Parallel()

	segments := []segment{{Text: "あ"}}

	_, errs := collectSynthesisResults(segments, []*segmentResult{nil})
	if len(errs) != 1 {
		t.Fatalf("エラーが %d 件です（want 1）", len(errs))
	}
	if !strings.Contains(errs[0].Error(), "記録されませんでした") {
		t.Errorf("理由が読み取れません: %v", errs[0])
	}
}

// errClient は、合成で必ず指定のエラーを返すクライアントです。
type errClient struct{ err error }

func (c errClient) RunAudioQuery(context.Context, string, int) ([]byte, error) {
	return nil, fmt.Errorf("audio_query の呼び出しに失敗: %w", c.err)
}

func (c errClient) RunSynthesis(context.Context, []byte, int) ([]byte, error) {
	return nil, c.err
}

// TestErrSynthesisBatchKeepsErrorTypes は、まとめたエラーが型と原因を失わないことを検証します。
//
// 以前は []string に潰していました。そのため呼び出し側は、打ち切られたのか
// エンジンが落ちているのかを、メッセージの文字列照合でしか区別できませんでした。
// Unwrap() []error があると、errors.Is / errors.As がバッチ越しに届きます。
func TestErrSynthesisBatchKeepsErrorTypes(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("エンジンに接続できません")

	e := New(
		errClient{err: sentinel},
		stubFinder{styleIDs: map[string]int{"[話者アルファ][標準]": 1}},
		stubConverter{},
		WithMaxParallelSegments(4),
		WithSegmentRateLimit(time.Millisecond),
	)

	_, err := e.Run(context.Background(), testLines(3))
	if err == nil {
		t.Fatal("全件失敗したのにエラーになりません")
	}

	var batch *ErrSynthesisBatch
	if !errors.As(err, &batch) {
		t.Fatalf("ErrSynthesisBatch ではありません: %v", err)
	}
	if len(batch.Errors) != 3 {
		t.Errorf("エラー件数 = %d, want 3", len(batch.Errors))
	}
	// ここが本題です。バッチとセグメントの 2 段を越えて原因まで辿れること。
	if !errors.Is(err, sentinel) {
		t.Errorf("errors.Is(err, sentinel) = false: %v", err)
	}
}

// TestRunSurfacesDeadlineFromInFlightSegments は、合成の最中に打ち切られた
// セグメントからも打ち切りが判別できることを検証します。
//
// 待機中に落ちた分は ctx.Err() をそのまま包むので元から辿れましたが、
// 通信中に落ちた分は api.ErrAPINetwork に包まれます。その型が Unwrap を
// 持たないと、セグメント数が並列数以下のとき（全件が通信中）に
// 打ち切りだと分からなくなります。
func TestRunSurfacesDeadlineFromInFlightSegments(t *testing.T) {
	t.Parallel()

	e := New(
		errClient{err: context.DeadlineExceeded},
		stubFinder{styleIDs: map[string]int{"[話者アルファ][標準]": 1}},
		stubConverter{},
		// 並列数がセグメント数以上なので、待機で落ちるものはありません。
		WithMaxParallelSegments(8),
		WithSegmentRateLimit(time.Millisecond),
	)

	_, err := e.Run(context.Background(), testLines(3))
	if err == nil {
		t.Fatal("全件失敗したのにエラーになりません")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(err, context.DeadlineExceeded) = false: %v", err)
	}
}
