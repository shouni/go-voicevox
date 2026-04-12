package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/shouni/go-http-kit/httpkit"
)

// Client は、VOICEVOX エンジンなどの API リクエストを処理するクライアントです。
// httpkit.Requester を介して、リトライやエラーハンドリングを伴う通信を行います。
type Client struct {
	client httpkit.Requester
	apiURL string
}

// NewClient は、指定された HTTP リクエスト実行器と API ベース URL を使用して、
// 新しい Client インスタンスを初期化して返します。
func NewClient(client httpkit.Requester, apiURL string) *Client {
	return &Client{
		client: client,
		apiURL: apiURL,
	}
}

// buildURL はベース URL とエンドポイントを結合し、解析可能な URL 構造体を返します。
// 内部的なヘルパー関数として機能します。
func (c *Client) buildURL(endpoint string) (*url.URL, error) {
	u, err := url.Parse(c.apiURL)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: fmt.Errorf("API URLのパース失敗: %w", err)}
	}

	u.Path, err = url.JoinPath(u.Path, endpoint)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: fmt.Errorf("エンドポイント結合失敗: %w", err)}
	}

	return u, nil
}

// RunAudioQuery は /audio_query API を呼び出し、音声合成に必要なクエリ JSON を取得します。
// 引数 text には合成したい文字列、styleID には話者のスタイル ID を指定します。
func (c *Client) RunAudioQuery(ctx context.Context, text string, styleID int) ([]byte, error) {
	const endpoint = "/audio_query"

	u, err := c.buildURL(endpoint)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("text", text)
	q.Set("speaker", fmt.Sprintf("%d", styleID))
	u.RawQuery = q.Encode()
	finalURL := u.String()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, finalURL, nil)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: fmt.Errorf("リクエスト構築失敗: %w", err)}
	}

	bodyBytes, err := c.client.DoRequest(req)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: err}
	}

	var aqr AudioQueryResponse
	if err := json.Unmarshal(bodyBytes, &aqr); err != nil {
		return nil, &ErrInvalidJSON{Details: fmt.Sprintf("%s応答JSONのデコード", endpoint), WrappedErr: err}
	}

	return bodyBytes, nil
}

// RunSynthesis は /synthesis API を呼び出し、WAV 形式の音声データを取得します。
// queryBody には RunAudioQuery で取得したクエリ JSON バイト列を指定し、
// styleID には話者のスタイル ID を指定します。
func (c *Client) RunSynthesis(ctx context.Context, queryBody []byte, styleID int) ([]byte, error) {
	const endpoint = "/synthesis"

	u, err := c.buildURL(endpoint)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("speaker", fmt.Sprintf("%d", styleID))
	u.RawQuery = q.Encode()
	finalURL := u.String()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, finalURL, bytes.NewReader(queryBody))
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: fmt.Errorf("リクエスト構築失敗: %w", err)}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/wav")

	wavData, err := c.client.DoRequest(req)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: err}
	}

	if len(wavData) < wavTotalHeaderSize {
		return nil, &ErrInvalidWAVHeader{
			Index:   -1,
			Details: fmt.Sprintf("WAVデータのサイズが短すぎます (%dバイト)", len(wavData)),
		}
	}

	return wavData, nil
}

// GetSpeakers は /speakers API を呼び出し、VOICEVOX エンジンが提供する
// すべての話者情報を JSON バイト列として取得します。
func (c *Client) GetSpeakers(ctx context.Context) ([]byte, error) {
	const endpoint = "/speakers"

	u, err := c.buildURL(endpoint)
	if err != nil {
		return nil, err
	}
	speakersURL := u.String()

	bodyBytes, err := c.client.FetchBytes(ctx, speakersURL)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: err}
	}

	return bodyBytes, nil
}
