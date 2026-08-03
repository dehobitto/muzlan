package bot

import (
	"testing"
	"time"
)

func TestDownloadButtonsExpire(t *testing.T) {
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	buttons := newDownloadButtons(time.Minute, func() time.Time { return now })

	data := buttons.put("https://youtu.be/abc123", "Artist - Song")
	if got, ok := buttons.get(data); !ok || got.url != "https://youtu.be/abc123" || got.title != "Artist - Song" {
		t.Fatalf("expected stored entry, got %#v, %v", got, ok)
	}

	now = now.Add(time.Minute)
	if got, ok := buttons.get(data); ok || got.url != "" {
		t.Fatalf("expected expired URL, got %#v, %v", got, ok)
	}
}
