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

func TestAudioMetadataUsesTitleAsFilename(t *testing.T) {
	got := audioMetadata(`Artist: "Song"?`, "audio.m4a")
	if got.title != `Artist: "Song"?` {
		t.Fatalf("unexpected title: %q", got.title)
	}
	if got.filename != "Artist_ _Song__.m4a" {
		t.Fatalf("unexpected filename: %q", got.filename)
	}
}
