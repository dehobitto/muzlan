package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sendMessage" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":42,"chat":{"id":7},"from":{"id":1},"text":"ok"}}`))
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	message, err := client.SendMessage(context.Background(), SendMessageRequest{ChatID: 7, Text: "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if message.MessageID != 42 {
		t.Fatalf("expected message id 42, got %d", message.MessageID)
	}
}
