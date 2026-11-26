package bot

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dehobitto/muzlan/internal/ratelimit"
	"github.com/dehobitto/muzlan/internal/search"
	"github.com/dehobitto/muzlan/internal/telegram"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type stubSearch []search.Result

func (s stubSearch) Search(context.Context, string, int) ([]search.Result, error) { return s, nil }

// newTestHandler returns a handler and the texts (or rich HTML) it sends to
// Telegram. With rejectRich, Telegram answers rich messages with an error;
// with rejectImages, only rich messages that contain an image.
func newTestHandler(results []search.Result, rejectRich bool, rejectImages bool) (*Handler, *[]string) {
	texts := &[]string{}
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body struct {
			Text        string `json:"text"`
			RichMessage *struct {
				HTML string `json:"html"`
			} `json:"rich_message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return nil, err
		}
		if body.RichMessage != nil {
			*texts = append(*texts, body.RichMessage.HTML)
			if rejectRich || (rejectImages && strings.Contains(body.RichMessage.HTML, "<img")) {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader(`{"ok":false,"error_code":400,"description":"Bad Request"}`)),
				}, nil
			}
		} else {
			*texts = append(*texts, body.Text)
		}

		result := `{"message_id":1,"chat":{"id":7}}`
		if strings.HasSuffix(r.URL.Path, "/answerCallbackQuery") {
			result = "true"
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":` + result + `}`)),
		}, nil
	})}

	handler := NewHandler(
		HandlerConfig{QueryMinLength: 3, QueryMaxLength: 10, ResultLimit: 30, SearchTimeout: time.Second},
		telegram.NewClient("test", httpClient),
		search.NewService(stubSearch(results), time.Minute, time.Now),
		nil,
		ratelimit.NewPerUser(time.Hour, time.Now),
		nil,
		log.New(io.Discard, "", 0),
	)
	return handler, texts
}

func message(text string) telegram.Update {
	return telegram.Update{Message: &telegram.Message{Text: text, Chat: telegram.Chat{ID: 7}, From: telegram.User{ID: 1}}}
}

func TestHandleUpdateReplies(t *testing.T) {
	tests := []struct {
		name         string
		results      []search.Result
		rejectRich   bool
		rejectImages bool
		updates      []telegram.Update
		want         string
	}{
		{name: "help", updates: []telegram.Update{message("/start")}, want: msgHelp},
		{name: "short query", updates: []telegram.Update{message("ab")}, want: "Please send at least 3 characters."},
		{name: "long query", updates: []telegram.Update{message("abcdefghijk")}, want: "Please keep the search under 10 characters."},
		{name: "no results", updates: []telegram.Update{message("song")}, want: `No links found for "song".`},
		{name: "rich results", results: []search.Result{{Title: "t", URL: "u", Thumbnail: "https://x/t.jpg"}}, updates: []telegram.Update{message("song")}, want: `<table bordered><tr><td align="center">&nbsp;<br><b>song...</b><br>&nbsp;</td></tr></table><img`},
		{name: "no thumbnails when they are rejected", results: []search.Result{{Title: "t", URL: "u", Thumbnail: "https://x/t.jpg"}}, rejectImages: true, updates: []telegram.Update{message("song")}, want: `<table bordered><tr><td align="center">&nbsp;<br><b>song...</b><br>&nbsp;</td></tr></table><table bordered><tr><td>01</td><td><b>t</b></td>`},
		{name: "plain results when rich is rejected", results: []search.Result{{Title: "t", URL: "u"}}, rejectRich: true, updates: []telegram.Update{message("song")}, want: `Found results for "song":`},
		{name: "cooldown", updates: []telegram.Update{message("song"), message("song")}, want: "Please wait a moment before searching again."},
		{
			name:    "expired button",
			updates: []telegram.Update{{CallbackQuery: &telegram.CallbackQuery{ID: "1", Data: "download:missing"}}},
			want:    "This download button expired. Search again.",
		},
		{
			name:    "expired page",
			updates: []telegram.Update{{CallbackQuery: &telegram.CallbackQuery{ID: "1", Data: "page:missing:2"}}},
			want:    "These results expired. Search again.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, texts := newTestHandler(test.results, test.rejectRich, test.rejectImages)
			for _, update := range test.updates {
				handler.HandleUpdate(context.Background(), update)
			}
			if len(*texts) == 0 || !strings.HasPrefix((*texts)[len(*texts)-1], test.want) {
				t.Fatalf("expected last reply %q, got %q", test.want, *texts)
			}
		})
	}
}

func TestHandleUpdatePages(t *testing.T) {
	results := make([]search.Result, 7) // 2 pages
	for i := range results {
		results[i] = search.Result{Title: "t", URL: "u"}
	}
	handler, texts := newTestHandler(results, false, false)

	handler.HandleUpdate(context.Background(), message("song"))
	first := (*texts)[len(*texts)-1]
	assertContains(t, first, "<sub>1 / 2</sub>", "<td>05</td>")
	if strings.Contains(first, "<td>06</td>") {
		t.Fatalf("first page must show 5 results:\n%s", first)
	}

	next := regexp.MustCompile(`data="(page:[^"]+)">Next`).FindStringSubmatch(first)
	if next == nil {
		t.Fatalf("no Next button in:\n%s", first)
	}
	handler.HandleUpdate(context.Background(), telegram.Update{CallbackQuery: &telegram.CallbackQuery{
		ID: "2", Data: next[1], From: telegram.User{ID: 1},
		Message: &telegram.Message{MessageID: 1, Chat: telegram.Chat{ID: 7}},
	}})
	assertContains(t, (*texts)[len(*texts)-1], "<sub>2 / 2</sub>", "<td>06</td>", "<td>07</td>", "‹ Back")
}
