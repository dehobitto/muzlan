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
	err := os.WriteFile(path, []byte("TELEGRAM_BOT_TOKEN=from-file\nBOT_YTDLP_AUTO_INSTALL=false\nBOT_ADMIN_USER_IDS=12, 34\n"), 0o600)
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
	if cfg.YTDLPAutoInstall {
		t.Fatal("expected ytdlp auto install to come from file")
	}
	if !cfg.AdminUserIDs[12] || !cfg.AdminUserIDs[34] {
		t.Fatalf("expected admin user ids from file, got %#v", cfg.AdminUserIDs)
	}
}

func TestValidateRequiresCredentials(t *testing.T) {
	err := (Config{}).Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestInt64SetEnvIgnoresInvalidValues(t *testing.T) {
	t.Setenv("BOT_ADMIN_USER_IDS", "1, nope, 2")

	got := int64SetEnv("BOT_ADMIN_USER_IDS")
	if len(got) != 2 || !got[1] || !got[2] {
		t.Fatalf("unexpected ids: %#v", got)
	}
}
