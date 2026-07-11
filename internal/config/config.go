package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	TelegramBotToken string
	YTDLPAutoInstall bool
	SkipOldUpdates   bool
	PollTimeout      time.Duration
	SearchTimeout    time.Duration
	RetryCount       int
	UserCooldown     time.Duration
	CacheTTL         time.Duration
	QueryMinLength   int
	QueryMaxLength   int
	ResultLimit      int
}

func Load(envPath string) (Config, error) {
	if envPath != "" {
		if err := loadDotEnv(envPath); err != nil {
			return Config{}, err
		}
	}

	cfg := Config{
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		YTDLPAutoInstall: boolEnv("BOT_YTDLP_AUTO_INSTALL", true),
		SkipOldUpdates:   boolEnv("BOT_SKIP_OLD_UPDATES", true),
		PollTimeout:      secondsEnv("BOT_POLL_TIMEOUT_SECONDS", 30),
		SearchTimeout:    secondsEnv("BOT_SEARCH_TIMEOUT_SECONDS", 15),
		RetryCount:       intEnv("BOT_RETRY_COUNT", 1),
		UserCooldown:     secondsEnv("BOT_USER_COOLDOWN_SECONDS", 3),
		CacheTTL:         secondsEnv("BOT_CACHE_TTL_SECONDS", 600),
		QueryMinLength:   intEnv("BOT_QUERY_MIN_LENGTH", 3),
		QueryMaxLength:   intEnv("BOT_QUERY_MAX_LENGTH", 100),
		ResultLimit:      intEnv("BOT_RESULT_LIMIT", 10),
	}

	return cfg, nil
}

func (c Config) Validate() error {
	var missing []string
	if strings.TrimSpace(c.TelegramBotToken) == "" {
		missing = append(missing, "TELEGRAM_BOT_TOKEN")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	if c.QueryMinLength < 1 {
		return errors.New("BOT_QUERY_MIN_LENGTH must be at least 1")
	}
	if c.QueryMaxLength < c.QueryMinLength {
		return errors.New("BOT_QUERY_MAX_LENGTH must be greater than or equal to BOT_QUERY_MIN_LENGTH")
	}
	if c.ResultLimit < 1 || c.ResultLimit > 10 {
		return errors.New("BOT_RESULT_LIMIT must be between 1 and 10")
	}
	if c.SearchTimeout <= 0 {
		return errors.New("BOT_SEARCH_TIMEOUT_SECONDS must be greater than 0")
	}
	if c.UserCooldown < 0 {
		return errors.New("BOT_USER_COOLDOWN_SECONDS cannot be negative")
	}
	if c.CacheTTL <= 0 {
		return errors.New("BOT_CACHE_TTL_SECONDS must be greater than 0")
	}
	return nil
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	return scanner.Err()
}

func intEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func secondsEnv(key string, fallback int) time.Duration {
	return time.Duration(intEnv(key, fallback)) * time.Second
}

func boolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
