package voicevox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// ----------------------------------------------------------------------
// ロードロジック
// ----------------------------------------------------------------------

// LoadSpeakers は /speakers エンドポイントからデータを取得し、SpeakerDataを構築します。
func LoadSpeakers(ctx context.Context, client SpeakerClient) (*SpeakerData, error) {
	// 1. 静的なSupportedSpeakersから、内部使用のためのマップを構築
	apiNameToToolTag := make(map[string]string)
	for _, mapping := range SupportedSpeakers {
		apiNameToToolTag[mapping.APIName] = mapping.ToolTag
	}

	// 2. API呼び出し: 戻り値が []byte と error の二つであるため、両方を受け取る
	bodyBytes, err := client.GetSpeakers(ctx)
	if err != nil {
		// GetSpeakers 内でErrAPINetworkなどがラップされているため、そのまま返す
		return nil, err
	}

	// 3. JSONデコード
	var vvSpeakers []VVSpeaker
	if err := json.Unmarshal(bodyBytes, &vvSpeakers); err != nil {
		// ErrInvalidJSON を利用 (error.go で定義を想定)
		return nil, &ErrInvalidJSON{Details: "/speakers 応答", WrappedErr: err}
	}

	// 4. データ構造の構築
	data := &SpeakerData{
		StyleIDMap:      make(map[string]int),
		DefaultStyleMap: make(map[string]string),
	}

	// 応答データから StyleIDMap と DefaultStyleMap を構築
	for _, spk := range vvSpeakers {
		toolTag, tagFound := apiNameToToolTag[spk.Name]
		if !tagFound {
			continue // サポート対象外の話者はスキップ
		}

		for _, style := range spk.Styles {
			styleTag, tagExists := StyleApiNameToToolTag[style.Name]
			if !tagExists {
				slog.Debug("サポートされていないスタイルをスキップします", "speaker", spk.Name, "style", style.Name)
				continue
			}

			combinedTag := toolTag + styleTag // 例: "[めたん][ノーマル]"
			data.StyleIDMap[combinedTag] = style.ID

			// VvTagNormal ([ノーマル]) スタイルをデフォルトとして登録
			if styleTag == VvTagNormal {
				data.DefaultStyleMap[toolTag] = combinedTag
			}
		}
	}

	// 5. 必須のデフォルトスタイルが存在するかチェック
	missingDefaults := []string{}
	for _, mapping := range SupportedSpeakers {
		toolTag := mapping.ToolTag
		if _, ok := data.DefaultStyleMap[toolTag]; !ok {
			slog.Error("必須話者のデフォルトスタイルが見つかりません", "speaker", toolTag, "required_style", VvTagNormal)
			missingDefaults = append(missingDefaults, mapping.APIName)
		}
	}

	if len(missingDefaults) > 0 {
		// ErrMissingRequiredField を利用 (error.go で定義を想定)
		return nil, &ErrMissingRequiredField{
			Field:   fmt.Sprintf("デフォルトスタイル（%s）", VvTagNormal),
			Context: fmt.Sprintf("必須話者: %s", strings.Join(missingDefaults, ", ")),
		}
	}

	slog.InfoContext(ctx, "VOICEVOXスタイルデータが正常にロードされました", "styles_count", len(data.StyleIDMap))

	return data, nil
}
