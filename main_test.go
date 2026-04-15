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
	if strings.TrimSpace(inputScript) == "" {
		t.Fatal("inputScript should not be empty")
	}
	if !strings.Contains(inputScript, "[ずんだもん][ノーマル]") {
		t.Fatal("inputScript should contain sample speaker tags")
	}
}
