package search

import "testing"

func TestNormalize(t *testing.T) {
	got := normalize("  Daft   Punk  ")
	if got != "daft punk" {
		t.Fatalf("expected normalized query, got %q", got)
	}
}
