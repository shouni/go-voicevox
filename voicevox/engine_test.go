package voicevox

import (
	"context"
	"testing"
)

func TestNewReturnsNoopEngineWhenDisabled(t *testing.T) {
	engine, err := New(context.Background(), nil, "", false)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if engine == nil {
		t.Fatal("New() returned nil engine")
	}
	if _, err := engine.Run(context.Background(), []ScriptLine{{Speaker: "ずんだもん", Style: "ノーマル", Text: "sample"}}); err != nil {
		t.Fatalf("noop Run() error = %v", err)
	}
}
