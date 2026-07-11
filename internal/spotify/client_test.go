package spotify

import (
	"errors"
	"testing"
	"time"
)

func TestTrackLabel(t *testing.T) {
	track := Track{Name: "One More Time", Artists: []string{"Daft Punk"}}
	if track.Label() != "One More Time - Daft Punk" {
		t.Fatalf("unexpected label: %q", track.Label())
	}
}

func TestRetryAfterDuration(t *testing.T) {
	if retryAfterDuration("5") != 5*time.Second {
		t.Fatal("expected retry-after seconds")
	}
	if retryAfterDuration("nope") != 0 {
		t.Fatal("expected invalid retry-after to be zero")
	}
}

func TestIsTemporary(t *testing.T) {
	if !IsTemporary(temporaryError{statusCode: 500}) {
		t.Fatal("expected temporary error")
	}
	if IsTemporary(errors.New("other")) {
		t.Fatal("did not expect generic error to be temporary")
	}
}
