package speaker

import (
	"reflect"
	"testing"
)

func TestSupportedSpeakerNames(t *testing.T) {
	got := SupportedSpeakerNames()
	want := []string{"めたん", "ずんだもん", "つむぎ"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SupportedSpeakerNames() = %v, want %v", got, want)
	}
}

func TestSupportedStyleNames(t *testing.T) {
	got := SupportedStyleNames()
	want := []string{"ノーマル", "あまあま", "ツンツン", "セクシー", "ささやき"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SupportedStyleNames() = %v, want %v", got, want)
	}
}
