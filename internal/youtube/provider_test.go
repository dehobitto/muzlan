package youtube

import (
	"testing"

	ytdlp "github.com/lrstanley/go-ytdlp"
)

func TestYoutubeURLBuildsWatchURLFromID(t *testing.T) {
	got := youtubeURL(&ytdlp.ExtractedInfo{ID: "abc123"})
	if got != "https://www.youtube.com/watch?v=abc123" {
		t.Fatalf("unexpected url: %q", got)
	}
}

func TestResultFromInfoUsesCreatorFallbacks(t *testing.T) {
	title := "Song"
	uploader := "Uploader"

	got := resultFromInfo(&ytdlp.ExtractedInfo{
		ID:       "abc123",
		Title:    &title,
		Uploader: &uploader,
	})

	if got.Label() != "Song - Uploader" {
		t.Fatalf("unexpected label: %q", got.Label())
	}
}

func TestIsYouTubeURL(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{value: "https://www.youtube.com/watch?v=abc123", want: true},
		{value: "https://youtu.be/abc123", want: true},
		{value: "https://example.com/watch?v=abc123", want: false},
	}

	for _, tt := range tests {
		if got := IsYouTubeURL(tt.value); got != tt.want {
			t.Fatalf("IsYouTubeURL(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}
