package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestSendAudio(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sendAudio" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1024 * 1024); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("chat_id") != "7" {
			t.Fatalf("unexpected chat id: %s", r.FormValue("chat_id"))
		}
		if r.MultipartForm.File["audio"] == nil {
			t.Fatal("expected audio upload")
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":43,"chat":{"id":7},"from":{"id":1},"text":""}}`))
	}))
	defer server.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "audio.mp3")
	if err := os.WriteFile(path, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	message, err := client.SendAudio(context.Background(), SendAudioRequest{ChatID: 7, AudioPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if message.MessageID != 43 {
		t.Fatalf("expected message id 43, got %d", message.MessageID)
	}
}
