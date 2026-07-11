package spotify

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	tokenURL  = "https://accounts.spotify.com/api/token"
	searchURL = "https://api.spotify.com/v1/search"
)

type Client struct {
	clientID     string
	clientSecret string
	httpClient   *http.Client
	now          func() time.Time

	mu               sync.Mutex
	token            string
	tokenExpiresAt   time.Time
	rateLimitedUntil time.Time
}

type Track struct {
	Name    string
	Artists []string
	URL     string
}

func (t Track) Label() string {
	if len(t.Artists) == 0 {
		return t.Name
	}
	return fmt.Sprintf("%s - %s", t.Name, strings.Join(t.Artists, ", "))
}

type RateLimitedError struct {
	RetryAfter time.Duration
}

func (e RateLimitedError) Error() string {
	return fmt.Sprintf("spotify rate limited for %s", e.RetryAfter)
}

type temporaryError struct {
	statusCode int
	body       string
}

func (e temporaryError) Error() string {
	return fmt.Sprintf("spotify temporary error: status %d", e.statusCode)
}

func IsTemporary(err error) bool {
	var temporary temporaryError
	return errors.As(err, &temporary)
}

func NewClient(clientID, clientSecret string, httpClient *http.Client) *Client {
	return &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   httpClient,
		now:          time.Now,
	}
}

func (c *Client) SearchTracks(ctx context.Context, query string, limit int) ([]Track, error) {
	if err := c.checkRateLimit(); err != nil {
		return nil, err
	}

	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}

	values := url.Values{}
	values.Set("q", query)
	values.Set("type", "track")
	values.Set("limit", strconv.Itoa(limit))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL+"?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, c.handleRateLimit(resp)
	}
	if resp.StatusCode >= 500 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, temporaryError{statusCode: resp.StatusCode, body: string(body)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("spotify search failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	tracks := make([]Track, 0, len(payload.Tracks.Items))
	for _, item := range payload.Tracks.Items {
		track := Track{
			Name: item.Name,
			URL:  item.ExternalURLs.Spotify,
		}
		for _, artist := range item.Artists {
			track.Artists = append(track.Artists, artist.Name)
		}
		if track.Name != "" && track.URL != "" {
			tracks = append(tracks, track)
		}
	}

	return tracks, nil
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	if c.token != "" && c.now().Before(c.tokenExpiresAt.Add(-30*time.Second)) {
		token := c.token
		c.mu.Unlock()
		return token, nil
	}
	c.mu.Unlock()

	form := url.Values{}
	form.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+basicAuth(c.clientID, c.clientSecret))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return "", c.handleRateLimit(resp)
	}
	if resp.StatusCode >= 500 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", temporaryError{statusCode: resp.StatusCode, body: string(body)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("spotify token failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.AccessToken == "" {
		return "", errors.New("spotify token response did not include an access token")
	}

	c.mu.Lock()
	c.token = payload.AccessToken
	c.tokenExpiresAt = c.now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	c.mu.Unlock()

	return payload.AccessToken, nil
}

func (c *Client) checkRateLimit() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.rateLimitedUntil.IsZero() || c.now().After(c.rateLimitedUntil) {
		return nil
	}

	return RateLimitedError{RetryAfter: time.Until(c.rateLimitedUntil)}
}

func (c *Client) handleRateLimit(resp *http.Response) error {
	retryAfter := retryAfterDuration(resp.Header.Get("Retry-After"))
	if retryAfter <= 0 {
		retryAfter = 30 * time.Second
	}

	c.mu.Lock()
	c.rateLimitedUntil = c.now().Add(retryAfter)
	c.mu.Unlock()

	return RateLimitedError{RetryAfter: retryAfter}
}

func basicAuth(clientID, clientSecret string) string {
	return base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
}

func retryAfterDuration(value string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type searchResponse struct {
	Tracks struct {
		Items []struct {
			Name         string `json:"name"`
			ExternalURLs struct {
				Spotify string `json:"spotify"`
			} `json:"external_urls"`
			Artists []struct {
				Name string `json:"name"`
			} `json:"artists"`
		} `json:"items"`
	} `json:"tracks"`
}
