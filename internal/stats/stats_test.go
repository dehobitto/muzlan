package stats

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecorderRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "events.jsonl")
	recorder, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	user := recorder.User(42)
	if user == "" || user == "42" || user != recorder.User(42) || user == recorder.User(43) {
		t.Fatalf("user hash must be stable, distinct and not the raw id: %q", user)
	}
	for _, event := range []Event{
		{Type: TypeSearch, User: user, Results: 10, MS: 1200},
		{Type: TypeDownload, User: user, OK: true, MS: 9000},
	} {
		if err := recorder.Record(event); err != nil {
			t.Fatal(err)
		}
	}

	// Reopening keeps the salt, so the same user gets the same hash.
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.User(42) != user {
		t.Fatal("salt must survive a restart")
	}

	// A malformed line is skipped, not fatal.
	file, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	_, _ = file.WriteString("not json\n")
	_ = file.Close()

	events, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Type != TypeSearch || events[1].MS != 9000 || events[0].Time.IsZero() {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestDisabledRecorder(t *testing.T) {
	recorder, err := Open("")
	if err != nil || recorder != nil {
		t.Fatalf("empty path must disable recording, got %v %v", recorder, err)
	}
	if err := recorder.Record(Event{Type: TypeSearch}); err != nil || recorder.User(1) != "" || recorder.Path() != "" {
		t.Fatal("nil recorder must be a no-op")
	}
	if events, err := Load(filepath.Join(t.TempDir(), "missing")); err != nil || events != nil {
		t.Fatalf("missing file must have no events, got %v %v", events, err)
	}
}

func TestSummarize(t *testing.T) {
	now := time.Date(2026, 11, 27, 15, 0, 0, 0, time.UTC)
	day := func(daysAgo int) time.Time { return now.AddDate(0, 0, -daysAgo) }

	events := []Event{
		{Time: day(30), Type: TypeSearch, User: "old", MS: 1000},
		{Time: day(30), Type: TypeDownload, User: "old", OK: true, MS: 10000},
		{Time: day(3), Type: TypeSearch, User: "a", MS: 2000},
		{Time: day(3), Type: TypeDownload, User: "a", OK: true, MS: 20000},
		{Time: day(0), Type: TypeSearch, User: "a", MS: 3000},
		{Time: day(0), Type: TypeDownload, User: "a", Reason: ReasonSignIn},
		{Time: day(0), Type: TypeSearch, User: "b", MS: 4000},
		{Time: day(0), Type: TypeDownload, User: "b", OK: true, MS: 30000},
		{Time: day(0), Type: TypePage, User: "b"},
	}
	got := Summarize(events, now, 14)

	for name, check := range map[string]struct{ got, want [3]int }{
		"downloads": {got.Downloads, [3]int{1, 2, 3}},
		"failed":    {got.Failed, [3]int{1, 1, 1}},
		"users":     {got.Users, [3]int{2, 2, 3}},
		"new users": {got.NewUsers, [3]int{1, 2, 3}},
		"searches":  {got.Searches, [3]int{2, 3, 4}},
	} {
		if check.got != check.want {
			t.Fatalf("%s: got %v, want %v", name, check.got, check.want)
		}
	}

	if len(got.DailyDownloads) != 14 || got.DailyDownloads[13].Count != 1 || got.DailyDownloads[10].Count != 1 || got.DailyDownloads[0].Count != 0 {
		t.Fatalf("unexpected daily downloads: %#v", got.DailyDownloads)
	}
	if !got.DailyDownloads[13].Day.Equal(time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("last day must be today, got %v", got.DailyDownloads[13].Day)
	}
	if got.Failures[ReasonSignIn] != 1 {
		t.Fatalf("unexpected failures: %v", got.Failures)
	}
	if got.SearchMedian != 2*time.Second || got.SearchP95 != 4*time.Second || got.DownloadMedian != 20*time.Second {
		t.Fatalf("unexpected times: search %v/%v download %v", got.SearchMedian, got.SearchP95, got.DownloadMedian)
	}
}
