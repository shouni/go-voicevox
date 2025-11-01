package voicevox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
// API呼び出しロジック
// ----------------------------------------------------------------------

// runAudioQuery は /audio_query APIを呼び出し、音声合成のためのクエリJSONを返します。
func (c *Client) runAudioQuery(text string, styleID int, ctx context.Context) ([]byte, error) {
	endpoint := "/audio_query"

	// 1. URLとクエリパラメータの構築
	urlStr := fmt.Sprintf("%s%s?text=%s&speaker=%d", c.apiURL, endpoint, url.QueryEscape(text), styleID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, nil)
	if err != nil {
		// ErrAPINetwork を利用
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: err}
	}

	// 2. リクエスト実行 (httpkit.Client.Do() がリトライを処理)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: err}
	}
	defer resp.Body.Close()

	// 3. 応答コードのチェック
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: fmt.Errorf("応答ボディの読み取り失敗: %w", err)}
	}

	if resp.StatusCode != http.StatusOK {
		// ErrAPIResponse を利用
		return nil, &ErrAPIResponse{
			Endpoint:   endpoint,
			StatusCode: resp.StatusCode,
			Body:       string(bodyBytes),
		}
	}

	// 4. JSON構造の検証 (model.AudioQueryResponse の型を利用)
	var aqr AudioQueryResponse
	if err := json.Unmarshal(bodyBytes, &aqr); err != nil {
		// ErrInvalidJSON を利用
		return nil, &ErrInvalidJSON{Details: fmt.Sprintf("%s応答JSONのデコード", endpoint), WrappedErr: err}
	}

	// 成功: VOICEVOXエンジンからのクエリJSONバイトをそのまま返す
	return bodyBytes, nil
}

// runSynthesis は /synthesis APIを呼び出し、WAV形式の音声データを返します。
func (c *Client) runSynthesis(queryBody []byte, styleID int, ctx context.Context) ([]byte, error) {
	endpoint := "/synthesis"

	// 1. URLとクエリパラメータの構築
	urlStr := fmt.Sprintf("%s%s?speaker=%d", c.apiURL, endpoint, styleID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, bytes.NewReader(queryBody))
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: err}
	}
	req.Header.Set("Content-Type", "application/json")

	// 2. リクエスト実行
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: err}
	}
	defer resp.Body.Close()

	// 3. 応答コードのチェック
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: fmt.Errorf("応答ボディの読み取り失敗: %w", err)}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &ErrAPIResponse{
			Endpoint:   endpoint,
			StatusCode: resp.StatusCode,
			Body:       string(bodyBytes),
		}
	}

	// 4. 成功: WAV形式のバイトデータを返す
	return bodyBytes, nil
}

// ----------------------------------------------------------------------
// 汎用GETリクエスト (model.SpeakerClient インターフェースの実装)
// ----------------------------------------------------------------------

// Get は汎用のGETリクエストを実行し、応答ボディのバイトスライスを返します。
// model.SpeakerClient インターフェースを満たします。
func (c *Client) Get(urlStr string, ctx context.Context) ([]byte, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		// ErrAPINetwork を利用
		return nil, &ErrAPINetwork{Endpoint: urlStr, WrappedErr: err}
	}

	// リクエスト実行 (httpkit.Client.Do() がリトライを処理)
	resp, err := c.client.Do(req)
	if err != nil {
		// ErrAPINetwork を利用
		return nil, &ErrAPINetwork{Endpoint: urlStr, WrappedErr: err}
	}
	defer resp.Body.Close()

	// 応答コードのチェックとボディ読み込み
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: urlStr, WrappedErr: fmt.Errorf("応答ボディの読み取り失敗: %w", err)}
	}

	if resp.StatusCode != http.StatusOK {
		// ErrAPIResponse を利用
		return nil, &ErrAPIResponse{
			Endpoint:   urlStr,
			StatusCode: resp.StatusCode,
			Body:       string(bodyBytes),
		}
	}

	return bodyBytes, nil
}
