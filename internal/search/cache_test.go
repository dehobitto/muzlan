package search

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeProvider struct {
	calls int
	err   error
}

func (p *fakeProvider) Search(context.Context, string, int) ([]Result, error) {
	p.calls++
	return []Result{{Title: "song"}}, p.err
}

func TestServiceCachesUntilTTL(t *testing.T) {
	now := time.Unix(0, 0)
	provider := &fakeProvider{}
	service := NewService(provider, time.Minute, func() time.Time { return now })

	search := func(query string) []Result {
		t.Helper()
		results, err := service.Search(context.Background(), query, 10)
		if err != nil {
			t.Fatal(err)
		}
		return results
	}

	search("Daft Punk")[0].Title = "changed by caller"
	cached := search("  daft   punk ")
	cached[0].Title = "changed again"
	if got := search("daft punk")[0].Title; got != "song" {
		t.Fatalf("cache was mutated by caller: %q", got)
	}
	if provider.calls != 1 {
		t.Fatalf("expected 1 provider call, got %d", provider.calls)
	}

	now = now.Add(time.Minute)
	search("daft punk")
	if provider.calls != 2 {
		t.Fatalf("expected expired entry to be refetched, got %d calls", provider.calls)
	}
}

func TestServiceDoesNotCacheErrors(t *testing.T) {
	failure := errors.New("search failed")
	provider := &fakeProvider{err: failure}
	service := NewService(provider, time.Minute, time.Now)

	for range 2 {
		if _, err := service.Search(context.Background(), "song", 10); !errors.Is(err, failure) {
			t.Fatalf("expected provider error, got %v", err)
		}
	}
	if provider.calls != 2 {
		t.Fatalf("expected 2 provider calls, got %d", provider.calls)
	}
}
