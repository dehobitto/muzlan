package youtube

import (
	"errors"
	"strings"
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

func TestDownloadErrorDiagnostics(t *testing.T) {
	err := newDownloadError("https://youtu.be/abc123", &ytdlp.Result{
		ExitCode: 1,
		Stdout:   "out",
		Stderr:   "yt-dlp failed",
	}, []string{"partial.webm(100 bytes)"}, errors.New("failed"))

	diagnostics := err.Diagnostics()
	for _, want := range []string{
		`url="https://youtu.be/abc123"`,
		"exit_code=1",
		"stdout=out",
		"stderr=yt-dlp failed",
		"files=partial.webm(100 bytes)",
		"error=failed",
	} {
		if !strings.Contains(diagnostics, want) {
			t.Fatalf("expected diagnostics to contain %q, got %q", want, diagnostics)
		}
	}
}
