package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

type InlineKeyboardButton struct {
	Text string `json:"text"`
	URL  string `json:"url,omitempty"`
}

type Update struct {
	UpdateID int      `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
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
