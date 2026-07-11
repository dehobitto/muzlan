package bot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/dehobitto/muzlan/internal/ratelimit"
	"github.com/dehobitto/muzlan/internal/search"
	"github.com/dehobitto/muzlan/internal/spotify"
	"github.com/dehobitto/muzlan/internal/telegram"
)

type HandlerConfig struct {
	QueryMinLength int
	QueryMaxLength int
	ResultLimit    int
	SearchTimeout  time.Duration
	RetryCount     int
}

type Handler struct {
	cfg      HandlerConfig
	client   *telegram.Client
	searcher *search.Service
	limiter  *ratelimit.PerUser
	logger   *log.Logger
}

func NewHandler(cfg HandlerConfig, client *telegram.Client, searcher *search.Service, limiter *ratelimit.PerUser, logger *log.Logger) *Handler {
	return &Handler{
		cfg:      cfg,
		client:   client,
		searcher: searcher,
		limiter:  limiter,
		logger:   logger,
	}
}

func (h *Handler) HandleUpdate(ctx context.Context, update telegram.Update) {
	if update.Message == nil || update.Message.Text == "" {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	query := strings.TrimSpace(update.Message.Text)
	if query == "" {
		return
	}

	if strings.HasPrefix(query, "/start") || strings.HasPrefix(query, "/help") {
		_, _ = h.client.SendMessage(ctx, telegram.SendMessageRequest{
			ChatID: chatID,
			Text:   "Send a song name and I will search Spotify.",
		})
		return
	}

	if len([]rune(query)) < h.cfg.QueryMinLength {
		_, _ = h.client.SendMessage(ctx, telegram.SendMessageRequest{
			ChatID: chatID,
			Text:   fmt.Sprintf("Please send at least %d characters.", h.cfg.QueryMinLength),
		})
		return
	}

	if len([]rune(query)) > h.cfg.QueryMaxLength {
		_, _ = h.client.SendMessage(ctx, telegram.SendMessageRequest{
			ChatID: chatID,
			Text:   fmt.Sprintf("Please keep the search under %d characters.", h.cfg.QueryMaxLength),
		})
		return
	}

	if !h.limiter.Allow(userID) {
		_, _ = h.client.SendMessage(ctx, telegram.SendMessageRequest{
			ChatID: chatID,
			Text:   "Please wait a moment before searching again.",
		})
		return
	}

	status, err := h.client.SendMessage(ctx, telegram.SendMessageRequest{
		ChatID: chatID,
		Text:   "Got it. Searching Spotify...",
	})
	if err != nil {
		h.logger.Printf("send status message: %v", err)
		return
	}

	tracks, err := h.searchWithRetry(ctx, status.Chat.ID, status.MessageID, query)
	if err != nil {
		h.editOrSend(ctx, status.Chat.ID, status.MessageID, errorMessage(err), nil)
		return
	}

	if len(tracks) == 0 {
		h.editOrSend(ctx, status.Chat.ID, status.MessageID, fmt.Sprintf("No Spotify results found for %q.", query), nil)
		return
	}

	text := fmt.Sprintf("Found results for %q:", query)
	h.editOrSend(ctx, status.Chat.ID, status.MessageID, text, keyboardForTracks(tracks))
}

func (h *Handler) searchWithRetry(ctx context.Context, chatID int64, messageID int, query string) ([]spotify.Track, error) {
	var lastErr error
	for attempt := 0; attempt <= h.cfg.RetryCount; attempt++ {
		searchCtx, cancel := context.WithTimeout(ctx, h.cfg.SearchTimeout)
		tracks, err := h.searcher.Search(searchCtx, query, h.cfg.ResultLimit)
		cancel()
		if err == nil {
			return tracks, nil
		}

		lastErr = err
		if !shouldRetry(err) || attempt == h.cfg.RetryCount {
			break
		}

		h.editOrSend(ctx, chatID, messageID, "Spotify is slow. Retrying once...", nil)
	}

	return nil, lastErr
}

func shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	var rateLimited spotify.RateLimitedError
	if errors.As(err, &rateLimited) {
		return false
	}

	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || spotify.IsTemporary(err)
}

func errorMessage(err error) string {
	var rateLimited spotify.RateLimitedError
	if errors.As(err, &rateLimited) {
		return "Spotify asked us to slow down. Try again shortly."
	}

	return "Could not search Spotify right now. Please try again in a minute."
}

func (h *Handler) editOrSend(ctx context.Context, chatID int64, messageID int, text string, markup *telegram.InlineKeyboardMarkup) {
	err := h.client.EditMessageText(ctx, telegram.EditMessageTextRequest{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ReplyMarkup: markup,
	})
	if err == nil {
		return
	}

	_, sendErr := h.client.SendMessage(ctx, telegram.SendMessageRequest{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: markup,
	})
	if sendErr != nil {
		h.logger.Printf("send fallback message: %v", sendErr)
	}
}

func keyboardForTracks(tracks []spotify.Track) *telegram.InlineKeyboardMarkup {
	rows := make([][]telegram.InlineKeyboardButton, 0, len(tracks))
	for i, track := range tracks {
		rows = append(rows, []telegram.InlineKeyboardButton{{
			Text: fmt.Sprintf("%02d. %s", i+1, trimButtonText(track.Label(), 56)),
			URL:  track.URL,
		}})
	}

	return &telegram.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func trimButtonText(text string, maxRunes int) string {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}
