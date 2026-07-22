package main

import (
	"strings"
	"testing"
)

func TestDemoConfigurationConstants(t *testing.T) {
	if appClientTimeout <= 0 {
		t.Fatalf("appClientTimeout = %v, want positive", appClientTimeout)
	}
	if !strings.HasSuffix(outputFilename, ".wav") {
		t.Fatalf("outputFilename = %q, want .wav suffix", outputFilename)
	}
	if len(inputScriptLines) == 0 {
		t.Fatal("inputScriptLines should not be empty")
	}
	found := false
	for _, line := range inputScriptLines {
		if line.Speaker == "ずんだもん" && line.Style == "ノーマル" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("inputScriptLines should contain a sample ずんだもん/ノーマル line")
	}
}
