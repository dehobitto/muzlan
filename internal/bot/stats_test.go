package bot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dehobitto/muzlan/internal/ratelimit"
	"github.com/dehobitto/muzlan/internal/search"
	"github.com/dehobitto/muzlan/internal/stats"
	"github.com/dehobitto/muzlan/internal/telegram"
	"github.com/dehobitto/muzlan/internal/youtube"
)

// telegramCalls records each Bot API method called and its raw body.
type telegramCalls struct {
	mu    sync.Mutex
	calls []string
}

func (c *telegramCalls) add(call string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// JSON escapes <, > and &; undo it so tests can match HTML.
	c.calls = append(c.calls, strings.NewReplacer("\\u003c", "<", "\\u003e", ">", "\\u0026", "&").Replace(call))
}

func (c *telegramCalls) joined() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strings.Join(c.calls, "\n")
}

func newStatsTestHandler(t *testing.T, results []search.Result) (*Handler, *telegramCalls, *stats.Recorder) {
	t.Helper()
	calls := &telegramCalls{}
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		calls.add(r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:] + " " + string(body))
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":1,"chat":{"id":7}}}`)),
		}, nil
	})}

	events, err := stats.Open(filepath.Join(t.TempDir(), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(
		HandlerConfig{QueryMinLength: 3, QueryMaxLength: 10, ResultLimit: 30, SearchTimeout: time.Second, AdminUserIDs: map[int64]bool{1: true}},
		telegram.NewClient("test", httpClient),
		search.NewService(stubSearch(results), time.Minute, time.Now),
		nil,
		ratelimit.NewPerUser(0, time.Now),
		events,
		log.New(io.Discard, "", 0),
	)
	return handler, calls, events
}

func messageFrom(userID int64, text string) telegram.Update {
	return telegram.Update{Message: &telegram.Message{Text: text, Chat: telegram.Chat{ID: 7}, From: telegram.User{ID: userID}}}
}

func TestStatsForAdmin(t *testing.T) {
	handler, calls, events := newStatsTestHandler(t, []search.Result{{Title: "t", URL: "u"}})

	handler.HandleUpdate(context.Background(), messageFrom(2, "song"))
	recorded, _ := stats.Load(events.Path())
	if len(recorded) != 1 || recorded[0].Type != stats.TypeSearch || !recorded[0].OK || recorded[0].Results != 1 || recorded[0].User != events.User(2) {
		t.Fatalf("unexpected search event: %#v", recorded)
	}

	handler.HandleUpdate(context.Background(), messageFrom(1, "/stats"))
	got := calls.joined()
	assertContains(t, got, "sendRichMessage", "<b>Stats</b>", "<tr><td>Searches</td><td>1</td><td>1</td><td>1</td></tr>", "sendDocument", "All 1 events")
}

func TestStatsHiddenFromOthers(t *testing.T) {
	handler, calls, _ := newStatsTestHandler(t, nil)

	handler.HandleUpdate(context.Background(), messageFrom(2, "/stats"))
	got := calls.joined()
	if strings.Contains(got, "sendRichMessage") || strings.Contains(got, "sendDocument") || !strings.Contains(got, msgHelp) {
		t.Fatalf("non-admin must get the help text only, got:\n%s", got)
	}
}

func TestSignInAlertOncePerHour(t *testing.T) {
	handler, calls, _ := newStatsTestHandler(t, nil)

	handler.alertSignIn(context.Background())
	handler.alertSignIn(context.Background())
	if got := strings.Count(calls.joined(), "YouTube asks to sign in"); got != 1 {
		t.Fatalf("expected 1 alert, got %d", got)
	}

	handler.lastAlert = time.Now().Add(-alertInterval)
	handler.alertSignIn(context.Background())
	if got := strings.Count(calls.joined(), "YouTube asks to sign in"); got != 2 {
		t.Fatalf("expected a new alert after an hour, got %d", got)
	}
}

func TestFailureReason(t *testing.T) {
	signIn := search.TemporaryError{Err: youtube.DownloadError{Err: errors.New("exit 1"), Stderr: "ERROR: [youtube] x: Sign in to confirm you're not a bot."}}
	for _, test := range []struct {
		err  error
		want string
	}{
		{signIn, stats.ReasonSignIn},
		{fmt.Errorf("wrapped: %w", context.DeadlineExceeded), stats.ReasonTimeout},
		{search.TemporaryError{Err: youtube.DownloadError{Err: errors.New("x"), Stderr: "HTTP 403"}}, stats.ReasonYouTube},
	} {
		if got := failureReason(test.err); got != test.want {
			t.Fatalf("failureReason(%v) = %q, want %q", test.err, got, test.want)
		}
	}
}

func TestBar(t *testing.T) {
	for _, test := range []struct {
		count, most int
		want        string
	}{
		{0, 10, ""},
		{10, 10, strings.Repeat("█", statsBarWidth)},
		{5, 10, strings.Repeat("█", statsBarWidth/2)},
		{1, 100, "█"}, // never invisible
	} {
		if got := bar(test.count, test.most); got != test.want {
			t.Fatalf("bar(%d, %d) = %q, want %q", test.count, test.most, got, test.want)
		}
	}
}

func TestRichStats(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	summary := stats.Summarize([]stats.Event{
		{Time: now, Type: stats.TypeDownload, User: "a", OK: true, MS: 9000},
		{Time: now, Type: stats.TypeDownload, User: "a", Reason: stats.ReasonSignIn},
	}, now, statsDays)

	got := richStats(summary).HTML
	assertContains(t, got,
		"<tr><td>Downloads</td><td>1</td><td>1</td><td>1</td></tr>",
		"<tr><td>failed</td><td>1</td><td>1</td><td>1</td></tr>",
		"<h4>Downloads, last 14 days</h4>",
		"<tr><td>11.27</td><td>"+strings.Repeat("█", statsBarWidth)+"</td><td>1</td></tr>",
		"YouTube sign-in 1 · timeout 0",
		"download 9.0s / 9.0s",
	)
}
