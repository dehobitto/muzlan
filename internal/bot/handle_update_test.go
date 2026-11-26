package bot

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
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

// newTestHandler returns a handler and the texts it sends to Telegram.
func newTestHandler(results []search.Result) (*Handler, *[]string) {
	texts := &[]string{}
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return nil, err
		}
		*texts = append(*texts, body.Text)

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
		HandlerConfig{QueryMinLength: 3, QueryMaxLength: 10, ResultLimit: 10, SearchTimeout: time.Second},
		telegram.NewClient("test", httpClient),
		search.NewService(stubSearch(results), time.Minute, time.Now),
		nil,
		ratelimit.NewPerUser(time.Hour, time.Now),
		log.New(io.Discard, "", 0),
	)
	return handler, texts
}

func message(text string) telegram.Update {
	return telegram.Update{Message: &telegram.Message{Text: text, Chat: telegram.Chat{ID: 7}, From: telegram.User{ID: 1}}}
}

func TestHandleUpdateReplies(t *testing.T) {
	tests := []struct {
		name    string
		results []search.Result
		updates []telegram.Update
		want    string
	}{
		{name: "help", updates: []telegram.Update{message("/start")}, want: msgHelp},
		{name: "short query", updates: []telegram.Update{message("ab")}, want: "Please send at least 3 characters."},
		{name: "long query", updates: []telegram.Update{message("abcdefghijk")}, want: "Please keep the search under 10 characters."},
		{name: "no results", updates: []telegram.Update{message("song")}, want: `No links found for "song".`},
		{name: "results", results: []search.Result{{Title: "t", URL: "u"}}, updates: []telegram.Update{message("song")}, want: `Found results for "song":`},
		{name: "cooldown", updates: []telegram.Update{message("song"), message("song")}, want: "Please wait a moment before searching again."},
		{
			name:    "expired button",
			updates: []telegram.Update{{CallbackQuery: &telegram.CallbackQuery{ID: "1", Data: "download:missing"}}},
			want:    "This download button expired. Search again.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, texts := newTestHandler(test.results)
			for _, update := range test.updates {
				handler.HandleUpdate(context.Background(), update)
			}
			if len(*texts) == 0 || (*texts)[len(*texts)-1] != test.want {
				t.Fatalf("expected last reply %q, got %q", test.want, *texts)
			}
		})
	}
}
