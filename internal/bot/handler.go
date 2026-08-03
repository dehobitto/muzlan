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
	buttons    *downloadButtons
	limiter    *ratelimit.PerUser
	logger     *log.Logger
}

func NewHandler(cfg HandlerConfig, client *telegram.Client, searcher *search.Service, downloader *youtube.Provider, limiter *ratelimit.PerUser, logger *log.Logger) *Handler {
	return &Handler{
		cfg:        cfg,
		client:     client,
		searcher:   searcher,
		downloader: downloader,
		buttons:    newDownloadButtons(30*time.Minute, time.Now),
		limiter:    limiter,
		logger:     logger,
	}
}

const HelpText = "Send a song name to search. Tap a result to receive an MP3 for your authorized video."

func (h *Handler) HandleUpdate(ctx context.Context, update telegram.Update) {
	if update.CallbackQuery != nil {
		h.handleCallbackQuery(ctx, *update.CallbackQuery)
		return
	}

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
			Text:   HelpText,
		})
		return
	}

	if youtube.IsYouTubeURL(query) {
		h.handleDownload(ctx, chatID, userID, query, "audio", true)
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
	h.editOrSend(ctx, status.Chat.ID, status.MessageID, text, h.keyboardForResults(userID, results))
}

func (h *Handler) handleCallbackQuery(ctx context.Context, callback telegram.CallbackQuery) {
	if callback.Data == "" {
		_ = h.client.AnswerCallbackQuery(ctx, telegram.AnswerCallbackQueryRequest{
			CallbackQueryID: callback.ID,
			Text:            "Nothing to do for this button.",
		})
		return
	}

	//if !h.isAdmin(callback.From.ID) {
	//	_ = h.client.AnswerCallbackQuery(ctx, telegram.AnswerCallbackQueryRequest{
	//		CallbackQueryID: callback.ID,
	//		Text:            "MP3 conversion is available only to configured admins.",
	//		ShowAlert:       true,
	//	})
	//	return
	//}

	entry, ok := h.buttons.get(callback.Data)
	if !ok {
		_ = h.client.AnswerCallbackQuery(ctx, telegram.AnswerCallbackQueryRequest{
			CallbackQueryID: callback.ID,
			Text:            "This download button expired. Search again.",
			ShowAlert:       true,
		})
		return
	}

	_ = h.client.AnswerCallbackQuery(ctx, telegram.AnswerCallbackQueryRequest{
		CallbackQueryID: callback.ID,
		Text:            "Starting MP3 conversion...",
	})

	if callback.Message == nil {
		return
	}

	h.handleDownload(ctx, callback.Message.Chat.ID, callback.From.ID, entry.url, entry.title, false)
}

func (h *Handler) handleDownload(ctx context.Context, chatID int64, userID int64, videoURL string, title string, enforceCooldown bool) {
	//if !h.isAdmin(userID) {
	//	_, _ = h.client.SendMessage(ctx, telegram.SendMessageRequest{
	//		ChatID: chatID,
	//		Text:   "MP3 conversion is available only to configured admins.",
	//	})
	//	return
	//}

	if enforceCooldown && !h.limiter.Allow(userID) {
		_, _ = h.client.SendMessage(ctx, telegram.SendMessageRequest{
			ChatID: chatID,
			Text:   "Please wait a moment before starting another conversion.",
		})
		return
	}

	status, err := h.client.SendMessage(ctx, telegram.SendMessageRequest{
		ChatID: chatID,
		Text:   "Got it. Downloading audio...",
	})
	if err != nil {
		h.logger.Printf("send conversion status message: %v", err)
		return
	}

	h.logger.Printf("audio download started: chat_id=%d user_id=%d url=%q", chatID, userID, videoURL)
	downloadCtx, cancel := context.WithTimeout(ctx, h.cfg.DownloadTimeout)
	uploadErr, downloadErr := h.streamAudio(downloadCtx, chatID, userID, videoURL, title)
	cancel()
	if uploadErr != nil {
		if downloadErr != nil {
			h.logDownloadError(chatID, userID, videoURL, downloadErr)
		}
		h.logger.Printf("telegram audio stream upload failed, trying document stream: chat_id=%d user_id=%d error=%v", chatID, userID, uploadErr)
		h.editOrSend(ctx, status.Chat.ID, status.MessageID, "Telegram did not accept it as audio. Sending as file...", nil)

		documentCtx, documentCancel := context.WithTimeout(ctx, h.cfg.DownloadTimeout)
		documentUploadErr, documentDownloadErr := h.streamDocument(documentCtx, chatID, userID, videoURL, title)
		documentCancel()
		if documentUploadErr != nil {
			if documentDownloadErr != nil {
				h.logDownloadError(chatID, userID, videoURL, documentDownloadErr)
			}
			h.editOrSend(ctx, status.Chat.ID, status.MessageID, "Audio was downloaded, but Telegram upload failed. Please try again.", nil)
			h.logger.Printf("telegram document stream upload failed: chat_id=%d user_id=%d error=%v", chatID, userID, documentUploadErr)
			return
		}
		if documentDownloadErr != nil {
			h.logDownloadError(chatID, userID, videoURL, documentDownloadErr)
			h.editOrSend(ctx, status.Chat.ID, status.MessageID, downloadErrorMessage(documentDownloadErr), nil)
			return
		}
		h.logger.Printf("telegram document upload finished: chat_id=%d user_id=%d", chatID, userID)
		h.editOrSend(ctx, status.Chat.ID, status.MessageID, "Done. Sent audio file.", nil)
		return
	}
	if downloadErr != nil {
		h.logDownloadError(chatID, userID, videoURL, downloadErr)
		h.editOrSend(ctx, status.Chat.ID, status.MessageID, downloadErrorMessage(downloadErr), nil)
		return
	}

	h.logger.Printf("telegram audio upload finished: chat_id=%d user_id=%d", chatID, userID)
	h.editOrSend(ctx, status.Chat.ID, status.MessageID, "Done. Sent audio.", nil)
}

