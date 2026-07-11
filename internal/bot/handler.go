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
	"github.com/dehobitto/muzlan/internal/telegram"
	"github.com/dehobitto/muzlan/internal/youtube"
)

type HandlerConfig struct {
	QueryMinLength  int
	QueryMaxLength  int
	ResultLimit     int
	SearchTimeout   time.Duration
	DownloadTimeout time.Duration
	RetryCount      int
	AdminUserIDs    map[int64]bool
}

type Handler struct {
	cfg        HandlerConfig
	client     *telegram.Client
	searcher   *search.Service
	downloader *youtube.Provider
	limiter    *ratelimit.PerUser
	logger     *log.Logger
}

func NewHandler(cfg HandlerConfig, client *telegram.Client, searcher *search.Service, downloader *youtube.Provider, limiter *ratelimit.PerUser, logger *log.Logger) *Handler {
	return &Handler{
		cfg:        cfg,
		client:     client,
		searcher:   searcher,
		downloader: downloader,
		limiter:    limiter,
		logger:     logger,
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
			Text:   h.helpText(userID),
		})
		return
	}

	if youtube.IsYouTubeURL(query) {
		h.handleDownload(ctx, chatID, userID, query)
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
		Text:   "Got it. Searching links...",
	})
	if err != nil {
		h.logger.Printf("send status message: %v", err)
		return
	}

	results, err := h.searchWithRetry(ctx, status.Chat.ID, status.MessageID, query)
	if err != nil {
		h.editOrSend(ctx, status.Chat.ID, status.MessageID, errorMessage(err), nil)
		return
	}

	if len(results) == 0 {
		h.editOrSend(ctx, status.Chat.ID, status.MessageID, fmt.Sprintf("No links found for %q.", query), nil)
		return
	}

	text := fmt.Sprintf("Found results for %q:", query)
	h.editOrSend(ctx, status.Chat.ID, status.MessageID, text, keyboardForResults(results))
}

func (h *Handler) handleDownload(ctx context.Context, chatID int64, userID int64, videoURL string) {
	if !h.isAdmin(userID) {
		_, _ = h.client.SendMessage(ctx, telegram.SendMessageRequest{
			ChatID: chatID,
			Text:   "MP3 conversion is available only to configured admins.",
		})
		return
	}

	if !h.limiter.Allow(userID) {
		_, _ = h.client.SendMessage(ctx, telegram.SendMessageRequest{
			ChatID: chatID,
			Text:   "Please wait a moment before starting another conversion.",
		})
		return
	}

	status, err := h.client.SendMessage(ctx, telegram.SendMessageRequest{
		ChatID: chatID,
		Text:   "Got it. Converting authorized video to MP3...",
	})
	if err != nil {
		h.logger.Printf("send conversion status message: %v", err)
		return
	}

	downloadCtx, cancel := context.WithTimeout(ctx, h.cfg.DownloadTimeout)
	audioPath, cleanup, err := h.downloader.DownloadMP3(downloadCtx, videoURL)
	cancel()
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		h.editOrSend(ctx, status.Chat.ID, status.MessageID, downloadErrorMessage(err), nil)
		return
	}

	h.editOrSend(ctx, status.Chat.ID, status.MessageID, "MP3 is ready. Sending audio...", nil)
	_, err = h.client.SendAudio(ctx, telegram.SendAudioRequest{
		ChatID:    chatID,
		AudioPath: audioPath,
		Caption:   "Converted from your authorized video.",
	})
	if err != nil {
		h.editOrSend(ctx, status.Chat.ID, status.MessageID, "MP3 was created, but Telegram upload failed. Please try again.", nil)
		h.logger.Printf("send audio: %v", err)
		return
	}

	h.editOrSend(ctx, status.Chat.ID, status.MessageID, "Done. Sent MP3.", nil)
}

func (h *Handler) searchWithRetry(ctx context.Context, chatID int64, messageID int, query string) ([]search.Result, error) {
	var lastErr error
	for attempt := 0; attempt <= h.cfg.RetryCount; attempt++ {
		searchCtx, cancel := context.WithTimeout(ctx, h.cfg.SearchTimeout)
		results, err := h.searcher.Search(searchCtx, query, h.cfg.ResultLimit)
		cancel()
		if err == nil {
			return results, nil
		}

		lastErr = err
		if !shouldRetry(err) || attempt == h.cfg.RetryCount {
			break
		}

		h.editOrSend(ctx, chatID, messageID, "Search is slow. Retrying once...", nil)
	}

	return nil, lastErr
}

func shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || search.IsTemporary(err)
}

func errorMessage(err error) string {
	return "Could not search links right now. Please try again in a minute."
}

func downloadErrorMessage(err error) string {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "Conversion took too long and was stopped. Please try a shorter authorized video."
	}

	return "Could not create MP3. Make sure this is your authorized video and ffmpeg is installed."
}

func (h *Handler) isAdmin(userID int64) bool {
	return h.cfg.AdminUserIDs[userID]
}

func (h *Handler) helpText(userID int64) string {
	if h.isAdmin(userID) {
		return "Send a song name to search YouTube links. Send your authorized YouTube video URL to receive an MP3."
	}
	return "Send a song name and I will search YouTube links."
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

func keyboardForResults(results []search.Result) *telegram.InlineKeyboardMarkup {
	rows := make([][]telegram.InlineKeyboardButton, 0, len(results))
	for i, result := range results {
		rows = append(rows, []telegram.InlineKeyboardButton{{
			Text: fmt.Sprintf("%02d. %s", i+1, trimButtonText(result.Label(), 56)),
			URL:  result.URL,
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
