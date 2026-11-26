package bot

import (
	"testing"
	"time"

	"github.com/dehobitto/muzlan/internal/search"
)

func TestNewResultsPage(t *testing.T) {
	results := make([]search.Result, 12)
	tests := []struct {
		page, wantPage, wantFirst, wantLen, wantPages int
	}{
		{page: 1, wantPage: 1, wantFirst: 1, wantLen: 5, wantPages: 3},
		{page: 3, wantPage: 3, wantFirst: 11, wantLen: 2, wantPages: 3},
		{page: 9, wantPage: 3, wantFirst: 11, wantLen: 2, wantPages: 3}, // clamped
		{page: 0, wantPage: 1, wantFirst: 1, wantLen: 5, wantPages: 3},
	}
	for _, test := range tests {
		got := newResultsPage("q", results, test.page)
		if got.page != test.wantPage || got.first != test.wantFirst || len(got.results) != test.wantLen || got.pages != test.wantPages {
			t.Fatalf("page %d: got page=%d first=%d len=%d pages=%d", test.page, got.page, got.first, len(got.results), got.pages)
		}
	}
}

func TestPageCallbackRoundTrip(t *testing.T) {
	token, page, ok := parsePageCallback(pageCallback("abc", 4))
	if !ok || token != "abc" || page != 4 {
		t.Fatalf("got %q %d %v", token, page, ok)
	}
	if len(pageCallback(randomToken(), 10)) > 64 {
		t.Fatal("callback data must fit Telegram's 64 bytes")
	}
	for _, bad := range []string{"page:", "page:abc", "page:abc:x", "page::2", "page:abc:0", "download:abc"} {
		if _, _, ok := parsePageCallback(bad); ok {
			t.Fatalf("expected %q to be rejected", bad)
		}
	}
}

func TestSearchPagesExpire(t *testing.T) {
	now := time.Unix(0, 0)
	pages := newSearchPages(time.Minute, func() time.Time { return now })

	token := pages.put("tool")
	if query, ok := pages.get(token); !ok || query != "tool" {
		t.Fatalf("got %q %v", query, ok)
	}
	now = now.Add(time.Minute)
	if _, ok := pages.get(token); ok {
		t.Fatal("expected token to expire")
	}
}
