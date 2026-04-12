package speaker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/shouni/go-voicevox/api"
	"github.com/shouni/go-voicevox/ports"
)

// LoadSpeakers は /speakers エンドポイントからデータを取得し、SpeakerData を構築します。
//
// この関数は、API から取得した全話者データの中から SupportedSpeakers に定義された
// 話者のみを抽出し、ツール内で利用可能な StyleIDMap と DefaultStyleMap を生成します。
// 必須話者のデフォルトスタイル（VvTagNormal）が見つからない場合はエラーを返します。
func LoadSpeakers(ctx context.Context, client ports.APIClient) (*SpeakerData, error) {
	// 1. 静的なSupportedSpeakersから、内部使用のためのマップを構築
	apiNameToToolTag := make(map[string]string)
	for _, mapping := range SupportedSpeakers {
		apiNameToToolTag[mapping.APIName] = mapping.ToolTag
	}

	// 2. API呼び出し
	bodyBytes, err := client.GetSpeakers(ctx)
	if err != nil {
		return nil, err
	}

	// 3. JSONデコード
	var vvSpeakers []VVSpeaker
	if err := json.Unmarshal(bodyBytes, &vvSpeakers); err != nil {
		return nil, &api.ErrInvalidJSON{Details: "/speakers 応答", WrappedErr: err}
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

			combinedTag := toolTag + styleTag
			data.StyleIDMap[combinedTag] = style.ID

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
		return nil, &ErrMissingRequiredField{
			Field:   fmt.Sprintf("デフォルトスタイル（%s）", VvTagNormal),
			Context: fmt.Sprintf("必須話者: %s", strings.Join(missingDefaults, ", ")),
		}
	}

	slog.InfoContext(ctx, "VOICEVOXスタイルデータが正常にロードされました", "styles_count", len(data.StyleIDMap))

	return data, nil
}
