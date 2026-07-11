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
