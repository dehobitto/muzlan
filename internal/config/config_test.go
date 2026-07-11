package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvDoesNotOverrideExistingEnv(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "from-env")

	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	err := os.WriteFile(path, []byte("TELEGRAM_BOT_TOKEN=from-file\nSPOTIFY_CLIENT_ID=id\nSPOTIFY_CLIENT_SECRET=secret\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.TelegramBotToken != "from-env" {
		t.Fatalf("expected env value to win, got %q", cfg.TelegramBotToken)
	}
	if cfg.SpotifyClientID != "id" {
		t.Fatalf("expected spotify client id from file, got %q", cfg.SpotifyClientID)
	}
}

func TestValidateRequiresCredentials(t *testing.T) {
	err := (Config{}).Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
}
