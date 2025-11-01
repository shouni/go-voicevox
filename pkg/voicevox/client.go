package voicevox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"net/url" // url.JoinPath を利用
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
	u, err := url.Parse(c.apiURL)
	if err != nil {
		// API URL自体のパースエラーを ErrAPINetwork でラップ
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: fmt.Errorf("API URLのパース失敗: %w", err)}
	}
	u.Path, err = url.JoinPath(u.Path, endpoint)

	// クエリパラメータの構築
	q := u.Query()
	q.Set("text", text)
	q.Set("speaker", fmt.Sprintf("%d", styleID)) // intを文字列に変換
	u.RawQuery = q.Encode()

	// 2. リクエストの作成 (POSTで空のボディを送信)
	// NOTE: VOICEVOXエンジンはPOST時にクエリパラメータでtextを受け取る
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), nil)
	if err != nil {
		// リクエスト構築失敗を ErrAPINetwork でラップ
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: fmt.Errorf("リクエスト構築失敗: %w", err)}
	}

	// 3. リクエスト実行 (httpkit.Client.Do() がリトライを処理)
	resp, err := c.client.Do(req)
	if err != nil {
		// ネットワークエラーまたはリトライ失敗
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: err}
	}
	defer resp.Body.Close()

	// 4. 応答ボディの読み取り
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		// ボディの読み取り失敗
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: fmt.Errorf("応答ボディの読み取り失敗: %w", err)}
	}

	// 5. 応答コードのチェック
	if resp.StatusCode != http.StatusOK {
		// ErrAPIResponse を利用
		return nil, &ErrAPIResponse{
			Endpoint:   endpoint,
			StatusCode: resp.StatusCode,
			Body:       string(bodyBytes),
		}
	}

	// 6. JSON構造の検証 (成功コードが返ってもJSONが不正な場合があるためチェック)
	var aqr AudioQueryResponse // model.go で定義された型
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
	u, err := url.Parse(c.apiURL)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: fmt.Errorf("API URLのパース失敗: %w", err)}
	}
	u.Path, err = url.JoinPath(u.Path, endpoint)

	// クエリパラメータの構築
	q := u.Query()
	q.Set("speaker", fmt.Sprintf("%d", styleID))
	u.RawQuery = q.Encode()

	// 2. リクエストの作成
	// クエリJSON (queryBody) をリクエストボディとして使用します。
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(queryBody))
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: fmt.Errorf("リクエスト構築失敗: %w", err)}
	}

	// リクエストボディがJSON形式であることを明示
	req.Header.Set("Content-Type", "application/json")
	// レスポンスがWAV形式であることを明示
	req.Header.Set("Accept", "audio/wav")

	// 3. リクエスト実行 (httpkit.Client.Do() がリトライを処理)
	resp, err := c.client.Do(req)
	if err != nil {
		// ネットワークエラーまたはリトライ失敗
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: err}
	}
	defer resp.Body.Close()

	// 4. 応答ボディの読み取り
	wavData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ErrAPINetwork{Endpoint: endpoint, WrappedErr: fmt.Errorf("応答ボディ（WAV）の読み取り失敗: %w", err)}
	}

	// 5. 応答コードのチェック
	if resp.StatusCode != http.StatusOK {
		// エラー時、ボディはエラーメッセージを含む可能性が高いため、そのまま表示
		return nil, &ErrAPIResponse{
			Endpoint:   endpoint,
			StatusCode: resp.StatusCode,
			Body:       string(wavData),
		}
	}

	// 6. データ検証 (WAVデータとして十分なサイズがあるか)
	if len(wavData) < WavTotalHeaderSize { // const.go の定数 WavTotalHeaderSize (44) を使用
		return nil, &ErrInvalidWAVHeader{
			Index:   -1, // インデックスは不明
			Details: fmt.Sprintf("WAVデータのサイズが短すぎます (%dバイト)", len(wavData)),
		}
	}

	// 成功: WAVデータのバイトスライスを返す
	return wavData, nil
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
