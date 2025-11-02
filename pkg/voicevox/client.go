package voicevox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/shouni/go-http-kit/pkg/httpkit"
)

// ----------------------------------------------------------------------
// クライアント構造体とコンストラクタ
// ----------------------------------------------------------------------

// Client はVOICEVOXエンジンへのAPIリクエストを処理するクライアントです。
// httpkit.Client を利用してリトライ機能を内包します。
type Client struct {
	client *httpkit.Client // リトライ機能付きHTTPクライアント
	apiURL string
}

// NewClient は新しいClientインスタンスを初期化します。
// httpkit.New() を利用して内部クライアントを設定します。
func NewClient(apiURL string, timeout time.Duration) *Client {
	// httpkit.New() はリトライ設定込みのクライアントを初期化
	return &Client{
		client: httpkit.New(timeout),
		apiURL: apiURL,
	}
}

// ----------------------------------------------------------------------
// ヘルパー: API URLの構築
// ----------------------------------------------------------------------

// buildURL はベースURLとエンドポイントを結合し、エラー処理を行います。
func (c *Client) buildURL(endpoint string) (*url.URL, error) {
	u, err := url.Parse(c.apiURL)
	if err != nil {
		// API URL自体のパースエラーを ErrAPINetwork でラップ
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: fmt.Errorf("API URLのパース失敗: %w", err)}
	}

	// url.JoinPath は Go 1.19 以降で利用可能
	u.Path, err = url.JoinPath(u.Path, endpoint)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: fmt.Errorf("エンドポイント結合失敗: %w", err)}
	}

	return u, nil
}

// ----------------------------------------------------------------------
// API呼び出しロジック
// ----------------------------------------------------------------------

// runAudioQuery は /audio_query APIを呼び出し、音声合成のためのクエリJSONを返します。
func (c *Client) runAudioQuery(text string, styleID int, ctx context.Context) ([]byte, error) {
	const endpoint = "/audio_query"

	// 1. URLとクエリパラメータの構築
	u, err := c.buildURL(endpoint)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("text", text)
	q.Set("speaker", fmt.Sprintf("%d", styleID))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), nil)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: fmt.Errorf("リクエスト構築失敗: %w", err)}
	}

	// 3. リクエスト実行 (httpkit.Client.DoRequest() がリトライ、ステータスチェック、ボディ読み取りを処理)
	bodyBytes, err := c.client.DoRequest(req)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: err}
	}

	// 4. JSON構造の検証
	var aqr AudioQueryResponse
	if err := json.Unmarshal(bodyBytes, &aqr); err != nil {
		return nil, &ErrInvalidJSON{Details: fmt.Sprintf("%s応答JSONのデコード", endpoint), WrappedErr: err}
	}

	return bodyBytes, nil
}

// runSynthesis は /synthesis APIを呼び出し、WAV形式の音声データを返します。
func (c *Client) runSynthesis(queryBody []byte, styleID int, ctx context.Context) ([]byte, error) {
	const endpoint = "/synthesis"

	// 1. URLとクエリパラメータの構築
	u, err := c.buildURL(endpoint)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("speaker", fmt.Sprintf("%d", styleID))
	u.RawQuery = q.Encode()

	wavData, err := c.client.PostRawBodyAndFetchBytes(
		u.String(),
		queryBody,
		"application/json",
		ctx,
	)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: err}
	}

	// 3. データ検証
	if len(wavData) < WavTotalHeaderSize {
		return nil, &ErrInvalidWAVHeader{
			Index:   -1,
			Details: fmt.Sprintf("WAVデータのサイズが短すぎます (%dバイト)", len(wavData)),
		}
	}

	return wavData, nil
}

// GetSpeakers は /speakers APIを呼び出し、VOICEVOXエンジンが提供する
// 全てのスピーカー情報（JSONバイトスライス）を返します。
func (c *Client) GetSpeakers(ctx context.Context) ([]byte, error) {
	const endpoint = "/speakers"

	// 1. URLの構築
	u, err := c.buildURL(endpoint)
	if err != nil {
		return nil, err
	}
	speakersURL := u.String()

	// 2. httpkit.FetchBytes を使用してリクエストを簡潔化
	bodyBytes, err := c.client.FetchBytes(speakersURL, ctx)
	if err != nil {
		// httpkit.Client.FetchBytes からのエラーを ErrAPINetwork または ErrAPIResponse としてラップ
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: err}
	}

	return bodyBytes, nil
}