func (h *Handler) streamAudio(ctx context.Context, chatID int64, userID int64, videoURL string, title string) (error, error) {
	stream, err := h.downloader.StreamAudio(ctx, videoURL)
	if err != nil {
		return nil, err
	}
	metadata := audioMetadata(title, stream.Filename)

	h.logger.Printf("telegram audio stream upload started: chat_id=%d user_id=%d filename=%q title=%q", chatID, userID, metadata.filename, metadata.title)
	_, uploadErr := h.client.SendAudioStream(ctx, telegram.SendAudioStreamRequest{
		ChatID:   chatID,
		Audio:    stream.Reader,
		Filename: metadata.filename,
		Title:    metadata.title,
	})
	if uploadErr != nil {
		_ = stream.Close()
	}

	downloadErr := stream.Wait()
	if uploadErr == nil && downloadErr == nil {
		h.logger.Printf("telegram audio stream upload finished: chat_id=%d user_id=%d", chatID, userID)
	}
	return uploadErr, downloadErr
}

func (h *Handler) streamDocument(ctx context.Context, chatID int64, userID int64, videoURL string, title string) (error, error) {
	stream, err := h.downloader.StreamAudio(ctx, videoURL)
	if err != nil {
		return nil, err
	}
	metadata := audioMetadata(title, stream.Filename)

	h.logger.Printf("telegram document stream upload started: chat_id=%d user_id=%d filename=%q title=%q", chatID, userID, metadata.filename, metadata.title)
	_, uploadErr := h.client.SendDocumentStream(ctx, telegram.SendDocumentStreamRequest{
		ChatID:   chatID,
		Document: stream.Reader,
		Filename: metadata.filename,
	})
	if uploadErr != nil {
		_ = stream.Close()
	}

	downloadErr := stream.Wait()
	return uploadErr, downloadErr
}

type streamMetadata struct {
	title    string
	filename string
}

func audioMetadata(title string, fallbackFilename string) streamMetadata {
	cleanTitle := strings.TrimSpace(title)
	if cleanTitle == "" {
		cleanTitle = "audio"
	}

	extension := ".m4a"
	if ext := strings.TrimSpace(fallbackFilenameExtension(fallbackFilename)); ext != "" {
		extension = ext
	}

	return streamMetadata{
		title:    cleanTitle,
		filename: sanitizeFilename(cleanTitle) + extension,
	}
}

func fallbackFilenameExtension(filename string) string {
	index := strings.LastIndex(filename, ".")
	if index == -1 || index == len(filename)-1 {
		return ""
	}
	return filename[index:]
}

func sanitizeFilename(value string) string {
	replacer := strings.NewReplacer(
		`<`, "_",
		`>`, "_",
		`:`, "_",
		`"`, "_",
		`/`, "_",
		`\`, "_",
		`|`, "_",
		`?`, "_",
		`*`, "_",
	)
	value = replacer.Replace(strings.TrimSpace(value))
	value = strings.Trim(value, ". ")
	if value == "" {
		return "audio"
	}
	if len([]rune(value)) > 120 {
		return string([]rune(value)[:120])
	}
	return value
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
		return "Download took too long and was stopped. Please try a shorter authorized video."
	}

	return "Could not download audio. Make sure this is your authorized video."
}

func (h *Handler) logDownloadError(chatID int64, userID int64, videoURL string, err error) {
	var downloadErr youtube.DownloadError
	if errors.As(err, &downloadErr) {
		h.logger.Printf("audio download failed: chat_id=%d user_id=%d %s", chatID, userID, downloadErr.Diagnostics())
		return
	}

	h.logger.Printf("audio download failed: chat_id=%d user_id=%d url=%q error=%v", chatID, userID, videoURL, err)
}

func (h *Handler) isAdmin(userID int64) bool {
	return h.cfg.AdminUserIDs[userID]
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

func (h *Handler) keyboardForResults(userID int64, results []search.Result) *telegram.InlineKeyboardMarkup {
	rows := make([][]telegram.InlineKeyboardButton, 0, len(results))
	for i, result := range results {
		button := telegram.InlineKeyboardButton{
			Text: fmt.Sprintf("%02d. %s", i+1, trimButtonText(result.Label(), 56)),
		}

		button.CallbackData = h.buttons.put(result.URL, result.Label())
		rows = append(rows, []telegram.InlineKeyboardButton{button})
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
