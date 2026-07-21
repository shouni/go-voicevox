package voicevox

import (
	"context"
	"testing"
)

func TestNewReturnsNoopEngineWhenDisabled(t *testing.T) {
	engine, err := New(context.Background(), nil, nil, "", false)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if engine == nil {
		t.Fatal("New() returned nil engine")
	}
	if err := engine.Run(context.Background(), "", "sample"); err != nil {
		t.Fatalf("noop Run() error = %v", err)
	}
	if err := engine.RunScript(context.Background(), "", []ScriptLine{{Speaker: "ずんだもん", Style: "ノーマル", Text: "sample"}}); err != nil {
		t.Fatalf("noop RunScript() error = %v", err)
	}
}
