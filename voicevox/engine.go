package voicevox

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shouni/audio/phonetic"

	"github.com/shouni/go-voicevox/internal/api"
	internalengine "github.com/shouni/go-voicevox/internal/engine"
	"github.com/shouni/go-voicevox/speaker"
)

const defaultVoicevoxAPIURL = "http://localhost:50021"

// New wires up the dependencies and returns an Engine.
//
// Passing a *speaker.Registry as speakers restricts synthesis to the speakers and styles in
// that list. nil accepts everything the engine provides.
func New(
	ctx context.Context,
	httpClient Requester,
	voicevoxAPIURL string,
	speakers *speaker.Registry,
	opts ...Option,
) (Engine, error) {
	if voicevoxAPIURL == "" {
		voicevoxAPIURL = defaultVoicevoxAPIURL
		slog.WarnContext(ctx, "VOICEVOX_API_URL is not set, using the default", "url", voicevoxAPIURL)
	}

	o := newOptions(opts...)

	voicevoxClient := api.New(httpClient, voicevoxAPIURL)

	slog.InfoContext(ctx, "loading VOICEVOX speaker style data")
	// This works with a nil list too (no filter). LoadStyles logs the counts itself on
	// success, so no completion log is added here.
	styles, err := speakers.LoadStyles(ctx, voicevoxClient)
	if err != nil {
		return nil, fmt.Errorf("load VOICEVOX data: %w", err)
	}

	// Reading overrides are passed here. The synthesis engine only sees converted text, so the
	// reading vocabulary is a converter setting, not an engine setting.
	converter, err := phonetic.NewConverter(o.converter...)
	if err != nil {
		return nil, fmt.Errorf("initialize reading converter: %w", err)
	}

	return internalengine.New(voicevoxClient, styles, converter, o.engine...), nil
}
