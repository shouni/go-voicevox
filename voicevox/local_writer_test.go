package voicevox

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalWriterWritesFileAndCreatesDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "out.wav")

	w := NewLocalWriter()
	if err := w.Write(context.Background(), path, strings.NewReader("wav-bytes")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "wav-bytes" {
		t.Fatalf("content = %q, want %q", got, "wav-bytes")
	}
}

func TestLocalWriterOverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.wav")

	w := NewLocalWriter()
	if err := w.Write(context.Background(), path, strings.NewReader("first")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := w.Write(context.Background(), path, strings.NewReader("second")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "second" {
		t.Fatalf("content = %q, want %q", got, "second")
	}
}
