package bot

import "testing"

func TestTrimButtonText(t *testing.T) {
	got := trimButtonText("abcdefghijklmnopqrstuvwxyz", 10)
	if got != "abcdefg..." {
		t.Fatalf("expected trimmed text, got %q", got)
	}
}

func TestTrimButtonTextKeepsShortText(t *testing.T) {
	got := trimButtonText("short", 10)
	if got != "short" {
		t.Fatalf("expected short text unchanged, got %q", got)
	}
}
