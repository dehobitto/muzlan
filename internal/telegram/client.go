package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

const apiBaseURL = "https://api.telegram.org/bot"

type Client struct {
	token      string
	httpClient *http.Client
	baseURL    string
}

func NewClient(token string, httpClient *http.Client) *Client {
	return &Client{
		token:      token,
		httpClient: httpClient,
		baseURL:    apiBaseURL + token,
	}
}

func (c *Client) GetUpdates(ctx context.Context, request GetUpdatesRequest) ([]Update, error) {
	var result []Update
	err := c.post(ctx, "getUpdates", request, &result)
	return result, err
}

func (c *Client) SendMessage(ctx context.Context, request SendMessageRequest) (Message, error) {
	var result Message
	err := c.post(ctx, "sendMessage", request, &result)
	return result, err
}

func (c *Client) EditMessageText(ctx context.Context, request EditMessageTextRequest) error {
	var result Message
	return c.post(ctx, "editMessageText", request, &result)
}

func (c *Client) AnswerCallbackQuery(ctx context.Context, request AnswerCallbackQueryRequest) error {
	var result bool
	return c.post(ctx, "answerCallbackQuery", request, &result)
}

func (c *Client) SendAudio(ctx context.Context, request SendAudioRequest) (Message, error) {
	file, err := os.Open(request.AudioPath)
	if err != nil {
		return Message{}, err
	}
	defer file.Close()

	return c.sendMultipartFile(ctx, "sendAudio", "audio", multipartFileRequest{
		ChatID:   request.ChatID,
		Reader:   file,
		Filename: filepath.Base(request.AudioPath),
		Caption:  request.Caption,
		Title:    request.Title,
	})
}

func (c *Client) SendAudioStream(ctx context.Context, request SendAudioStreamRequest) (Message, error) {
	return c.sendMultipartFile(ctx, "sendAudio", "audio", multipartFileRequest{
		ChatID:   request.ChatID,
		Reader:   request.Audio,
		Filename: request.Filename,
		Caption:  request.Caption,
		Title:    request.Title,
	})
}

func (c *Client) SendDocument(ctx context.Context, request SendDocumentRequest) (Message, error) {
	file, err := os.Open(request.DocumentPath)
	if err != nil {
		return Message{}, err
	}
	defer file.Close()

	return c.sendMultipartFile(ctx, "sendDocument", "document", multipartFileRequest{
		ChatID:   request.ChatID,
		Reader:   file,
		Filename: filepath.Base(request.DocumentPath),
		Caption:  request.Caption,
	})
}

func (c *Client) SendDocumentStream(ctx context.Context, request SendDocumentStreamRequest) (Message, error) {
	return c.sendMultipartFile(ctx, "sendDocument", "document", multipartFileRequest{
		ChatID:   request.ChatID,
		Reader:   request.Document,
		Filename: request.Filename,
		Caption:  request.Caption,
	})
}

func (c *Client) sendMultipartFile(ctx context.Context, method string, fileField string, request multipartFileRequest) (Message, error) {
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	writeDone := make(chan error, 1)

	go func() {
		err := writeMultipartFile(multipartWriter, fileField, request)
		if closeErr := multipartWriter.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			_ = writer.CloseWithError(err)
			writeDone <- err
			return
		}

		writeDone <- writer.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/"+method, reader)
	if err != nil {
		_ = reader.CloseWithError(err)
		<-writeDone
		return Message{}, err
	}
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		_ = reader.CloseWithError(err)
		<-writeDone
		return Message{}, err
	}
	_ = reader.Close()
	writeErr := <-writeDone
	defer resp.Body.Close()

	var payload response[json.RawMessage]
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		if writeErr != nil {
			return Message{}, writeErr
		}
		return Message{}, err
	}
	if !payload.OK {
		return Message{}, fmt.Errorf("telegram %s failed: status %d: %s", method, payload.ErrorCode, payload.Description)
	}
	if writeErr != nil {
		return Message{}, writeErr
	}

	var result Message
	if err := json.Unmarshal(payload.Result, &result); err != nil {
		return Message{}, err
	}

	return result, nil
}

func writeMultipartFile(writer *multipart.Writer, fileField string, request multipartFileRequest) error {
	if request.Filename == "" {
		request.Filename = "audio"
	}

	if err := writer.WriteField("chat_id", strconv.FormatInt(request.ChatID, 10)); err != nil {
		return err
	}
	if request.Caption != "" {
		if err := writer.WriteField("caption", request.Caption); err != nil {
			return err
		}
	}
	if request.Title != "" {
		if err := writer.WriteField("title", request.Title); err != nil {
			return err
		}
	}

	part, err := writer.CreateFormFile(fileField, request.Filename)
	if err != nil {
		return err
	}
	_, err = io.Copy(part, request.Reader)
	return err
}

type multipartFileRequest struct {
	ChatID   int64
	Reader   io.Reader
	Filename string
	Caption  string
	Title    string
}

func (c *Client) post(ctx context.Context, method string, request any, result any) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/"+method, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var payload response[json.RawMessage]
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}
	if !payload.OK {
		return fmt.Errorf("telegram %s failed: status %d: %s", method, payload.ErrorCode, payload.Description)
	}

	if result == nil || len(payload.Result) == 0 {
		return nil
	}
	return json.Unmarshal(payload.Result, result)
}

type response[T any] struct {
	OK          bool   `json:"ok"`
	Result      T      `json:"result"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}

type GetUpdatesRequest struct {
	Offset  int `json:"offset,omitempty"`
	Limit   int `json:"limit,omitempty"`
	Timeout int `json:"timeout,omitempty"`
}

type SendMessageRequest struct {
	ChatID      int64                 `json:"chat_id"`
	Text        string                `json:"text"`
	ReplyMarkup *InlineKeyboardMarkup `json:"reply_markup,omitempty"`
}

type EditMessageTextRequest struct {
	ChatID      int64                 `json:"chat_id"`
	MessageID   int                   `json:"message_id"`
	Text        string                `json:"text"`
	ReplyMarkup *InlineKeyboardMarkup `json:"reply_markup,omitempty"`
}

type AnswerCallbackQueryRequest struct {
	CallbackQueryID string `json:"callback_query_id"`
	Text            string `json:"text,omitempty"`
	ShowAlert       bool   `json:"show_alert,omitempty"`
}

type SendAudioRequest struct {
	ChatID    int64
	AudioPath string
	Caption   string
	Title     string
}

type SendAudioStreamRequest struct {
	ChatID   int64
	Audio    io.Reader
	Filename string
	Caption  string
	Title    string
}

type SendDocumentRequest struct {
	ChatID       int64
	DocumentPath string
	Caption      string
}

type SendDocumentStreamRequest struct {
	ChatID   int64
	Document io.Reader
	Filename string
	Caption  string
}

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

type InlineKeyboardButton struct {
	Text         string `json:"text"`
	URL          string `json:"url,omitempty"`
	CallbackData string `json:"callback_data,omitempty"`
}

type Update struct {
	UpdateID      int            `json:"update_id"`
	Message       *Message       `json:"message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	From      User   `json:"from"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text,omitempty"`
}

type User struct {
	ID int64 `json:"id"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message,omitempty"`
	Data    string   `json:"data,omitempty"`
}
