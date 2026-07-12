package bot

import (
	"testing"
	"time"
)

func TestDownloadButtonsExpire(t *testing.T) {
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	buttons := newDownloadButtons(time.Minute, func() time.Time { return now })

	data := buttons.put("https://youtu.be/abc123")
	if got, ok := buttons.get(data); !ok || got != "https://youtu.be/abc123" {
		t.Fatalf("expected stored URL, got %q, %v", got, ok)
	}

	now = now.Add(time.Minute)
	if got, ok := buttons.get(data); ok || got != "" {
		t.Fatalf("expected expired URL, got %q, %v", got, ok)
	}
}
