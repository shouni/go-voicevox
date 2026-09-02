package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// requester は、voicevox.Requester を net/http だけで満たす最小の実装です。
//
// ライブラリが呼ぶのはこの 2 メソッドだけなので、localhost のエンジンを相手にする
// デモはこれで足ります。実運用の呼び出し側は、リトライやエラーハンドリングを備えた
// クライアントをそのまま渡せます。ライブラリはこの口しか知りません。
type requester struct {
	client *http.Client
}

func newRequester(timeout time.Duration) *requester {
	return &requester{client: &http.Client{Timeout: timeout}}
}

// SendBytes は、組み立て済みのリクエストを実行し、応答ボディを返します。
func (r *requester) SendBytes(req *http.Request) ([]byte, error) {
	return r.do(req)
}

// GetBytes は、URL から応答ボディを取得します。
func (r *requester) GetBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("リクエスト構築失敗: %w", err)
	}

	return r.do(req)
}

// do は 2 つの口の共通部分です。ステータスコードの判定はここで行います。
func (r *requester) do(req *http.Request) ([]byte, error) {
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("応答の読み取りに失敗しました: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%s が %s を返しました: %s", req.URL.Path, resp.Status, body)
	}

	return body, nil
}
